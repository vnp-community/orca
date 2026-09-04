package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// ssoStateTTL bounds how long a StartSsoLogin state token stays valid —
// same rationale and value as scm-integration-service's oauthStateTTL: long
// enough for a human to complete a provider's consent screen, short enough
// that a leaked/logged callback URL isn't a standing replay risk.
const ssoStateTTL = 15 * time.Minute

// StartSsoLoginInput mirrors StartSsoLoginRequest 1:1.
type StartSsoLoginInput struct {
	Provider    domain.SsoProvider
	RedirectURI string
}

// StartSsoLoginResult mirrors StartSsoLoginResponse 1:1.
type StartSsoLoginResult struct {
	AuthorizationURL string
	State            string
}

// StartSsoLogin begins the CR-LOGIN-001 authorization-code + PKCE flow:
// resolves the provider's SsoExchanger, generates a PKCE verifier/challenge
// pair, mints a signed state token binding provider/redirect_uri/verifier,
// and returns the URL to redirect the browser to. No provider HTTP call
// happens here, mirroring scm-integration-service's StartOAuthFlow shape.
type StartSsoLogin struct {
	exchangers SsoExchangerRegistry
	states     SsoStateCodec
	now        func() time.Time
}

func NewStartSsoLogin(exchangers SsoExchangerRegistry, states SsoStateCodec, now func() time.Time) *StartSsoLogin {
	if now == nil {
		now = time.Now
	}
	return &StartSsoLogin{exchangers: exchangers, states: states, now: now}
}

func (uc *StartSsoLogin) Execute(ctx context.Context, in StartSsoLoginInput) (StartSsoLoginResult, error) {
	if in.RedirectURI == "" {
		return StartSsoLoginResult{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_SSO_NO_REDIRECT_URI", "redirect_uri is required", nil)
	}

	exchanger, err := uc.exchangers.Resolve(in.Provider)
	if err != nil {
		return StartSsoLoginResult{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_SSO_PROVIDER_UNSUPPORTED", "no sso configuration registered for this provider", err)
	}

	codeVerifier, err := newPkceCodeVerifier()
	if err != nil {
		return StartSsoLoginResult{}, apperrors.New(apperrors.KindInternal, "AUTH_SSO_PKCE_FAILED", "failed to generate pkce verifier", err)
	}

	state, err := uc.states.Encode(SsoState{
		Provider:     in.Provider,
		RedirectURI:  in.RedirectURI,
		CodeVerifier: codeVerifier,
		ExpiresAt:    uc.now().Add(ssoStateTTL),
	})
	if err != nil {
		return StartSsoLoginResult{}, apperrors.New(apperrors.KindInternal, "AUTH_SSO_STATE_ENCODE_FAILED", "failed to mint sso state token", err)
	}

	return StartSsoLoginResult{
		AuthorizationURL: exchanger.AuthorizationURL(state, in.RedirectURI, pkceChallengeS256(codeVerifier)),
		State:            state,
	}, nil
}

// newPkceCodeVerifier returns a 43-128 char URL-safe verifier per RFC 7636
// §4.1 — 32 random bytes, base64url-encoded (43 chars), comfortably within
// range.
func newPkceCodeVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("usecase: generating pkce verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// pkceChallengeS256 derives the S256 code_challenge from a verifier per RFC
// 7636 §4.2: BASE64URL-ENCODE(SHA256(ASCII(code_verifier))).
func pkceChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
