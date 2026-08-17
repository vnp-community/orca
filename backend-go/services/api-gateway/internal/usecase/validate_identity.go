package usecase

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Identity is the tenant/user identity resolved from a validated
// credential — the only thing downstream code should carry, never a raw
// token (api-gateway.md §9: "identity propagation, not process
// boundaries").
type Identity struct {
	TenantID string
	UserID   string
}

var (
	// ErrNoCredential means neither an Authorization: Bearer header nor the
	// session cookie was present.
	ErrNoCredential = errors.New("authvalidator: no bearer token or session cookie present")
	// ErrMalformedToken means a credential was present but isn't
	// structurally a JWT (three dot-separated, base64url segments).
	ErrMalformedToken = errors.New("authvalidator: malformed JWT")
	// ErrMissingIdentityClaims means the JWT parsed but lacked the claims
	// needed to resolve an Identity.
	ErrMissingIdentityClaims = errors.New("authvalidator: token missing tenant_id/sub claims")
)

// SessionCookieName is the HTTP-only browser session cookie, SameSite=strict
// + Secure always on per api-gateway.md §9 (issued by auth-service; this
// service only reads it).
const SessionCookieName = "orca_session"

// AuthValidator resolves the caller's tenant/user Identity from an inbound
// HTTP request.
//
// # PLACEHOLDER — NOT PRODUCTION SAFE
//
// This extracts claims from a JWT's payload segment WITHOUT verifying its
// signature, and treats the session cookie's value as if it were a JWT too.
// Per api-gateway.md §9, production must instead:
//  1. Validate short-lived RS256 JWTs (mobile/CLI) against auth-service's
//     JWKS — fetch the key for the token's "kid" via a real JWKSClient
//     (ports.go, currently unimplemented) and verify the signature before
//     trusting any claim.
//  2. Resolve the browser session cookie against auth-service's session
//     store (an opaque session ID, not a bearer JWT, in the real design) —
//     not parsed as a JWT the way this placeholder does for simplicity.
//
// Today, any caller can forge a tenant_id/sub claim and be trusted. This is
// acceptable only because this is a scaffold with nothing production-real
// reachable through it; wire real JWKS/session-store verification before
// this service ever sees real traffic. See README.md "Known gaps".
type AuthValidator struct{}

// NewAuthValidator returns a placeholder AuthValidator. See the type doc
// for what real verification still needs to be wired.
func NewAuthValidator() *AuthValidator { return &AuthValidator{} }

// Validate extracts a bearer token from the Authorization header, falling
// back to the session cookie, and parses it as an unverified JWT to recover
// tenant_id/sub claims.
func (v *AuthValidator) Validate(r *http.Request) (Identity, error) {
	token := bearerToken(r)
	if token == "" {
		token = cookieToken(r)
	}
	if token == "" {
		return Identity{}, ErrNoCredential
	}

	claims, err := unverifiedJWTClaims(token)
	if err != nil {
		return Identity{}, err
	}

	tenantID, _ := claims["tenant_id"].(string)
	userID, _ := claims["sub"].(string)
	if userID == "" {
		userID, _ = claims["user_id"].(string)
	}
	if tenantID == "" || userID == "" {
		return Identity{}, ErrMissingIdentityClaims
	}
	return Identity{TenantID: tenantID, UserID: userID}, nil
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func cookieToken(r *http.Request) string {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

// unverifiedJWTClaims splits a JWT into its three dot-separated segments
// and base64url-decodes + JSON-decodes the payload (second) segment — no
// signature check performed. A real implementation must verify the
// header's "alg" against an allow-list and the signature against
// auth-service's JWKS before trusting any claim (see the AuthValidator doc
// comment above).
func unverifiedJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrMalformedToken
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrMalformedToken
	}
	return claims, nil
}
