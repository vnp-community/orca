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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"
)

// dbtx abstracts over the subset of *pgxpool.Pool and pgx.Tx that
// Repository's query methods need. Both satisfy it, so every method below
// runs unchanged whether db is the pool directly or a transaction opened by
// RunInTx — no SQL is duplicated between the two paths.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository implements both usecase.CredentialMetadataRepository and
// usecase.AuditRepository against Postgres via pgx — hand-written SQL, same
// convention as usage-service's reference adapter (see
// architecture/04-tech-stack.md: sqlc codegen is the eventual target). It
// also implements usecase.TxRunner (see RunInTx below) so
// write/rotate/revoke usecases can commit their metadata mutation and audit
// append together.
type Repository struct {
	pool *pgxpool.Pool
	db   dbtx // == pool outside a transaction; == a pgx.Tx inside RunInTx's fn
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, db: pool}
}

// RunInTx implements usecase.TxRunner: opens one Postgres transaction via
// pgx.BeginFunc (commits on fn returning nil, rolls back and returns fn's
// error otherwise) and hands fn a Repository scoped to that transaction —
// see usecase.TxRunner's doc comment for why this reuses the same port
// shapes rather than introducing transaction-specific interfaces.
func (r *Repository) RunInTx(ctx context.Context, fn func(ctx context.Context, metadataRepo usecase.CredentialMetadataRepository, auditRepo usecase.AuditRepository) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		scoped := &Repository{pool: r.pool, db: tx}
		return fn(ctx, scoped, scoped)
	})
}

func (r *Repository) Create(ctx context.Context, m domain.CredentialMetadata) error {
	_, err := r.db.Exec(ctx, `
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
	row := r.db.QueryRow(ctx, `
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

// GetByOwner implements usecase.CredentialMetadataRepository.GetByOwner —
// see that interface method's doc comment. "Most recent" is the tie-break
// when a tenant somehow has more than one non-revoked row for the same
// (category, owner_id): this scaffold has no uniqueness constraint
// enforcing at most one active credential per (tenant, category, owner_id)
// (see this service's README "Known gaps"), so WriteCredential calls for
// the same logical owner accumulate rows rather than replacing one, and
// this query picks the newest.
func (r *Repository) GetByOwner(ctx context.Context, tenantID string, category domain.Category, ownerID string) (domain.CredentialMetadata, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, owner_id, category, status, vault_path, created_at, updated_at
		FROM credential.credential_metadata
		WHERE tenant_id = $1 AND category = $2 AND owner_id = $3 AND status != $4
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, string(category), ownerID, string(domain.StatusRevoked))

	var m domain.CredentialMetadata
	var cat, status string
	err := row.Scan(&m.ID, &m.TenantID, &m.OwnerID, &cat, &status, &m.VaultPath, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CredentialMetadata{}, domain.ErrCredentialNotFound
	}
	if err != nil {
		return domain.CredentialMetadata{}, fmt.Errorf("postgres: query credential metadata by owner: %w", err)
	}
	m.Category = domain.Category(cat)
	m.Status = domain.Status(status)
	return m, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status domain.Status, now time.Time) error {
	tag, err := r.db.Exec(ctx, `
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
	_, err := r.db.Exec(ctx, `
		INSERT INTO credential.access_audit_log (credential_id, accessor_service, action, occurred_at)
		VALUES ($1,$2,$3,$4)
	`, e.CredentialID, e.AccessorService, string(e.Action), e.OccurredAt)
	if err != nil {
		return fmt.Errorf("postgres: insert access audit entry: %w", err)
	}
	return nil
}
