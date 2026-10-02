// Package jwtauth is the shared RS256 JWT signing/verification code Epic D
// needs on both ends of the auth-service <-> api-gateway chain: auth-service
// signs via TransitSigner (Vault Transit-backed, private key never in
// process memory), api-gateway verifies via Verify against the JWKS
// auth-service publishes. One implementation shared by both sides means the
// claim shape and signing/verification semantics can never silently drift
// between issuer and verifier.
package jwtauth

import (
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Issuer is the standard "iss" claim every auth-service-issued JWT carries,
// and the value Verify checks it against.
const Issuer = "auth-service"

// Algorithm is the one signature algorithm this system issues and accepts,
// per specs/backend-go/architecture/04-tech-stack.md and auth-service.md
// §3/§6/§9 (both explicit — not a default picked here).
const Algorithm = jose.RS256

// Claims is this system's JWT payload: go-jose's standard registered claims
// (iss/sub/aud/exp/iat/jti) plus the one private claim every verifier
// needs — tenant_id — which api-gateway.AuthValidator has always read
// (previously without verifying the token was ever signed).
type Claims struct {
	jwt.Claims
	TenantID string `json:"tenant_id,omitempty"`
	// DeviceID is set only for a JWT minted through the mobile-pairing
	// handshake (auth-service's CompleteDevicePairing) — it's how
	// wscompat.Identity.DeviceID (TASK-MB-03/04) knows which paired device
	// issued a given request, for E2E-payload routing. Empty for every
	// other token this system issues.
	DeviceID string `json:"device_id,omitempty"`
	// Role is the caller's global role ("admin"/"user") at token-issuance
	// time — added so a bearer-JWT-authenticated caller propagates the same
	// role claim the cookie/session path already does (BE-SOL-002).
	Role string `json:"role,omitempty"`

	// MCP-only claims (BE-MCP-SOL-005/006): absent on every pre-existing
	// token, so verifiers of other audiences are unaffected. Role is never
	// set on MCP tokens; it is read live at validation time.
	Scope    string `json:"scope,omitempty"`     // space-delimited
	ClientID string `json:"client_id,omitempty"` // OAuth access token only
	GrantID  string `json:"grant_id,omitempty"`  // OAuth access token only (logical FK -> mcp.grants)
	FamilyID string `json:"fid,omitempty"`       // OAuth access token only (refresh family)
	TokenUse string `json:"token_use,omitempty"` // "mcp_oauth" | "mcp_pat"

	// Recursion guard (BE-MCP-SOL-013 G): set only by auth-service on child
	// tokens it mints for spawned agents; RS256 signing means a client can't
	// set them. Absent = depth 0.
	McpDepth int    `json:"mcp_depth,omitempty"`
	McpRoot  string `json:"mcp_root,omitempty"` // root MCP session id
}
