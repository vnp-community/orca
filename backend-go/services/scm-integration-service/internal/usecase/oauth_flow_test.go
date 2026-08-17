package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeOAuthExchanger is an in-memory OAuthExchanger — stands in for the
// real internal/adapter/oauth.Client.
type fakeOAuthExchanger struct {
	authURL      string
	token        OAuthToken
	exchangeErr  error
	lastCode     string
	lastRedirect string
}

func (f *fakeOAuthExchanger) AuthorizationURL(state, redirectURI string) string {
	return f.authURL + "?state=" + state + "&redirect_uri=" + redirectURI
}

func (f *fakeOAuthExchanger) ExchangeCode(_ context.Context, code, redirectURI string) (OAuthToken, error) {
	f.lastCode, f.lastRedirect = code, redirectURI
	if f.exchangeErr != nil {
		return OAuthToken{}, f.exchangeErr
	}
	return f.token, nil
}

// fakeOAuthExchangerRegistry is an in-memory OAuthExchangerRegistry.
type fakeOAuthExchangerRegistry struct {
	exchangers map[domain.ScmProvider]OAuthExchanger
}

func (r *fakeOAuthExchangerRegistry) Resolve(provider domain.ScmProvider) (OAuthExchanger, error) {
	e, ok := r.exchangers[provider]
	if !ok {
		return nil, errors.New("fakeOAuthExchangerRegistry: no exchanger registered for provider")
	}
	return e, nil
}

// fakeOAuthStateCodec is an in-memory OAuthStateCodec — bypasses real
// signing since these tests only exercise usecase dispatch logic, not the
// codec's own tamper/expiry handling (that's oauthstate's own test suite).
type fakeOAuthStateCodec struct {
	encoded    map[string]OAuthState
	nextToken  int
	decodeErr  error
	forceState *OAuthState // when set, Decode always returns this regardless of token
}

func (f *fakeOAuthStateCodec) Encode(state OAuthState) (string, error) {
	if f.encoded == nil {
		f.encoded = map[string]OAuthState{}
	}
	f.nextToken++
	token := "token-" + string(rune('a'+f.nextToken))
	f.encoded[token] = state
	return token, nil
}

func (f *fakeOAuthStateCodec) Decode(token string) (OAuthState, error) {
	if f.decodeErr != nil {
		return OAuthState{}, f.decodeErr
	}
	if f.forceState != nil {
		return *f.forceState, nil
	}
	state, ok := f.encoded[token]
	if !ok {
		return OAuthState{}, errors.New("fakeOAuthStateCodec: unknown token")
	}
	return state, nil
}

// fakeCredentialWriter is an in-memory CredentialWriter.
type fakeCredentialWriter struct {
	writeErr     error
	lastTenant   string
	lastProvider domain.ScmProvider
	lastToken    OAuthToken
	calls        int
}

func (f *fakeCredentialWriter) Write(_ context.Context, tenantID string, provider domain.ScmProvider, token OAuthToken) error {
	f.calls++
	f.lastTenant, f.lastProvider, f.lastToken = tenantID, provider, token
	return f.writeErr
}

func TestStartOAuthFlow_BuildsAuthorizationURLAndSignedState(t *testing.T) {
	exchangers := &fakeOAuthExchangerRegistry{exchangers: map[domain.ScmProvider]OAuthExchanger{
		domain.ScmProviderGitHub: &fakeOAuthExchanger{authURL: "https://github.com/login/oauth/authorize"},
	}}
	states := &fakeOAuthStateCodec{}

	uc := NewStartOAuthFlow(exchangers, states, nil)
	result, err := uc.Execute(context.Background(), StartOAuthFlowInput{
		TenantID: "tenant-1", UserID: "user-1", Provider: domain.ScmProviderGitHub, RedirectURI: "https://gateway.example.com/auth/github/callback",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State == "" {
		t.Fatal("expected a non-empty state token")
	}
	if result.AuthorizationURL == "" {
		t.Fatal("expected a non-empty authorization url")
	}
	encoded, ok := states.encoded[result.State]
	if !ok {
		t.Fatalf("expected the returned state to have been minted by the codec")
	}
	if encoded.TenantID != "tenant-1" || encoded.Provider != domain.ScmProviderGitHub {
		t.Errorf("unexpected encoded state: %+v", encoded)
	}
}

func TestStartOAuthFlow_RequiresTenantAndRedirectURI(t *testing.T) {
	uc := NewStartOAuthFlow(&fakeOAuthExchangerRegistry{}, &fakeOAuthStateCodec{}, nil)

	if _, err := uc.Execute(context.Background(), StartOAuthFlowInput{RedirectURI: "https://x"}); err == nil {
		t.Error("expected error when tenant_id is missing")
	}
	if _, err := uc.Execute(context.Background(), StartOAuthFlowInput{TenantID: "t1"}); err == nil {
		t.Error("expected error when redirect_uri is missing")
	}
}

func TestStartOAuthFlow_UnregisteredProviderFails(t *testing.T) {
	uc := NewStartOAuthFlow(&fakeOAuthExchangerRegistry{}, &fakeOAuthStateCodec{}, nil)
	_, err := uc.Execute(context.Background(), StartOAuthFlowInput{TenantID: "t1", RedirectURI: "https://x", Provider: domain.ScmProviderGitHub})
	if err == nil {
		t.Fatal("expected error when no oauth exchanger is registered for the provider")
	}
}

func TestCompleteOAuthFlow_ExchangesCodeAndWritesCredential(t *testing.T) {
	exchanger := &fakeOAuthExchanger{token: OAuthToken{AccessToken: "gho_exchanged", Scope: "repo"}}
	exchangers := &fakeOAuthExchangerRegistry{exchangers: map[domain.ScmProvider]OAuthExchanger{domain.ScmProviderGitHub: exchanger}}
	states := &fakeOAuthStateCodec{forceState: &OAuthState{
		TenantID: "tenant-1", UserID: "user-1", Provider: domain.ScmProviderGitHub,
		RedirectURI: "https://gateway.example.com/auth/github/callback", ExpiresAt: time.Now().Add(time.Hour),
	}}
	writer := &fakeCredentialWriter{}

	uc := NewCompleteOAuthFlow(exchangers, states, writer)
	connected, err := uc.Execute(context.Background(), CompleteOAuthFlowInput{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Code: "auth-code", State: "state-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected {
		t.Fatal("expected connected=true on a successful exchange+write")
	}
	if exchanger.lastCode != "auth-code" {
		t.Errorf("expected the code to reach the exchanger, got %q", exchanger.lastCode)
	}
	if writer.calls != 1 || writer.lastTenant != "tenant-1" || writer.lastProvider != domain.ScmProviderGitHub || writer.lastToken.AccessToken != "gho_exchanged" {
		t.Errorf("expected the exchanged token to be written for the right tenant/provider, got %+v", writer)
	}
}

func TestCompleteOAuthFlow_RejectsStateTenantMismatch(t *testing.T) {
	exchangers := &fakeOAuthExchangerRegistry{exchangers: map[domain.ScmProvider]OAuthExchanger{domain.ScmProviderGitHub: &fakeOAuthExchanger{}}}
	states := &fakeOAuthStateCodec{forceState: &OAuthState{TenantID: "tenant-OTHER", Provider: domain.ScmProviderGitHub, ExpiresAt: time.Now().Add(time.Hour)}}
	writer := &fakeCredentialWriter{}

	uc := NewCompleteOAuthFlow(exchangers, states, writer)
	_, err := uc.Execute(context.Background(), CompleteOAuthFlowInput{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Code: "auth-code", State: "state-token",
	})
	if err == nil {
		t.Fatal("expected an error when the request's tenant_id doesn't match the signed state's tenant_id")
	}
	if writer.calls != 0 {
		t.Errorf("expected no credential write on a state mismatch, got %d calls", writer.calls)
	}
}

func TestCompleteOAuthFlow_RejectsInvalidState(t *testing.T) {
	exchangers := &fakeOAuthExchangerRegistry{exchangers: map[domain.ScmProvider]OAuthExchanger{domain.ScmProviderGitHub: &fakeOAuthExchanger{}}}
	states := &fakeOAuthStateCodec{decodeErr: errors.New("oauthstate: state token has expired")}

	uc := NewCompleteOAuthFlow(exchangers, states, &fakeCredentialWriter{})
	_, err := uc.Execute(context.Background(), CompleteOAuthFlowInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Code: "c", State: "s"})
	if err == nil {
		t.Fatal("expected an error for an invalid/expired state token")
	}
}

func TestCompleteOAuthFlow_PropagatesExchangeFailure(t *testing.T) {
	exchanger := &fakeOAuthExchanger{exchangeErr: errors.New("oauth: provider rejected code exchange")}
	exchangers := &fakeOAuthExchangerRegistry{exchangers: map[domain.ScmProvider]OAuthExchanger{domain.ScmProviderGitHub: exchanger}}
	states := &fakeOAuthStateCodec{forceState: &OAuthState{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, ExpiresAt: time.Now().Add(time.Hour)}}
	writer := &fakeCredentialWriter{}

	uc := NewCompleteOAuthFlow(exchangers, states, writer)
	_, err := uc.Execute(context.Background(), CompleteOAuthFlowInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Code: "bad-code", State: "s"})
	if err == nil {
		t.Fatal("expected the exchange failure to propagate")
	}
	if writer.calls != 0 {
		t.Errorf("expected no credential write when the exchange itself failed, got %d calls", writer.calls)
	}
}

func TestGetAuthStatus_ConnectedWhenCredentialResolves(t *testing.T) {
	uc := NewGetAuthStatus(&fakeCredentialResolver{token: "tok"})
	connected, err := uc.Execute(context.Background(), GetAuthStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected {
		t.Fatal("expected connected=true when the credential resolver returns a token")
	}
}

func TestGetAuthStatus_NotConnectedWhenResolveFails(t *testing.T) {
	uc := NewGetAuthStatus(&fakeCredentialResolver{err: errors.New("not found")})
	connected, err := uc.Execute(context.Background(), GetAuthStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("expected a resolve failure to report connected=false, not an RPC error: %v", err)
	}
	if connected {
		t.Fatal("expected connected=false when the credential resolver fails")
	}
}

// RevokeAuth's own tests now live in revoke_auth_test.go — the prior
// "surfaces the missing broker RPC as a typed error" scaffold test was
// removed once RevokeCredentialByOwner closed that gap for real.
