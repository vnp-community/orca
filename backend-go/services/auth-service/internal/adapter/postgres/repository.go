// Package postgres implements auth-service's UserRepository/SessionRepository/
// AuditRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in auth-service that
// knows SQL exists.
package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements usecase.UserRepository, usecase.SessionRepository,
// and usecase.AuditRepository against Postgres via pgx — hand-written SQL
// (see architecture/04-tech-stack.md: sqlc codegen is the eventual target,
// this scaffold hand-writes the equivalent queries directly to avoid a
// build-time dependency on the sqlc binary, matching usage-service's
// reference implementation).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
