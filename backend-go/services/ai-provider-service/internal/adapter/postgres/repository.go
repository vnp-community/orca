// Package postgres implements ai-provider-service's ProviderAccountRepository
// and UsageRepository ports (defined in internal/usecase) against this
// service's own PostgreSQL database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule: this is the ONLY package in ai-provider-service
// that knows SQL exists. It stores and reads back credential_ref values
// only — an opaque credential-broker-service pointer — never a secret.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

// Repository implements both usecase.ProviderAccountRepository and
// usecase.UsageRepository against Postgres via pgx — hand-written SQL (see
// architecture/04-tech-stack.md: sqlc codegen is the eventual target, this
// scaffold hand-writes the equivalent queries directly to avoid a
// build-time dependency on the sqlc binary — same choice usage-service made).
type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

var (
	_ usecase.ProviderAccountRepository = (*Repository)(nil)
	_ usecase.UsageRepository           = (*Repository)(nil)
)

func (r *Repository) Create(ctx context.Context, account domain.ProviderAccount) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_provider.accounts (
			id, tenant_id, provider_type, status, credential_ref,
			scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		account.ID, account.TenantID, string(account.ProviderType), string(account.Status), account.CredentialRef,
		string(account.Scope), nullableString(account.UserID), nullableString(account.ProjectID), account.DevServerID,
		account.RotationGraceUntil, account.CreatedAt, account.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: insert account: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.ProviderAccount, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, provider_type, status, credential_ref,
		       scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
		FROM ai_provider.accounts
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, id)

	account, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("postgres: query account: %w", err)
	}
	return account, nil
}

func (r *Repository) List(ctx context.Context, filter usecase.ListAccountsFilter) ([]domain.ProviderAccount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, provider_type, status, credential_ref,
		       scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
		FROM ai_provider.accounts
		WHERE tenant_id = $1
		  AND deleted_at IS NULL
		  AND ($2 = '' OR scope = $2)
		  AND ($3 = '' OR user_id = $3::uuid OR project_id = $3::uuid)
		  AND ($4 = '' OR dev_server_id = $4)
		ORDER BY created_at
	`, filter.TenantID, string(filter.Scope), filter.ScopeRefID, filter.DevServerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query accounts: %w", err)
	}
	defer rows.Close()

	var out []domain.ProviderAccount
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan account row: %w", err)
		}
		out = append(out, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate account rows: %w", err)
	}
	return out, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, in usecase.UpdateStatusInput) (domain.ProviderAccount, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_provider.accounts SET
			status = $3,
			credential_ref = CASE WHEN $4 = '' THEN credential_ref ELSE $4 END,
			rotation_grace_until = COALESCE($5, rotation_grace_until),
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, provider_type, status, credential_ref,
		          scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
	`, in.TenantID, in.AccountID, string(in.Status), in.CredentialRef, in.RotationGraceUntil)

	account, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("postgres: update account status: %w", err)
	}
	return account, nil
}

// Update implements usecase.ProviderAccountRepository.Update — mutates only
// Label/ModelHint/BaseURL, never Status/CredentialRef (see ports.go's doc
// comment on why those two axes are kept apart).
func (r *Repository) Update(ctx context.Context, in usecase.UpdateFields) (domain.ProviderAccount, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_provider.accounts
		SET label = $3, model_hint = $4, base_url = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
		RETURNING id, tenant_id, provider_type, status, credential_ref,
		          scope, user_id, project_id, dev_server_id, rotation_grace_until, created_at, updated_at
	`, in.TenantID, in.AccountID, in.Label, nullableString(in.ModelHint), nullableString(in.BaseURL))

	account, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("postgres: update account: %w", err)
	}
	return account, nil
}

// Delete implements usecase.ProviderAccountRepository.Delete — soft-delete
// only (status='revoked' + deleted_at), never a hard DELETE — preserves
// usage_daily's FK and the account's row in the audit trail. See ports.go's
// doc comment.
func (r *Repository) Delete(ctx context.Context, tenantID, accountID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE ai_provider.accounts
		SET status = 'revoked', deleted_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL
	`, tenantID, accountID)
	if err != nil {
		return fmt.Errorf("postgres: deleting provider account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}

// GetToday implements usecase.UsageRepository — reads the daily rollup row
// only, never raw usage events (ai_provider.usage.md §5/§2 distinction from
// usage-service). No matching row means zero usage today, not an error.
func (r *Repository) GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT cost_usd, request_count
		FROM ai_provider.usage
		WHERE tenant_id = $1 AND account_id = $2 AND date = $3
	`, tenantID, accountID, day)

	state := domain.QuotaState{AccountID: accountID, Date: day}
	err := row.Scan(&state.CostUSD, &state.RequestCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return domain.QuotaState{}, fmt.Errorf("postgres: query usage rollup: %w", err)
	}
	return state, nil
}

// rowScanner abstracts over pgx.Row / pgx.Rows, both of which expose Scan
// with the same signature — lets scanAccount serve Get/UpdateStatus (single
// row) and List (multi-row) without duplicating the column list.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (domain.ProviderAccount, error) {
	var a domain.ProviderAccount
	var providerType, status, scope string
	var userID, projectID *string
	if err := row.Scan(
		&a.ID, &a.TenantID, &providerType, &status, &a.CredentialRef,
		&scope, &userID, &projectID, &a.DevServerID, &a.RotationGraceUntil, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.ProviderAccount{}, err
	}
	a.ProviderType = domain.ProviderType(providerType)
	a.Status = domain.AccountStatus(status)
	a.Scope = domain.AccountScope(scope)
	if userID != nil {
		a.UserID = *userID
	}
	if projectID != nil {
		a.ProjectID = *projectID
	}
	return a, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
