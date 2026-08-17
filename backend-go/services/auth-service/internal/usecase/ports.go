// Package usecase holds auth-service's application services and the ports
// they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// Sentinel errors an adapter/postgres implementation wraps (fmt.Errorf
// "...: %w") and a usecase unwraps via errors.Is, so the usecase layer can
// tell "not found"/"already exists" apart from a generic infrastructure
// failure without either layer importing the other's implementation
// package — only this shared, dependency-free set of values.
var (
	ErrUserNotFound      = errors.New("usecase: user not found")
	ErrUserAlreadyExists = errors.New("usecase: a user with this email already exists in this tenant")
	ErrSessionNotFound   = errors.New("usecase: session not found")
)

// UserRepository is the persistence port for users. Implemented by
// internal/adapter/postgres against auth-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
//
// GetUserByEmail and CreateUser carry the bcrypt password hash as an extra
// return/parameter rather than a field on domain.User — the hash is
// storage-layer material the Login/CreateUser usecases need but which has
// no business meaning to any other consumer of a domain.User value (see
// domain.User's doc comment).
type UserRepository interface {
	// CreateUser persists a new user together with its password hash.
	// Returns an error satisfying errors.Is(err, ErrEmailAlreadyExists) if
	// (tenant_id, email) already exists.
	CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error)
	// GetUserByEmail looks up a user by email and returns its password hash
	// alongside it, for Login to verify against. The proto's LoginRequest
	// carries no tenant discriminator (see auth-service's README "Known
	// gaps"), so lookup is by email alone.
	GetUserByEmail(ctx context.Context, email string) (domain.User, string, error)
	GetUserByID(ctx context.Context, userID string) (domain.User, error)
	ListUsers(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.User, string, error)
	UpdateUserRole(ctx context.Context, userID string, role domain.Role) (domain.User, error)
}

// SessionRepository is the persistence port for sessions. Only the SHA-256
// hash of a session token is ever passed across this interface — see
// domain.Session's doc comment.
type SessionRepository interface {
	CreateSession(ctx context.Context, session domain.Session) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error)
	RevokeSession(ctx context.Context, tokenHash string, revokedAt time.Time) error
}

// AuditRepository is the persistence port for the append-only audit log.
// There is deliberately no Update/Delete method — see domain.AuditEntry's
// doc comment.
type AuditRepository interface {
	Append(ctx context.Context, entry domain.AuditEntry) error
	Query(ctx context.Context, tenantID string, since time.Time, pageToken string, pageSize int32) ([]domain.AuditEntry, string, error)
}

// PasswordHasher hashes and verifies passwords. Implemented by
// internal/adapter/bcrypt — kept behind this port so domain/ and the rest
// of usecase/ never import bcrypt directly, per auth-service.md §6.
type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare returns nil if password matches hash, a non-nil error
	// otherwise. Never distinguishes "wrong password" from other failure
	// modes in its error value in a way callers should branch on — Login
	// treats any non-nil error as invalid credentials.
	Compare(hash, password string) error
}

// Clock abstracts time.Now so session-expiry logic (Login, ValidateSession)
// is deterministically testable against fakes, per
// specs/backend-go/standards/testing-strategy.md.
type Clock interface {
	Now() time.Time
}

// SystemClock is the real Clock, wired in cmd/server/main.go.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
