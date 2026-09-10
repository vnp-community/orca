// Package oauthstate implements usecase.SsoStateCodec as a stateless,
// HMAC-signed token — no database row, no in-memory map. auth-service's own
// data model has no natural "pending sso login" table, and api-gateway
// (which actually terminates the browser's redirect) owns no database at
// all — so the state token itself carries its own payload plus a signature
// that lets CompleteSsoLogin detect tampering or expiry without looking
// anything up. Mirrors scm-integration-service's internal/adapter/oauthstate
// package verbatim, adapted for usecase.SsoState's shape (no tenant/user —
// see that type's doc comment — plus a PKCE CodeVerifier field).
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

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

// ErrExpired is returned by Decode for a structurally-valid, correctly-
// signed token whose ExpiresAt has passed.
var ErrExpired = errors.New("oauthstate: state token has expired")

// ErrInvalidSignature is returned by Decode when the token's signature
// doesn't match — either tampered with or signed with a different secret
// (e.g. after a secret rotation).
var ErrInvalidSignature = errors.New("oauthstate: state token has an invalid signature")

// Codec implements usecase.SsoStateCodec using HMAC-SHA256.
type Codec struct {
	secret []byte
}

// New returns a Codec keyed by secret — see internal/config's
// SsoStateSecret doc comment for where this value comes from in
// cmd/server/main.go's composition root. An empty secret is accepted (so
// this package doesn't itself enforce config validity) but produces
// forgeable tokens; callers must not run with one in any real deployment.
func New(secret string) *Codec {
	return &Codec{secret: []byte(secret)}
}

var _ usecase.SsoStateCodec = (*Codec)(nil)

// statePayload is the JSON shape signed inside the token — a private
// mirror of usecase.SsoState so this package doesn't need the usecase
// package's exported field tags to double as a wire format.
type statePayload struct {
	Provider     string    `json:"provider"`
	RedirectURI  string    `json:"redirect_uri"`
	CodeVerifier string    `json:"code_verifier"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Encode signs state and returns "<payload>.<signature>", both
// base64url-encoded — dot-separated the way a JWT is, for the same reason
// (easy to eyeball-split, URL-safe without further escaping in a redirect
// query parameter).
func (c *Codec) Encode(state usecase.SsoState) (string, error) {
	payload, err := json.Marshal(statePayload{
		Provider:     string(state.Provider),
		RedirectURI:  state.RedirectURI,
		CodeVerifier: state.CodeVerifier,
		ExpiresAt:    state.ExpiresAt,
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
func (c *Codec) Decode(token string) (usecase.SsoState, error) {
	var encodedPayload, encodedSig string
	if i := lastDot(token); i >= 0 {
		encodedPayload, encodedSig = token[:i], token[i+1:]
	}
	if encodedPayload == "" || encodedSig == "" {
		return usecase.SsoState{}, fmt.Errorf("oauthstate: malformed state token")
	}

	gotSig, err := base64.RawURLEncoding.DecodeString(encodedSig)
	if err != nil {
		return usecase.SsoState{}, fmt.Errorf("oauthstate: decode signature: %w", err)
	}
	wantSig := c.sign(encodedPayload)
	if subtle.ConstantTimeCompare(gotSig, wantSig) != 1 {
		return usecase.SsoState{}, ErrInvalidSignature
	}

	rawPayload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return usecase.SsoState{}, fmt.Errorf("oauthstate: decode payload: %w", err)
	}
	var p statePayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return usecase.SsoState{}, fmt.Errorf("oauthstate: unmarshal payload: %w", err)
	}
	if time.Now().After(p.ExpiresAt) {
		return usecase.SsoState{}, ErrExpired
	}

	return usecase.SsoState{
		Provider:     domain.SsoProvider(p.Provider),
		RedirectURI:  p.RedirectURI,
		CodeVerifier: p.CodeVerifier,
		ExpiresAt:    p.ExpiresAt,
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
