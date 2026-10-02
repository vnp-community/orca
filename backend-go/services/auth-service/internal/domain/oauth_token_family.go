package domain

import "time"

// Revocation reasons stored on a token family.
const (
	OAuthRevokeUserRevoked   = "user_revoked"
	OAuthRevokeAdminRevoked  = "admin_revoked"
	OAuthRevokeReuseDetected = "reuse_detected"
	OAuthRevokeClientBlocked = "client_blocked"
)

// OAuthTokenFamily is one successful authorization: the authorization code,
// every rotated refresh token and every access token minted from them share
// its FamilyID, so one revoke (or reuse detection) cuts them all.
type OAuthTokenFamily struct {
	FamilyID     string
	TenantID     string
	UserID       string
	ClientID     string
	GrantID      string // logical FK to mcp.grants (no cross-DB FK)
	Scope        string // space-delimited, the scopes the user consented to
	Resource     string
	CreatedAt    time.Time
	RevokedAt    *time.Time
	RevokeReason string
}

// OAuthAuthCode is a single-use authorization code. Only the SHA-256 hash of
// the code is stored.
type OAuthAuthCode struct {
	CodeHash      string
	FamilyID      string
	TenantID      string
	RedirectURI   string
	CodeChallenge string
	ExpiresAt     time.Time
	UsedAt        *time.Time
}

// OAuthRefreshToken is one link of a rotating refresh-token chain. UsedAt
// non-nil means it was already rotated: presenting it again is reuse.
type OAuthRefreshToken struct {
	TokenHash      string
	FamilyID       string
	TenantID       string
	ExpiresAt      time.Time
	UsedAt         *time.Time
	ReplacedByHash string
}
