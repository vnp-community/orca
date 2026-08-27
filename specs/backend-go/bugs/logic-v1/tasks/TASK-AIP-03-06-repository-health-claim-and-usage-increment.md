# TASK-AIP-03-06: Postgres `ClaimDue`, `IncrementUsage`, `MarkQuotaWarningSent`, outbox `Enqueue`

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`
**Depends on:** TASK-AIP-03-01, TASK-AIP-03-03, TASK-AIP-03-05, TASK-AIP-01-07 (shares the `ai_provider.outbox` table SOL-AIP-01 creates)
**Status:** `[ ]` TODO

---

## Context

`TASK-AIP-03-03`'s ports (`DueHealthCheckClaimer`, `UsageRepository.
IncrementUsage`, `OutboxEnqueuer`) and `TASK-AIP-03-05`'s
`MarkQuotaWarningSent` have no Postgres implementation yet. `ClaimDue`
mirrors `AutomationRepository.ClaimDue`'s `SELECT ... FOR UPDATE SKIP
LOCKED` pattern
(`backend-go/services/automation-service/internal/adapter/postgres/repository.go:129-152`)
exactly, against `ai_provider.accounts` instead of
`automation.automations` — no tenant filter, same reasoning
automation-service's own comment gives: the scheduler scans across every
tenant on a timer, and every row it returns still carries its own
`tenant_id`.

## Changes to make

Add to
`backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`:

```go
type healthCheckBatch struct {
	tx       pgx.Tx
	accounts []domain.ProviderAccount
}

func (b *healthCheckBatch) Accounts() []domain.ProviderAccount { return b.accounts }

func (b *healthCheckBatch) RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error {
	_, err := b.tx.Exec(ctx, `
		UPDATE ai_provider.accounts
		SET status = $3, health_detail = $4, latency_ms = $5, last_health_check_at = $6, updated_at = now()
		WHERE id = $1 AND tenant_id = $2
	`, accountID, accountID /* placeholder — see note below */, string(status), healthDetail, latencyMs, checkedAt)
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
	rows, err := tx.Query(ctx, `
		SELECT id, tenant_id, provider_type, status, credential_ref,
		       scope, user_id, project_id, dev_server_id, label, model_hint, base_url,
		       quota_limit_day, models, is_default, last_health_check_at, created_by,
		       rotation_grace_until, created_at, updated_at
		FROM ai_provider.accounts
		WHERE status = 'active' AND deleted_at IS NULL
		  AND (last_health_check_at IS NULL OR last_health_check_at <= $1 - $2::interval)
		ORDER BY last_health_check_at NULLS FIRST
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`, now, staleness.String(), limit)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("postgres: query due-for-health-check accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.ProviderAccount
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("postgres: scan due-for-health-check account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("postgres: iterate due-for-health-check rows: %w", err)
	}
	return &healthCheckBatch{tx: tx, accounts: accounts}, nil
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

// Enqueue implements usecase.OutboxEnqueuer — same ai_provider.outbox
// table TASK-AIP-01-07's insertOutboxEvent writes into, reused here rather
// than adding a second table.
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
```

Fix the placeholder in `RecordResult`'s `WHERE` clause above — it should
bind `$2` to the batch's own tracked `tenantID` per account (thread
`tenantID` through `healthCheckBatch.accounts` or store a parallel
`map[string]string` of accountID→tenantID captured during `ClaimDue`'s
scan), not `accountID` twice; write this correctly in the actual
implementation rather than copying the placeholder verbatim.

Update `cmd/server/main.go`'s `usecase.New...` wiring to construct
`ReconcileProviderHealth`/`RecordTokenUsage` with `repo` satisfying all
the new ports (`var _ usecase.DueHealthCheckClaimer = (*Repository)(nil)`
etc.) — full ticker/goroutine wiring is `TASK-AIP-03-07`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./...
go test ./services/ai-provider-service/internal/adapter/postgres/... -run 'TestClaimDue|TestIncrementUsage'
```

Add (integration, `testcontainers-go`):
- `TestClaimDue_NoDoubleClaimUnderConcurrency` — two goroutines calling
  `ClaimDue` against the same due rows concurrently never claim the same
  row (assert the union of both goroutines' claimed IDs has no
  duplicates) — the core §8 correctness requirement, tested against
  Postgres's actual lock semantics.
- `TestIncrementUsage_AdditiveAcrossCalls` — three calls of 100 tokens
  each in the same day → 300, not 100.
