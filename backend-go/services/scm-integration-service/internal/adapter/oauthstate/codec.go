// Package oauthstate implements usecase.OAuthStateCodec as a stateless,
// HMAC-signed token — no database row, no in-memory map, per §9.1's design:
// this service's data model (scm-integration-service.md §5) deliberately
// holds only rate_limit_cache/webhook_delivery_log, so the state token
// carries its own payload plus a signature that lets CompleteOAuthFlow
// detect tampering or expiry without looking anything up. This is the same
// "self-contained signed token" shape a JWT uses, hand-rolled here (HMAC-
// SHA256 over a JSON payload) rather than pulling in a JWT dependency for
// one narrow, single-service use.
package oauthstate

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// ErrExpired is returned by Decode for a structurally-valid, correctly-
// signed token whose ExpiresAt has passed.
var ErrExpired = errors.New("oauthstate: state token has expired")

// ErrInvalidSignature is returned by Decode when the token's signature
// doesn't match — either tampered with or signed with a different secret
// (e.g. after a secret rotation).
var ErrInvalidSignature = errors.New("oauthstate: state token has an invalid signature")

// Codec implements usecase.OAuthStateCodec using HMAC-SHA256.
type Codec struct {
	secret []byte
}

// New returns a Codec keyed by secret — see internal/config's
// OAuthStateSecret doc comment for where this value comes from in
// cmd/server/main.go's composition root. An empty secret is accepted (so
// this package doesn't itself enforce config validity) but produces
// forgeable tokens; callers must not run with one in any real deployment.
func New(secret string) *Codec {
	return &Codec{secret: []byte(secret)}
}

var _ usecase.OAuthStateCodec = (*Codec)(nil)

// statePayload is the JSON shape signed inside the token — a private
// mirror of usecase.OAuthState so this package doesn't need the usecase
// package's exported field tags to double as a wire format.
type statePayload struct {
	TenantID    string    `json:"tenant_id"`
	UserID      string    `json:"user_id"`
	Provider    string    `json:"provider"`
	RedirectURI string    `json:"redirect_uri"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Encode signs state and returns "<payload>.<signature>", both
// base64url-encoded — dot-separated the way a JWT is, for the same reason
// (easy to eyeball-split, URL-safe without further escaping in a redirect
// query parameter).
func (c *Codec) Encode(state usecase.OAuthState) (string, error) {
	payload, err := json.Marshal(statePayload{
		TenantID:    state.TenantID,
		UserID:      state.UserID,
		Provider:    string(state.Provider),
		RedirectURI: state.RedirectURI,
		ExpiresAt:   state.ExpiresAt,
	})
	if err != nil {
		return "", fmt.Errorf("oauthstate: encode payload: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	sig := c.sign(encodedPayload)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Decode verifies the signature (constant-time comparison, guarding
// against timing attacks on the HMAC check) and expiry, and returns the
// original state on success.
func (c *Codec) Decode(token string) (usecase.OAuthState, error) {
	var encodedPayload, encodedSig string
	if i := lastDot(token); i >= 0 {
		encodedPayload, encodedSig = token[:i], token[i+1:]
	}
	if encodedPayload == "" || encodedSig == "" {
		return usecase.OAuthState{}, fmt.Errorf("oauthstate: malformed state token")
	}

	gotSig, err := base64.RawURLEncoding.DecodeString(encodedSig)
	if err != nil {
		return usecase.OAuthState{}, fmt.Errorf("oauthstate: decode signature: %w", err)
	}
	wantSig := c.sign(encodedPayload)
	if subtle.ConstantTimeCompare(gotSig, wantSig) != 1 {
		return usecase.OAuthState{}, ErrInvalidSignature
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return usecase.OAuthState{}, fmt.Errorf("oauthstate: decode payload: %w", err)
	}
	var p statePayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return usecase.OAuthState{}, fmt.Errorf("oauthstate: unmarshal payload: %w", err)
	}
	if time.Now().After(p.ExpiresAt) {
		return usecase.OAuthState{}, ErrExpired
	}

	return usecase.OAuthState{
		TenantID:    p.TenantID,
		UserID:      p.UserID,
		Provider:    domain.ScmProvider(p.Provider),
		RedirectURI: p.RedirectURI,
		ExpiresAt:   p.ExpiresAt,
	}, nil
}

func (c *Codec) sign(encodedPayload string) []byte {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(encodedPayload))
	return mac.Sum(nil)
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}
