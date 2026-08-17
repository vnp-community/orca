// Package postgres implements credential-broker-service's
// CredentialMetadataRepository and AuditRepository ports (defined in
// internal/usecase) against this service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in
// credential-broker-service that knows SQL exists. Per migrations/0001_init.up.sql,
// NOT ONE column this package reads or writes can hold a secret value.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// Repository implements both usecase.CredentialMetadataRepository and
// usecase.AuditRepository against Postgres via pgx — hand-written SQL, same
// convention as usage-service's reference adapter (see
// architecture/04-tech-stack.md: sqlc codegen is the eventual target).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, m domain.CredentialMetadata) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO credential.credential_metadata (
			id, tenant_id, owner_id, category, status, vault_path, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		m.ID, m.TenantID, m.OwnerID, string(m.Category), string(m.Status), m.VaultPath, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert credential metadata: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id string) (domain.CredentialMetadata, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, owner_id, category, status, vault_path, created_at, updated_at
		FROM credential.credential_metadata
		WHERE id = $1
	`, id)

	var m domain.CredentialMetadata
	var category, status string
	err := row.Scan(&m.ID, &m.TenantID, &m.OwnerID, &category, &status, &m.VaultPath, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CredentialMetadata{}, domain.ErrCredentialNotFound
	}
	if err != nil {
		return domain.CredentialMetadata{}, fmt.Errorf("postgres: query credential metadata: %w", err)
	}
	m.Category = domain.Category(category)
	m.Status = domain.Status(status)
	return m, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status domain.Status, now time.Time) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE credential.credential_metadata SET status = $2, updated_at = $3 WHERE id = $1
	`, id, string(status), now)
	if err != nil {
		return fmt.Errorf("postgres: update credential status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrCredentialNotFound
	}
	return nil
}

// Append inserts one access-audit row. Never wrapped in a retry-with-drop
// path — per credential-broker-service.md §8, a failed Append must
// propagate to the caller as a failed operation. internal/usecase's
// appendAudit helper is the only caller of this method and always
// propagates its error, never swallows it.
func (r *Repository) Append(ctx context.Context, e domain.AccessAuditEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO credential.access_audit_log (credential_id, accessor_service, action, occurred_at)
		VALUES ($1,$2,$3,$4)
	`, e.CredentialID, e.AccessorService, string(e.Action), e.OccurredAt)
	if err != nil {
		return fmt.Errorf("postgres: insert access audit entry: %w", err)
	}
	return nil
}
