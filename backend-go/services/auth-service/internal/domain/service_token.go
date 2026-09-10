package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyJTI guards an IssuedServiceToken's identifying claim — the
	// "jti" every minted CLI/service token carries, and the only value
	// RevokeCliToken ever looks up by (never the JWT itself, which is
	// never persisted — see this type's doc comment).
	ErrEmptyJTI = errors.New("domain: jti is required")
	// ErrEmptyServiceTokenUser guards an IssuedServiceToken's owning user.
	ErrEmptyServiceTokenUser = errors.New("domain: user_id is required")
	// ErrEmptyAudience guards an IssuedServiceToken's aud claim.
	ErrEmptyAudience = errors.New("domain: audience is required")
	// ErrServiceTokenZeroExpiry mirrors Session's ErrZeroExpiry — a
	// service token with no expiry is not a valid domain state.
	ErrServiceTokenZeroExpiry = errors.New("domain: expires_at is required")
)

// IssuedServiceToken is a record of one CLI/service JWT IssueServiceToken
// minted — kept ONLY so it can be revoked before its natural expiry
// (CR-CLI-002/TASK-BE-CLI-005). Unlike Session, which stores a hash to
// validate against on every request, IssueServiceToken is otherwise
// stateless RS256 signing (Vault Transit) — this row exists purely as a
// revocation-list entry, never re-derived to reconstruct or re-verify the
// JWT itself (that's what JWKS/signature verification is for).
type IssuedServiceToken struct {
	JTI       string
	UserID    string
	Audience  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// NewIssuedServiceToken constructs an IssuedServiceToken, enforcing the
// same non-empty/non-zero invariants NewSession enforces for sessions.
func NewIssuedServiceToken(jti, userID, audience string, issuedAt, expiresAt time.Time) (IssuedServiceToken, error) {
	if jti == "" {
		return IssuedServiceToken{}, ErrEmptyJTI
	}
	if userID == "" {
		return IssuedServiceToken{}, ErrEmptyServiceTokenUser
	}
	if audience == "" {
		return IssuedServiceToken{}, ErrEmptyAudience
	}
	if expiresAt.IsZero() {
		return IssuedServiceToken{}, ErrServiceTokenZeroExpiry
	}
	return IssuedServiceToken{
		JTI:       jti,
		UserID:    userID,
		Audience:  audience,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
	}, nil
}

// IsRevoked reports whether this token has been revoked — mirrors
// Session.IsValid's shape, kept here rather than inline at every call site.
func (t IssuedServiceToken) IsRevoked() bool {
	return t.RevokedAt != nil
}
