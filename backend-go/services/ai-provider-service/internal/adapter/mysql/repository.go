// Package mysql implements ai-provider-service's ProviderAccountRepository,
// UsageRepository, DueHealthCheckClaimer, OutboxEnqueuer, and outbox.Store
// ports (defined in internal/usecase) against MySQL/TiDB via database/sql +
// github.com/go-sql-driver/mysql — mirroring internal/adapter/postgres's
// behavior (idempotency, tenant scoping, transactional outbox) 1:1 against
// the dialect-safe schema created by migrations/mysql (tables `accounts`,
// `usage`, `outbox`, no schema/database prefix, per
// specs/backend-go/crs/v4/multi-database/solutions/BE-DB-SOL-002.md §3's
// naming convention). It stores and reads back credential_ref values only —
// an opaque credential-broker-service pointer — never a secret, same
// invariant as the Postgres adapter (see that package's doc comment).
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

// Repository implements every port internal/adapter/postgres.Repository
// implements, against MySQL/TiDB. No RLS equivalent exists in MySQL —
// every query below filters by tenant_id explicitly (where the Postgres
// original does), which is the ONLY tenant-isolation enforcement for this
// adapter (see BE-DB-SOL-001 §4's finding: this was already true for the
// Postgres adapter too, since RLS never actually activated there — this
// doesn't lower the bar, it just doesn't add a backstop that was never
// real). ClaimDue is the one method that deliberately has NO tenant_id
// filter, mirroring the Postgres original — see its own doc comment.
type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

var (
	_ usecase.ProviderAccountRepository = (*Repository)(nil)
	_ usecase.UsageRepository           = (*Repository)(nil)
	_ usecase.DueHealthCheckClaimer     = (*Repository)(nil)
	_ usecase.OutboxEnqueuer            = (*Repository)(nil)
	_ outbox.Store                      = (*Repository)(nil)
)

// accountColumns is the full column list every SELECT against accounts
// uses, in the exact order scanAccount expects — same list/order as
// internal/adapter/postgres's accountColumns, minus the schema prefix.
const accountColumns = `id, tenant_id, provider_type, status, credential_ref,
	       scope, user_id, project_id, dev_server_id, label, model_hint, base_url,
	       quota_limit_day, models, is_default, last_health_check_at, created_by,
	       latency_ms, health_detail, quota_warning_sent_date,
	       rotation_grace_until, created_at, updated_at`

func (r *Repository) Create(ctx context.Context, account domain.ProviderAccount) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin create-account tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	if account.IsDefault {
		// Demote any prior default for this dev_server+provider pair BEFORE
		// inserting — mirrors the Postgres original's ordering exactly. The
		// generated column default_slot_key (migrations/mysql/0003) would
		// otherwise reject the insert outright via
		// uq_accounts_one_default_per_dev_server_provider rather than
		// performing the demotion, same as the Postgres partial unique
		// index it translates.
		if _, err := tx.ExecContext(ctx, `
			UPDATE accounts SET is_default = false, updated_at = NOW(6)
			WHERE tenant_id = ? AND dev_server_id = ? AND provider_type = ? AND is_default AND deleted_at IS NULL
		`, account.TenantID, account.DevServerID, string(account.ProviderType)); err != nil {
			return fmt.Errorf("mysql: demote prior default account: %w", err)
		}
	}

	modelsJSON, err := json.Marshal(nonNilStringSlice(account.Models))
	if err != nil {
		return fmt.Errorf("mysql: marshal models: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO accounts (
			id, tenant_id, provider_type, status, credential_ref, scope, user_id, project_id,
			dev_server_id, label, model_hint, base_url, quota_limit_day, models, is_default,
			created_by, rotation_grace_until, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, account.ID, account.TenantID, string(account.ProviderType), string(account.Status), account.CredentialRef,
		string(account.Scope), nullableString(account.UserID), nullableString(account.ProjectID), account.DevServerID,
		account.Label, nullableString(account.ModelHint), nullableString(account.BaseURL), account.QuotaLimitDay,
		modelsJSON, account.IsDefault, nullableString(account.CreatedBy), account.RotationGraceUntil,
		account.CreatedAt, account.UpdatedAt); err != nil {
		return fmt.Errorf("mysql: insert account: %w", err)
	}

	if err := insertOutboxEvent(ctx, tx, account); err != nil {
		return err // same tx — a failed outbox write rolls back the account insert too
	}

	return tx.Commit()
}

// insertOutboxEvent builds and writes the ai_provider.account.registered
// event — same table shape internal/adapter/postgres.insertOutboxEvent
// writes into.
func insertOutboxEvent(ctx context.Context, tx *sql.Tx, account domain.ProviderAccount) error {
	payload, err := json.Marshal(map[string]any{
		"account_id":    account.ID,
		"provider_type": string(account.ProviderType),
		"dev_server_id": account.DevServerID,
		"scope":         string(account.Scope),
		"created_by":    account.CreatedBy,
	})
	if err != nil {
		return fmt.Errorf("mysql: marshal outbox payload: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?,?,'ai_provider.account.registered',?,1,?)
	`, uuid.NewString(), account.TenantID, account.CreatedAt, payload)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

// FetchUnpublished and MarkPublished implement common/outbox.Store — see
// cmd/server/main.go for where the relay is wired.
func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, tenant_id, subject, occurred_at, version, payload
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("mysql: query unpublished outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		if err := rows.Scan(&rec.ID, &rec.Event.TenantID, &rec.Subject, &rec.Event.OccurredAt, &rec.Event.Version, &rec.Event.Payload); err != nil {
			return nil, fmt.Errorf("mysql: scan outbox row: %w", err)
		}
		rec.Event.ID = rec.ID
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate outbox rows: %w", err)
	}
	return out, nil
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE outbox SET published_at = NOW(6) WHERE id IN (%s)`, placeholders)
	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mysql: mark outbox events published: %w", err)
	}
	return nil
}

// Enqueue implements usecase.OutboxEnqueuer — same outbox table Create's
// insertOutboxEvent writes into, reused here rather than adding a second
// table.
func (r *Repository) Enqueue(ctx context.Context, subject, tenantID string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mysql: marshal outbox payload: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO outbox (id, tenant_id, subject, occurred_at, version, payload)
		VALUES (?,?,?,NOW(6),1,?)
	`, uuid.NewString(), tenantID, subject, body)
	if err != nil {
		return fmt.Errorf("mysql: insert outbox event: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, tenantID, id string) (domain.ProviderAccount, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+accountColumns+`
		FROM accounts
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, tenantID, id)

	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("mysql: query account: %w", err)
	}
	return account, nil
}

func (r *Repository) List(ctx context.Context, filter usecase.ListAccountsFilter) ([]domain.ProviderAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+accountColumns+`
		FROM accounts
		WHERE tenant_id = ?
		  AND deleted_at IS NULL
		  AND (? = '' OR scope = ?)
		  AND (? = '' OR user_id = ? OR project_id = ?)
		  AND (? = '' OR dev_server_id = ?)
		  AND (? = '' OR provider_type = ?)
		ORDER BY created_at
	`, filter.TenantID,
		string(filter.Scope), string(filter.Scope),
		filter.ScopeRefID, filter.ScopeRefID, filter.ScopeRefID,
		filter.DevServerID, filter.DevServerID,
		string(filter.ProviderType), string(filter.ProviderType))
	if err != nil {
		return nil, fmt.Errorf("mysql: query accounts: %w", err)
	}
	defer rows.Close()

	var out []domain.ProviderAccount
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("mysql: scan account row: %w", err)
		}
		out = append(out, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: iterate account rows: %w", err)
	}
	return out, nil
}

// UpdateStatus implements usecase.ProviderAccountRepository.UpdateStatus.
// MySQL has no RETURNING, and — unlike a plain not-found check — this
// method cannot use ExecContext's RowsAffected() to detect "no such
// account" either: MySQL's default RowsAffected() counts CHANGED rows, not
// WHERE-matched rows, so an idempotent retry that sets every field to its
// current value would report 0 even though the row exists (see
// TASK-BE-DB-010's annotation-service finding, and this package's
// TestRepository_UpdateStatus_NoopRetryStillSucceeds regression test).
// Always re-SELECT by the same (tenant_id, id) the UPDATE used instead —
// this matches the Postgres original's RETURNING semantics exactly
// (RETURNING yields 0 rows precisely when that WHERE clause matches
// nothing), just as two statements instead of one.
func (r *Repository) UpdateStatus(ctx context.Context, in usecase.UpdateStatusInput) (domain.ProviderAccount, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE accounts SET
			status = ?,
			health_detail = COALESCE(?, health_detail),
			credential_ref = CASE WHEN ? = '' THEN credential_ref ELSE ? END,
			rotation_grace_until = COALESCE(?, rotation_grace_until),
			updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, string(in.Status), in.HealthDetail, in.CredentialRef, in.CredentialRef, in.RotationGraceUntil, in.TenantID, in.AccountID)
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("mysql: update account status: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT `+accountColumns+`
		FROM accounts
		WHERE tenant_id = ? AND id = ?
	`, in.TenantID, in.AccountID)
	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("mysql: query updated account status: %w", err)
	}
	return account, nil
}

// Update implements usecase.ProviderAccountRepository.Update — mutates only
// Label/ModelHint/BaseURL, never Status/CredentialRef (see ports.go's doc
// comment). Re-SELECTs after the UPDATE rather than trusting RowsAffected(),
// same matched-vs-changed reasoning as UpdateStatus above (a retry with
// unchanged label/model_hint/base_url is a realistic idempotent-retry
// shape here too).
func (r *Repository) Update(ctx context.Context, in usecase.UpdateFields) (domain.ProviderAccount, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE accounts
		SET label = ?, model_hint = ?, base_url = ?, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, in.Label, nullableString(in.ModelHint), nullableString(in.BaseURL), in.TenantID, in.AccountID)
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("mysql: update account: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT `+accountColumns+`
		FROM accounts
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, in.TenantID, in.AccountID)
	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProviderAccount{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.ProviderAccount{}, fmt.Errorf("mysql: query updated account: %w", err)
	}
	return account, nil
}

// Delete implements usecase.ProviderAccountRepository.Delete — soft-delete
// only (status='revoked' + deleted_at), never a hard DELETE — preserves
// usage's FK and the account's row in the audit trail. See ports.go's doc
// comment.
//
// Unlike UpdateStatus/Update above, RowsAffected() IS safe to trust here:
// the WHERE clause already requires deleted_at IS NULL, and this statement
// always moves deleted_at from NULL to a non-NULL value — so a matched row
// is always also a changed row, deterministically, regardless of driver
// RowsAffected mode. A row that was already soft-deleted (or never
// existed) can never match this WHERE clause, so 0 here always means "not
// found or already deleted," never a false negative on a genuine retry.
func (r *Repository) Delete(ctx context.Context, tenantID, accountID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE accounts
		SET status = 'revoked', deleted_at = NOW(6), updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ? AND deleted_at IS NULL
	`, tenantID, accountID)
	if err != nil {
		return fmt.Errorf("mysql: deleting provider account: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mysql: deleting provider account rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrAccountNotFound
	}
	return nil
}

// MarkQuotaWarningSent implements the 80%-warning idempotency guard —
// mirrors the Postgres original exactly, including its lack of a
// not-found check (this method's caller already has a valid accountID
// from a prior read in the same request).
func (r *Repository) MarkQuotaWarningSent(ctx context.Context, tenantID, accountID string, day time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE accounts SET quota_warning_sent_date = ?, updated_at = NOW(6)
		WHERE tenant_id = ? AND id = ?
	`, day, tenantID, accountID)
	if err != nil {
		return fmt.Errorf("mysql: mark quota warning sent: %w", err)
	}
	return nil
}

// GetToday implements usecase.UsageRepository — reads the daily rollup row
// only, never raw usage events. No matching row means zero usage today,
// not an error.
//
// The table name is backtick-quoted (“ `usage` “, here and in
// IncrementUsage below) because USAGE is a MySQL reserved word (the GRANT
// ... USAGE privilege grammar) — confirmed by a real syntax error against
// MySQL 8 with the bare identifier. See migrations/mysql/0001_init.up.sql's
// comment for the full explanation; Postgres has no such reserved word, so
// `ai_provider.usage` needs no quoting there.
func (r *Repository) GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error) {
	row := r.db.QueryRowContext(ctx, "SELECT cost_usd, request_count, tokens_used FROM `usage` WHERE tenant_id = ? AND account_id = ? AND date = ?",
		tenantID, accountID, day)

	state := domain.QuotaState{AccountID: accountID, Date: day}
	err := row.Scan(&state.CostUSD, &state.RequestCount, &state.TokensUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return domain.QuotaState{}, fmt.Errorf("mysql: query usage rollup: %w", err)
	}
	return state, nil
}

// IncrementUsage implements usecase.UsageRepository — additive upsert via
// ON DUPLICATE KEY UPDATE (MySQL's ON CONFLICT DO UPDATE equivalent).
// MySQL has no RETURNING, so the post-increment state is re-read by
// primary key right after — always accurate regardless of whether the
// upsert branch was INSERT or UPDATE, and (unlike UpdateStatus/Update)
// there's no not-found ambiguity to resolve here: IncrementUsage always
// either creates or updates a row, it never fails to match one.
func (r *Repository) IncrementUsage(ctx context.Context, tenantID, accountID string, day time.Time, tokensUsed, requestCount int64, costUSD float64) (domain.QuotaState, error) {
	_, err := r.db.ExecContext(ctx, "INSERT INTO `usage` (account_id, tenant_id, date, tokens_used, cost_usd, request_count) VALUES (?,?,?,?,?,?) "+
		"ON DUPLICATE KEY UPDATE tokens_used = tokens_used + VALUES(tokens_used), cost_usd = cost_usd + VALUES(cost_usd), request_count = request_count + VALUES(request_count)",
		accountID, tenantID, day, tokensUsed, costUSD, requestCount)
	if err != nil {
		return domain.QuotaState{}, fmt.Errorf("mysql: increment usage rollup: %w", err)
	}

	row := r.db.QueryRowContext(ctx, "SELECT tokens_used, cost_usd, request_count FROM `usage` WHERE account_id = ? AND date = ?",
		accountID, day)
	state := domain.QuotaState{AccountID: accountID, Date: day}
	if err := row.Scan(&state.TokensUsed, &state.CostUSD, &state.RequestCount); err != nil {
		return domain.QuotaState{}, fmt.Errorf("mysql: query incremented usage rollup: %w", err)
	}
	return state, nil
}

// healthCheckBatch implements usecase.ClaimedHealthCheckBatch — the claim
// transaction stays open across dispatch (RecordResult runs inside it), so
// a crash mid-batch rolls back to "still due" rather than silently
// skipping the next tick's retry. tenantByAccount is captured during
// ClaimDue's scan so RecordResult can scope its own UPDATE without a
// second lookup. Mirrors internal/adapter/postgres's healthCheckBatch,
// swapping pgx.Tx for *sql.Tx.
type healthCheckBatch struct {
	tx              *sql.Tx
	accounts        []domain.ProviderAccount
	tenantByAccount map[string]string
}

func (b *healthCheckBatch) Accounts() []domain.ProviderAccount { return b.accounts }

func (b *healthCheckBatch) RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error {
	tenantID := b.tenantByAccount[accountID]
	_, err := b.tx.ExecContext(ctx, `
		UPDATE accounts
		SET status = ?, health_detail = ?, latency_ms = ?, last_health_check_at = ?, updated_at = NOW(6)
		WHERE id = ? AND tenant_id = ?
	`, string(status), healthDetail, latencyMs, checkedAt, accountID, tenantID)
	return err
}

func (b *healthCheckBatch) Commit(ctx context.Context) error   { return b.tx.Commit() }
func (b *healthCheckBatch) Rollback(ctx context.Context) error { return b.tx.Rollback() }

// ClaimDue implements usecase.DueHealthCheckClaimer — same
// SELECT...FOR UPDATE SKIP LOCKED shape as the Postgres original (MySQL
// 8.0.1+ supports SKIP LOCKED); no tenant_id filter, the scheduler scans
// across every tenant on a timer; every returned row still carries its own
// tenant_id, which RecordResult scopes its own UPDATE to.
func (r *Repository) ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (usecase.ClaimedHealthCheckBatch, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin health-check claim tx: %w", err)
	}
	// cutoff is computed in Go, not as "? - ? INTERVAL" in SQL — mirrors
	// the Postgres original's own reasoning for sidestepping operator-type
	// ambiguity, kept for parity even though MySQL's INTERVAL syntax
	// doesn't share Postgres's exact ambiguity.
	cutoff := now.Add(-staleness)
	// FORCE INDEX (idx_accounts_due_for_health_check) is load-bearing, not
	// a perf tweak: confirmed empirically (EXPLAIN against a real MySQL 8
	// server) that without it, the optimizer can choose a full table scan
	// over this index depending on table statistics — and per documented
	// InnoDB behavior, `ORDER BY ... LIMIT ... FOR UPDATE SKIP LOCKED`
	// that falls back to a filesort locks EVERY row matching the WHERE
	// clause during the scan, not just the LIMIT rows returned. That
	// over-locking is exactly what
	// TestClaimDue_NoDoubleClaimUnderConcurrency caught: without this
	// hint, claim A (LIMIT 5 of 10 due rows) locked all 10, leaving claim
	// B 0 rows instead of the other 5. See
	// migrations/mysql/0005_health_and_usage_writes.up.sql's comment for
	// the full diagnosis — the composite index this hint forces is what
	// lets the scan stay in last_health_check_at order (no filesort) so
	// locking can stop as soon as LIMIT rows are collected, matching the
	// Postgres original's actual locking behavior (its LockRows executor
	// node only locks rows actually pulled through Limit).
	rows, err := tx.QueryContext(ctx, `
		SELECT `+accountColumns+`
		FROM accounts FORCE INDEX (idx_accounts_due_for_health_check)
		WHERE status = 'active' AND deleted_at IS NULL
		  AND (last_health_check_at IS NULL OR last_health_check_at <= ?)
		ORDER BY last_health_check_at
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, cutoff, limit)
	// ORDER BY last_health_check_at (no NULLS FIRST clause, which MySQL
	// doesn't have): MySQL sorts NULL as the smallest value in ASC order
	// by default, so accounts never health-checked already sort first —
	// the same effective ordering the Postgres original's explicit
	// NULLS FIRST produces.
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("mysql: query due-for-health-check accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.ProviderAccount
	tenantByAccount := make(map[string]string)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("mysql: scan due-for-health-check account: %w", err)
		}
		accounts = append(accounts, account)
		tenantByAccount[account.ID] = account.TenantID
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("mysql: iterate due-for-health-check rows: %w", err)
	}
	return &healthCheckBatch{tx: tx, accounts: accounts, tenantByAccount: tenantByAccount}, nil
}

// rowScanner abstracts over *sql.Row / *sql.Rows, both of which expose Scan
// with the same signature — lets scanAccount serve Get/UpdateStatus/Update
// (single row) and List/ClaimDue (multi-row) without duplicating the
// column list. Mirrors internal/adapter/postgres's rowScanner.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (domain.ProviderAccount, error) {
	var a domain.ProviderAccount
	var providerType, status, scope string
	var userID, projectID, modelHint, baseURL, createdBy, healthDetail *string
	var modelsJSON []byte
	if err := row.Scan(
		&a.ID, &a.TenantID, &providerType, &status, &a.CredentialRef,
		&scope, &userID, &projectID, &a.DevServerID, &a.Label, &modelHint, &baseURL,
		&a.QuotaLimitDay, &modelsJSON, &a.IsDefault, &a.LastHealthCheckAt, &createdBy,
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
	// models is a JSON column (migrations/mysql/0003) standing in for
	// Postgres's TEXT[] — see that migration's comment. Always a JSON
	// array (never SQL NULL, column is NOT NULL DEFAULT (JSON_ARRAY())),
	// so an empty/absent value unmarshals to a nil slice, matching what a
	// Postgres TEXT[] '{}' scans to via pgx.
	if len(modelsJSON) > 0 {
		if err := json.Unmarshal(modelsJSON, &a.Models); err != nil {
			return domain.ProviderAccount{}, fmt.Errorf("mysql: unmarshal models json: %w", err)
		}
	}
	return a, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nonNilStringSlice guards against marshaling a Go nil slice to the JSON
// literal `null` — models is NOT NULL DEFAULT (JSON_ARRAY()), and while a
// JSON 'null' value is not itself a SQL NULL (so it wouldn't violate the
// NOT NULL constraint), it would be a silent semantic drift from "always
// an array." Mirrors internal/adapter/postgres's nonNilStringSlice, same
// purpose against MySQL's JSON column instead of a TEXT[] column.
func nonNilStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
