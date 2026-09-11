// Package mysql implements auth-service's UserRepository/SessionRepository/
// ServiceTokenRepository/AccessPolicyRepository/AuditRepository/
// SsoIdentityRepository/SsoGroupRoleMappingRepository/
// PairingSessionRepository/PairedDeviceRepository ports (defined in
// internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — the multi-dialect rollout adapter for
// CR-DB-002/CR-DB-003, mirroring internal/adapter/postgres's behavior
// (password-hash storage, session-token-hash storage, tenant scoping)
// 1:1 against the dialect-safe schema created by migrations/mysql/*.sql.
// See specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-014.md.
//
// SECURITY BOUNDARY: this package changes SQL storage mechanics ONLY.
// Every password hash (bcrypt), JWT signature (Vault Transit), and
// paired-device shared secret (Vault Transit-sealed) arrives here already
// hashed/encrypted by internal/adapter/bcrypt or internal/adapter/vault —
// this package never hashes a password, signs a token, or seals/unseals a
// secret; it only stores and retrieves the opaque values those layers
// hand it, exactly as internal/adapter/postgres does.
package mysql

import (
	"context"
	"database/sql"
)

// dbtx abstracts over the subset of *sql.DB this package's query methods
// need. Unlike credential-broker-service's mysql adapter, auth-service has
// no TxRunner port (no usecase here wraps multiple writes in one
// transaction), so this is a thin alias over *sql.DB rather than a
// dual-purpose *sql.DB/*sql.Tx abstraction.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Repository implements usecase.UserRepository, usecase.SessionRepository,
// usecase.ServiceTokenRepository, usecase.AccessPolicyRepository,
// usecase.AuditRepository, usecase.SsoIdentityRepository, and
// usecase.SsoGroupRoleMappingRepository against MySQL/TiDB via
// database/sql — hand-written SQL, mirroring
// internal/adapter/postgres.Repository's shape/scope exactly (see that
// package's doc comment).
type Repository struct {
	db dbtx
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting a scan
// helper serve both QueryRowContext and QueryContext call sites — mirrors
// internal/adapter/postgres's rowScanner of the same purpose.
type rowScanner interface {
	Scan(dest ...any) error
}
