package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var (
	// ErrEmptyTokenHash is returned when a Session is constructed without a
	// token hash — per auth-service.md §5/§9, only the hash is ever stored,
	// but a Session always has one.
	ErrEmptyTokenHash = errors.New("domain: token_hash is required")
	// ErrEmptyUser guards a Session's owning user.
	ErrEmptyUser = errors.New("domain: user_id is required")
	// ErrZeroExpiry is returned when ExpiresAt is unset — "a session that
	// never expires" is not a valid domain state (auth-service.md §4).
	ErrZeroExpiry = errors.New("domain: expires_at is required")
	// ErrExpiryBeforeCreation guards time-range consistency.
	ErrExpiryBeforeCreation = errors.New("domain: expires_at must not be before created_at")
)

// Session is an opaque, high-entropy session token's server-side record.
// Only the SHA-256 hash of the raw token is ever held here or persisted —
// the raw token is returned to the caller once, at creation, and never
// stored anywhere. A stolen DB snapshot must not yield a usable token
// (auth-service.md §5/§9).
type Session struct {
	TokenHash  string
	UserID     string
	TenantID   string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastSeenAt *time.Time // nil until first touch — see ValidateSession (TASK-AUTH-02-06)
	IP         string     // may be "" for a session created before this migration, or if unresolved
	UserAgent  string
}

// NewSession constructs a Session, enforcing that every session has a
// token hash, an owning user, and an absolute expiry set at creation time.
func NewSession(tokenHash, userID, tenantID string, createdAt, expiresAt time.Time) (Session, error) {
	if tokenHash == "" {
		return Session{}, ErrEmptyTokenHash
	}
	if userID == "" {
		return Session{}, ErrEmptyUser
	}
	if tenantID == "" {
		return Session{}, ErrEmptyTenant
	}
	if expiresAt.IsZero() {
		return Session{}, ErrZeroExpiry
	}
	if expiresAt.Before(createdAt) {
		return Session{}, ErrExpiryBeforeCreation
	}
	return Session{
		TokenHash: tokenHash,
		UserID:    userID,
		TenantID:  tenantID,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

// WithClientInfo sets IP/UserAgent on a Session after construction —
// metadata, not an invariant NewSession's error returns need to enforce.
func (s Session) WithClientInfo(ip, userAgent string) Session {
	s.IP, s.UserAgent = ip, userAgent
	return s
}

// IsValid reports whether the session is neither revoked nor expired as of
// now — the check ValidateSession performs on every authenticated request.
func (s Session) IsValid(now time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	return now.Before(s.ExpiresAt)
}

// SessionWithUser pairs a Session with its owning user's email —
// denormalized via a JOIN at the postgres layer to avoid an N+1 lookup in
// the admin dashboard's cross-user session listing (ListSessions usecase).
type SessionWithUser struct {
	Session   Session
	UserEmail string
}

// HashSessionToken computes the SHA-256 hash (hex-encoded) of a raw session
// token. A pure function shared by Login (hash before storing) and
// ValidateSession/Logout/RevokeSession (hash before lookup) so the raw
// token is never the lookup key on either side.
func HashSessionToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
