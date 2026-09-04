package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// fakeSsoExchanger is a canned SsoExchanger — records the last
// AuthorizationURL/ExchangeAndVerify call so tests can assert on what a
// usecase passed it.
type fakeSsoExchanger struct {
	authURL string

	verified       VerifiedSsoIdentity
	exchangeErr    error
	exchangeCalled bool
}

func (f *fakeSsoExchanger) AuthorizationURL(state, redirectURI, codeChallenge string) string {
	return f.authURL + "?state=" + state + "&redirect_uri=" + redirectURI + "&code_challenge=" + codeChallenge
}

func (f *fakeSsoExchanger) ExchangeAndVerify(ctx context.Context, code, redirectURI, codeVerifier string) (VerifiedSsoIdentity, error) {
	f.exchangeCalled = true
	if f.exchangeErr != nil {
		return VerifiedSsoIdentity{}, f.exchangeErr
	}
	return f.verified, nil
}

// fakeSsoExchangerRegistry is an in-memory SsoExchangerRegistry.
type fakeSsoExchangerRegistry struct {
	exchangers map[domain.SsoProvider]SsoExchanger
}

func (f *fakeSsoExchangerRegistry) Resolve(provider domain.SsoProvider) (SsoExchanger, error) {
	e, ok := f.exchangers[provider]
	if !ok {
		return nil, errors.New("fake: no exchanger registered for provider")
	}
	return e, nil
}

// fakeSsoStateCodec is a deterministic, non-cryptographic SsoStateCodec —
// good enough to exercise StartSsoLogin/CompleteSsoLogin's own logic
// without pulling in the real HMAC adapter.
type fakeSsoStateCodec struct {
	decodeErr   error
	lastEncoded SsoState
}

func (f *fakeSsoStateCodec) Encode(state SsoState) (string, error) {
	f.lastEncoded = state
	return "encoded:" + string(state.Provider), nil
}

func (f *fakeSsoStateCodec) Decode(token string) (SsoState, error) {
	if f.decodeErr != nil {
		return SsoState{}, f.decodeErr
	}
	provider := strings.TrimPrefix(token, "encoded:")
	return SsoState{Provider: domain.SsoProvider(provider), RedirectURI: f.lastEncoded.RedirectURI, CodeVerifier: f.lastEncoded.CodeVerifier, ExpiresAt: f.lastEncoded.ExpiresAt}, nil
}

func TestStartSsoLogin_UnknownProvider(t *testing.T) {
	registry := &fakeSsoExchangerRegistry{exchangers: map[domain.SsoProvider]SsoExchanger{}}
	uc := NewStartSsoLogin(registry, &fakeSsoStateCodec{}, nil)

	_, err := uc.Execute(context.Background(), StartSsoLoginInput{Provider: domain.SsoProviderGitHub, RedirectURI: "https://app.example.com/auth/callback"})
	if err == nil {
		t.Fatal("expected an error for an unregistered provider")
	}
}

func TestStartSsoLogin_RequiresRedirectURI(t *testing.T) {
	registry := &fakeSsoExchangerRegistry{exchangers: map[domain.SsoProvider]SsoExchanger{domain.SsoProviderGitHub: &fakeSsoExchanger{}}}
	uc := NewStartSsoLogin(registry, &fakeSsoStateCodec{}, nil)

	_, err := uc.Execute(context.Background(), StartSsoLoginInput{Provider: domain.SsoProviderGitHub})
	if err == nil {
		t.Fatal("expected an error for a missing redirect_uri")
	}
}

func TestStartSsoLogin_ReturnsAuthorizationURLWithPkceChallenge(t *testing.T) {
	exchanger := &fakeSsoExchanger{authURL: "https://github.com/login/oauth/authorize"}
	registry := &fakeSsoExchangerRegistry{exchangers: map[domain.SsoProvider]SsoExchanger{domain.SsoProviderGitHub: exchanger}}
	states := &fakeSsoStateCodec{}
	uc := NewStartSsoLogin(registry, states, func() time.Time { return time.Unix(0, 0) })

	out, err := uc.Execute(context.Background(), StartSsoLoginInput{Provider: domain.SsoProviderGitHub, RedirectURI: "https://app.example.com/auth/callback"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.AuthorizationURL, "code_challenge=") {
		t.Errorf("authorization url = %q, want it to carry a code_challenge", out.AuthorizationURL)
	}
	if states.lastEncoded.CodeVerifier == "" {
		t.Error("expected a non-empty PKCE code_verifier to have been encoded into the state")
	}
	if states.lastEncoded.RedirectURI != "https://app.example.com/auth/callback" {
		t.Errorf("state redirect_uri = %q, want the input redirect_uri", states.lastEncoded.RedirectURI)
	}
}

func TestCompleteSsoLogin_RejectsInvalidState(t *testing.T) {
	states := &fakeSsoStateCodec{decodeErr: errors.New("tampered")}
	registry := &fakeSsoExchangerRegistry{exchangers: map[domain.SsoProvider]SsoExchanger{}}
	provision := newLoginOrProvisionSsoUserForTest(newFakeUserRepository(), newFakeSsoIdentityRepository(), &fakeTenantResolver{tenantID: "t1"}, time.Now())
	uc := NewCompleteSsoLogin(registry, states, provision)

	_, err := uc.Execute(context.Background(), CompleteSsoLoginInput{Code: "abc", State: "bad-state"})
	if err == nil {
		t.Fatal("expected an error for an invalid state token")
	}
}

func TestCompleteSsoLogin_RequiresCodeAndState(t *testing.T) {
	uc := NewCompleteSsoLogin(&fakeSsoExchangerRegistry{}, &fakeSsoStateCodec{}, nil)

	if _, err := uc.Execute(context.Background(), CompleteSsoLoginInput{State: "s"}); err == nil {
		t.Error("expected an error for a missing code")
	}
	if _, err := uc.Execute(context.Background(), CompleteSsoLoginInput{Code: "c"}); err == nil {
		t.Error("expected an error for a missing state")
	}
}

func TestCompleteSsoLogin_ExchangeFailure_NeverProvisions(t *testing.T) {
	exchanger := &fakeSsoExchanger{exchangeErr: errors.New("provider rejected code")}
	registry := &fakeSsoExchangerRegistry{exchangers: map[domain.SsoProvider]SsoExchanger{domain.SsoProviderGitHub: exchanger}}
	states := &fakeSsoStateCodec{}
	if _, err := states.Encode(SsoState{Provider: domain.SsoProviderGitHub, RedirectURI: "https://app.example.com/auth/callback", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	users := newFakeUserRepository()
	provision := newLoginOrProvisionSsoUserForTest(users, newFakeSsoIdentityRepository(), &fakeTenantResolver{tenantID: "t1"}, time.Now())
	uc := NewCompleteSsoLogin(registry, states, provision)

	_, err := uc.Execute(context.Background(), CompleteSsoLoginInput{Code: "abc", State: "encoded:github"})
	if err == nil {
		t.Fatal("expected an error when the provider exchange fails")
	}
	if !exchanger.exchangeCalled {
		t.Error("expected ExchangeAndVerify to have been attempted")
	}
	if len(users.byID) != 0 {
		t.Errorf("expected no user to be provisioned on exchange failure, got %d", len(users.byID))
	}
}

func TestCompleteSsoLogin_SuccessProvisionsAndReturnsSession(t *testing.T) {
	exchanger := &fakeSsoExchanger{verified: VerifiedSsoIdentity{Subject: "42", Email: "frank@example.com", EmailVerified: true, Name: "Frank"}}
	registry := &fakeSsoExchangerRegistry{exchangers: map[domain.SsoProvider]SsoExchanger{domain.SsoProviderGitHub: exchanger}}
	states := &fakeSsoStateCodec{}
	if _, err := states.Encode(SsoState{Provider: domain.SsoProviderGitHub, RedirectURI: "https://app.example.com/auth/callback", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	provision := newLoginOrProvisionSsoUserForTest(newFakeUserRepository(), newFakeSsoIdentityRepository(), &fakeTenantResolver{tenantID: "t1"}, time.Now())
	uc := NewCompleteSsoLogin(registry, states, provision)

	out, err := uc.Execute(context.Background(), CompleteSsoLoginInput{Code: "abc", State: "encoded:github"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.SessionToken == "" {
		t.Fatal("expected a non-empty session token")
	}
	if out.User.Email != "frank@example.com" {
		t.Errorf("user email = %q, want %q", out.User.Email, "frank@example.com")
	}
}
