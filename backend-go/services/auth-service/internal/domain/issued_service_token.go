package domain

import "time"

// IssuedServiceToken is one row of auth.issued_service_tokens — the
// revocation-list entry IssueServiceToken records for every CLI/service JWT
// it mints (CR-CLI-002/TASK-BE-CLI-005). The JWT itself is still verified by
// signature/exp (JWKS); this entity only backs the separate "has this jti
// been revoked" check and the admin-facing ListCliTokens/RevokeCliToken
// surface.
type IssuedServiceToken struct {
	JTI       string
	UserID    string
	Audience  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	// RevokedAt is nil for a currently-valid token.
	RevokedAt *time.Time
}

// IsRevoked reports whether this token has been revoked.
func (t IssuedServiceToken) IsRevoked() bool {
	return t.RevokedAt != nil
}
