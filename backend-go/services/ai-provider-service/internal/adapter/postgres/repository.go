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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
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
	_ usecase.DueHealthCheckClaimer     = (*Repository)(nil)
	_ usecase.OutboxEnqueuer            = (*Repository)(nil)
	_ outbox.Store                      = (*Repository)(nil)
)

// accountColumns is the full column list every SELECT against
// ai_provider.accounts uses, in the exact order scanAccount expects.
const accountColumns = `id, tenant_id, provider_type, status, credential_ref,
	       scope, user_id, project_id, dev_server_id, label, model_hint, base_url,
	       quota_limit_day, models, is_default, last_health_check_at, created_by,
	       latency_ms, health_detail, quota_warning_sent_date,
	       rotation_grace_until, created_at, updated_at`

func (r *Repository) Create(ctx context.Context, account domain.ProviderAccount) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin create-account tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	if account.IsDefault {
		// Demote any prior default for this dev_server+provider pair BEFORE
		// inserting — the partial unique index
		// (uq_accounts_one_default_per_dev_server_provider) would otherwise
		// reject the insert outright rather than performing the demotion.
		if _, err := tx.Exec(ctx, `
			UPDATE ai_provider.accounts SET is_default = false, updated_at = now()
			WHERE tenant_id = $1 AND dev_server_id = $2 AND provider_type = $3 AND is_default AND deleted_at IS NULL
		`, account.TenantID, account.DevServerID, string(account.ProviderType)); err != nil {
			return fmt.Errorf("postgres: demote prior default account: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_provider.accounts (
			id, tenant_id, provider_type, status, credential_ref, scope, user_id, project_id,
			dev_server_id, label, model_hint, base_url, quota_limit_day, models, is_default,
			created_by, rotation_grace_until, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, account.ID, account.TenantID, string(account.ProviderType), string(account.Status), account.CredentialRef,
		string(account.Scope), nullableString(account.UserID), nullableString(account.ProjectID), account.DevServerID,
		account.Label, nullableString(account.ModelHint), nullableString(account.BaseURL), account.QuotaLimitDay,
		nonNilStringSlice(account.Models), account.IsDefault, nullableString(account.CreatedBy), account.RotationGraceUntil,
		account.CreatedAt, account.UpdatedAt); err != nil {
		return fmt.Errorf("postgres: insert account: %w", err)
	}

	if err := insertOutboxEvent(ctx, tx, account); err != nil {
		return err // same tx — a failed outbox write rolls back the account insert too
	}

	return tx.Commit(ctx)
}

// insertOutboxEvent builds and writes the ai_provider.account.registered
// event — same table shape usage-service/issue-tracking-service already use.
func insertOutboxEvent(ctx context.Context, tx pgx.Tx, account domain.ProviderAccount) error {
	payload, err := json.Marshal(map[string]any{
		"account_id":    account.ID,
		"provider_type": string(account.ProviderType),
		"dev_server_id": account.DevServerID,
		"scope":         string(account.Scope),
		"created_by":    account.CreatedBy,
	})
	if err != nil {
		return fmt.Errorf("postgres: marshal outbox payload: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO ai_provider.outbox (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1,$2,'ai_provider.account.registered',$3,1,$4)
	`, uuid.NewString(), account.TenantID, account.CreatedAt, payload)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM ai_provider.outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate outbox rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `UPDATE ai_provider.outbox SET published_at = now() WHERE id = ANY($1)`, ids)
	if err != nil {
		return fmt.Errorf("postgres: mark outbox events published: %w", err)
	}
	return nil
}

// Enqueue implements usecase.OutboxEnqueuer — same ai_provider.outbox table
// Create's insertOutboxEvent writes into, reused here rather than adding a
// second table.
func (r *Repository) Enqueue(ctx context.Context, subject, tenantID string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("postgres: marshal outbox payload: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO ai_provider.outbox (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1,$2,$3,now(),1,$4)
	`, uuid.NewString(), tenantID, subject, body)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.ProviderAccount, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+accountColumns+`
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
		SELECT `+accountColumns+`
		FROM ai_provider.accounts
		WHERE tenant_id = $1
		  AND deleted_at IS NULL
		  AND ($2 = '' OR scope = $2)
		  AND ($3 = '' OR user_id = $3::uuid OR project_id = $3::uuid)
		  AND ($4 = '' OR dev_server_id = $4)
		  AND ($5 = '' OR provider_type = $5)
		ORDER BY created_at
	`, filter.TenantID, string(filter.Scope), filter.ScopeRefID, filter.DevServerID, string(filter.ProviderType))
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
			health_detail = COALESCE($6, health_detail),
			credential_ref = CASE WHEN $4 = '' THEN credential_ref ELSE $4 END,
			rotation_grace_until = COALESCE($5, rotation_grace_until),
			updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+accountColumns+`
	`, in.TenantID, in.AccountID, string(in.Status), in.CredentialRef, in.RotationGraceUntil, in.HealthDetail)

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
		RETURNING `+accountColumns+`
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

// MarkQuotaWarningSent implements the 80%-warning idempotency guard.
func (r *Repository) MarkQuotaWarningSent(ctx context.Context, tenantID, accountID string, day time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_provider.accounts SET quota_warning_sent_date = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, accountID, day)
	if err != nil {
		return fmt.Errorf("postgres: mark quota warning sent: %w", err)
	}
	return nil
}

// GetToday implements usecase.UsageRepository — reads the daily rollup row
// only, never raw usage events (ai_provider.usage.md §5/§2 distinction from
// usage-service). No matching row means zero usage today, not an error.
func (r *Repository) GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT cost_usd, request_count, tokens_used
		FROM ai_provider.usage
		WHERE tenant_id = $1 AND account_id = $2 AND date = $3
	`, tenantID, accountID, day)

	state := domain.QuotaState{AccountID: accountID, Date: day}
	err := row.Scan(&state.CostUSD, &state.RequestCount, &state.TokensUsed)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return domain.QuotaState{}, fmt.Errorf("postgres: query usage rollup: %w", err)
	}
	return state, nil
}

// IncrementUsage implements usecase.UsageRepository — additive upsert,
// returns the POST-increment state so RecordTokenUsage can compare against
// quota without a second read.
func (r *Repository) IncrementUsage(ctx context.Context, tenantID, accountID string, day time.Time, tokensUsed, requestCount int64, costUSD float64) (domain.QuotaState, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO ai_provider.usage (account_id, tenant_id, date, tokens_used, cost_usd, request_count)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (account_id, date) DO UPDATE SET
			tokens_used = ai_provider.usage.tokens_used + EXCLUDED.tokens_used,
			cost_usd = ai_provider.usage.cost_usd + EXCLUDED.cost_usd,
			request_count = ai_provider.usage.request_count + EXCLUDED.request_count
		RETURNING tokens_used, cost_usd, request_count
	`, accountID, tenantID, day, tokensUsed, costUSD, requestCount)

	state := domain.QuotaState{AccountID: accountID, Date: day}
	if err := row.Scan(&state.TokensUsed, &state.CostUSD, &state.RequestCount); err != nil {
		return domain.QuotaState{}, fmt.Errorf("postgres: increment usage rollup: %w", err)
	}
	return state, nil
}

// healthCheckBatch implements usecase.ClaimedHealthCheckBatch — the claim
// transaction stays open across dispatch (RecordResult runs inside it), so
// a crash mid-batch rolls back to "still due" rather than silently skipping
// the next tick's retry. tenantByAccount is captured during ClaimDue's scan
// so RecordResult can scope its own UPDATE without a second lookup.
type healthCheckBatch struct {
	tx              pgx.Tx
	accounts        []domain.ProviderAccount
	tenantByAccount map[string]string
}

func (b *healthCheckBatch) Accounts() []domain.ProviderAccount { return b.accounts }

func (b *healthCheckBatch) RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error {
	tenantID := b.tenantByAccount[accountID]
	_, err := b.tx.Exec(ctx, `
		UPDATE ai_provider.accounts
		SET status = $3, health_detail = $4, latency_ms = $5, last_health_check_at = $6, updated_at = now()
		WHERE id = $1 AND tenant_id = $2
	`, accountID, tenantID, string(status), healthDetail, latencyMs, checkedAt)
	return err
}

func (b *healthCheckBatch) Commit(ctx context.Context) error   { return b.tx.Commit(ctx) }
func (b *healthCheckBatch) Rollback(ctx context.Context) error { return b.tx.Rollback(ctx) }

// ClaimDue implements usecase.DueHealthCheckClaimer — same
// SELECT...FOR UPDATE SKIP LOCKED shape as automation-service's
// DueAutomationClaimer.ClaimDue: no tenant_id filter, the scheduler scans
// across every tenant on a timer; every returned row still carries its own
// tenant_id, which RecordResult scopes its own UPDATE to.
func (r *Repository) ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (usecase.ClaimedHealthCheckBatch, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: begin health-check claim tx: %w", err)
	}
	// cutoff is computed in Go rather than as "$1 - $2::interval" in SQL —
	// with two untyped parameters either side of "-", Postgres's operator
	// resolution can infer both as interval instead of ($1 timestamptz,
	// $2 interval), producing "operator does not exist: timestamp with
	// time zone <= interval". Passing one already-computed timestamptz
	// parameter sidesteps the ambiguity entirely.
	cutoff := now.Add(-staleness)
	rows, err := tx.Query(ctx, `
		SELECT `+accountColumns+`
		FROM ai_provider.accounts
		WHERE status = 'active' AND deleted_at IS NULL
		  AND (last_health_check_at IS NULL OR last_health_check_at <= $1)
		ORDER BY last_health_check_at NULLS FIRST
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, cutoff, limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("postgres: query due-for-health-check accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.ProviderAccount
	tenantByAccount := make(map[string]string)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("postgres: scan due-for-health-check account: %w", err)
		}
		accounts = append(accounts, account)
		tenantByAccount[account.ID] = account.TenantID
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("postgres: iterate due-for-health-check rows: %w", err)
	}
	return &healthCheckBatch{tx: tx, accounts: accounts, tenantByAccount: tenantByAccount}, nil
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
	var userID, projectID, modelHint, baseURL, createdBy, healthDetail *string
	if err := row.Scan(
		&a.ID, &a.TenantID, &providerType, &status, &a.CredentialRef,
		&scope, &userID, &projectID, &a.DevServerID, &a.Label, &modelHint, &baseURL,
		&a.QuotaLimitDay, &a.Models, &a.IsDefault, &a.LastHealthCheckAt, &createdBy,
		&a.LatencyMs, &healthDetail, &a.QuotaWarningSentDate,
		&a.RotationGraceUntil, &a.CreatedAt, &a.UpdatedAt,
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
	if modelHint != nil {
		a.ModelHint = *modelHint
	}
	if baseURL != nil {
		a.BaseURL = *baseURL
	}
	if createdBy != nil {
		a.CreatedBy = *createdBy
	}
	a.HealthDetail = healthDetail
	return a, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nonNilStringSlice guards against pgx sending a Go nil slice as SQL NULL
// into models (TEXT[] NOT NULL DEFAULT '{}') — an explicit column in an
// INSERT's column list always overrides the column default, so a nil here
// would violate the NOT NULL constraint instead of falling back to '{}'.
func nonNilStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
