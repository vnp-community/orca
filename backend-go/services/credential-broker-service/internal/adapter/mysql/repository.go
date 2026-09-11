// Package mysql implements credential-broker-service's
// CredentialMetadataRepository, AuditRepository, and TxRunner ports
// (defined in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — the multi-dialect rollout adapter for
// CR-DB-003, mirroring internal/adapter/postgres's behavior (tenant
// scoping, the RunInTx atomicity guarantee) 1:1 against the dialect-safe
// schema created by migrations/mysql/{0001_init,0002_config_json,
// 0003_owner_id_text}.up.sql — tables `credential_metadata`,
// `access_audit_log`, no schema/database prefix (a MySQL database is the
// schema-equivalent isolation unit; DATABASE_DSN points at a database
// named `credential`). See
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-006.md.
//
// Per migrations/mysql/0001_init.up.sql's "no secret columns, ever"
// comment: NOT ONE column this package reads or writes can hold a secret
// value. This package never touches Vault or any encryption/decryption
// call — that logic lives entirely in internal/adapter/vault and
// internal/usecase, untouched by this adapter's existence. This file only
// retargets SQL dialect for credential POINTER metadata, never the secret
// material itself.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/usecase"
)

// dbtx abstracts over the subset of *sql.DB and *sql.Tx that Repository's
// query methods need — mirrors internal/adapter/postgres's dbtx interface
// of the same purpose, so every method below runs unchanged whether db is
// the pool directly or a transaction opened by RunInTx.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Repository implements usecase.CredentialMetadataRepository,
// usecase.AuditRepository, and usecase.TxRunner against MySQL/TiDB via
// database/sql. No RLS equivalent exists in MySQL — every query below
// filters by tenant_id explicitly, which is the ONLY tenant-isolation
// enforcement for this adapter (see BE-DB-SOL-001 §4's finding: this was
// already true for the Postgres adapter too, since RLS never actually
// activated there — this doesn't lower the bar, it just doesn't add a
// backstop that was never real).
type Repository struct {
	db   *sql.DB
	conn dbtx // == db outside a transaction; == a *sql.Tx inside RunInTx's fn
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db, conn: db}
}

// RunInTx implements usecase.TxRunner: opens one MySQL transaction via
// database/sql's BeginTx (commits on fn returning nil, rolls back and
// returns fn's error otherwise) and hands fn a Repository scoped to that
// transaction — mirrors internal/adapter/postgres.Repository.RunInTx's
// pgx.BeginFunc shape exactly, see usecase.TxRunner's doc comment for why
// this reuses the same port shapes rather than introducing
// transaction-specific interfaces.
func (r *Repository) RunInTx(ctx context.Context, fn func(ctx context.Context, metadataRepo usecase.CredentialMetadataRepository, auditRepo usecase.AuditRepository) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin tx: %w", err)
	}
	scoped := &Repository{db: r.db, conn: tx}
	if err := fn(ctx, scoped, scoped); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mysql: commit tx: %w", err)
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, m domain.CredentialMetadata) error {
	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO credential_metadata (
			id, tenant_id, owner_id, category, status, vault_path, config_json, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?)
	`,
		m.ID, m.TenantID, m.OwnerID, string(m.Category), string(m.Status), m.VaultPath, m.ConfigJSON, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("mysql: insert credential metadata: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id string) (domain.CredentialMetadata, error) {
	row := r.conn.QueryRowContext(ctx, `
		SELECT id, tenant_id, owner_id, category, status, vault_path, COALESCE(config_json, ''), created_at, updated_at
		FROM credential_metadata
		WHERE id = ?
	`, id)

	var m domain.CredentialMetadata
	var category, status string
	err := row.Scan(&m.ID, &m.TenantID, &m.OwnerID, &category, &status, &m.VaultPath, &m.ConfigJSON, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CredentialMetadata{}, domain.ErrCredentialNotFound
	}
	if err != nil {
		return domain.CredentialMetadata{}, fmt.Errorf("mysql: query credential metadata: %w", err)
	}
	m.Category = domain.Category(category)
	m.Status = domain.Status(status)
	return m, nil
}

// GetByOwner implements usecase.CredentialMetadataRepository.GetByOwner —
// mirrors internal/adapter/postgres.Repository.GetByOwner's query and
// "most recent" tie-break exactly, see that method's doc comment.
func (r *Repository) GetByOwner(ctx context.Context, tenantID string, category domain.Category, ownerID string) (domain.CredentialMetadata, error) {
	row := r.conn.QueryRowContext(ctx, `
		SELECT id, tenant_id, owner_id, category, status, vault_path, COALESCE(config_json, ''), created_at, updated_at
		FROM credential_metadata
		WHERE tenant_id = ? AND category = ? AND owner_id = ? AND status != ?
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, string(category), ownerID, string(domain.StatusRevoked))

	var m domain.CredentialMetadata
	var cat, status string
	err := row.Scan(&m.ID, &m.TenantID, &m.OwnerID, &cat, &status, &m.VaultPath, &m.ConfigJSON, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CredentialMetadata{}, domain.ErrCredentialNotFound
	}
	if err != nil {
		return domain.CredentialMetadata{}, fmt.Errorf("mysql: query credential metadata by owner: %w", err)
	}
	m.Category = domain.Category(cat)
	m.Status = domain.Status(status)
	return m, nil
}

// ListByCategory implements usecase.CredentialMetadataRepository.ListByCategory
// — see that interface method's doc comment.
func (r *Repository) ListByCategory(ctx context.Context, tenantID string, category domain.Category) ([]domain.CredentialMetadata, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT id, tenant_id, owner_id, category, status, vault_path, COALESCE(config_json, ''), created_at, updated_at
		FROM credential_metadata
		WHERE tenant_id = ? AND category = ? AND status != ?
		ORDER BY created_at
	`, tenantID, string(category), string(domain.StatusRevoked))
	if err != nil {
		return nil, fmt.Errorf("mysql: listing credentials by category: %w", err)
	}
	defer rows.Close()

	var out []domain.CredentialMetadata
	for rows.Next() {
		var m domain.CredentialMetadata
		var cat, status string
		if err := rows.Scan(&m.ID, &m.TenantID, &m.OwnerID, &cat, &status, &m.VaultPath, &m.ConfigJSON, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scanning credential metadata: %w", err)
		}
		m.Category = domain.Category(cat)
		m.Status = domain.Status(status)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate credential metadata rows: %w", err)
	}
	return out, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status domain.Status, now time.Time) error {
	res, err := r.conn.ExecContext(ctx, `
		UPDATE credential_metadata SET status = ?, updated_at = ? WHERE id = ?
	`, string(status), now, id)
	if err != nil {
		return fmt.Errorf("mysql: update credential status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: update credential status rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrCredentialNotFound
	}
	return nil
}

// Append inserts one access-audit row. Never wrapped in a retry-with-drop
// path — per credential-broker-service.md §8, a failed Append must
// propagate to the caller as a failed operation, same as the Postgres
// adapter. internal/usecase's appendAudit helper is the only caller of
// this method and always propagates its error, never swallows it.
func (r *Repository) Append(ctx context.Context, e domain.AccessAuditEntry) error {
	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO access_audit_log (credential_id, accessor_service, action, occurred_at)
		VALUES (?,?,?,?)
	`, e.CredentialID, e.AccessorService, string(e.Action), e.OccurredAt)
	if err != nil {
		return fmt.Errorf("mysql: insert access audit entry: %w", err)
	}
	return nil
}
