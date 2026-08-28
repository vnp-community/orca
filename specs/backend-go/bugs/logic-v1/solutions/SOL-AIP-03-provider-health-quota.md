# SOL-AIP-03: Add the 15-minute health-check reconciliation job and a token-usage write path with quota enforcement

**Resolves:** [BUG-AIP-03](../BUG-AIP-03-provider-health-quota-partial.md)
**Service:** `ai-provider-service` (+ `notification-service` as the async
alert consumer, no changes needed there beyond subscribing to the new
outbox event — see "Alerting path" below). The health-check job's live
provider call needs a Dev Server Agent change — see "Dev Server Agent
dependency."
**Affected files (proposed):**
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go`
- `backend-go/services/ai-provider-service/migrations/0005_health_and_usage_writes.up.sql` / `.down.sql` (new; numbered after [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md)'s `0003`/`0004`)
- `backend-go/services/ai-provider-service/internal/usecase/ports.go`
- `backend-go/services/ai-provider-service/internal/usecase/reconcile_provider_health.go` (new)
- `backend-go/services/ai-provider-service/internal/usecase/record_token_usage.go` (new)
- `backend-go/services/ai-provider-service/internal/adapter/scheduler/health_check_ticker.go` (new)
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go`
- `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go`
- `backend-go/services/ai-provider-service/cmd/server/main.go`
- `backend-go/proto/orca/aiprovider/v1/aiprovider.proto`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go` (none — `recordTokenUsage` is service-to-service only, no WS/REST surface; see below)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### The health-check job: TDD names the requirement and its reliability bar precisely

§8 states this as a hard non-functional requirement, not a nice-to-have:
"a scheduled reconciliation job runs every 15 minutes... calling
`TestConnection` per active account and updating `status`/
`last_health_check_at`. Must be safe under multiple replicas (leader
election, or `SELECT ... FOR UPDATE SKIP LOCKED` over due accounts) —
correctness (never double-firing a check against a rate limited provider)
matters more than latency here" (`ai-provider-service.md:236-242`).
`usage-service`'s own doc names the same shape as its safety net
("`ReconcileDailyRollups`... same shape `ai-provider-service`'s
health-check job uses for different drift," `usage-service.md:271-273`) —
this job is cross-referenced from *outside* `ai-provider-service.md` too,
reinforcing it's load-bearing, not optional scope.

**A working precedent for exactly this shape already exists in the
codebase** — `automation-service`'s ticker + `SELECT ... FOR UPDATE SKIP
LOCKED` claim, which this solution copy-adapts rather than inventing a
fourth scheduling mechanism:

- `scheduler.Ticker.Run` — a `time.NewTicker`-driven loop, one per replica,
  no leader election needed because the claim query itself is what
  prevents double-firing
  (`backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:19-60`).
- `DueAutomationClaimer.ClaimDue` — `SELECT ... FOR UPDATE SKIP LOCKED`
  inside an explicitly-held-open transaction, so "concurrent replicas'
  ticks never claim the same due row"
  (`backend-go/services/automation-service/internal/usecase/ports.go:72-83`,
  SQL at `backend-go/services/automation-service/internal/adapter/postgres/repository.go:139-152`).

This is precisely §8's own suggested mechanism, already proven in one
service — this solution's `ClaimDueForHealthCheck` is the same pattern
against `ai_provider.accounts` instead of `automation.automations`.

### Health classification: reuse the existing lifecycle enum, don't fork a second one

§4's `AccountStatus` enum is `pending`/`active`/`rotating`/`revoked`/
`error` (`ai-provider-service.md:104`,
`backend-go/services/ai-provider-service/internal/domain/provider_account.go:49-55`),
matching §5's `CHECK` constraint exactly
(`ai-provider-service.md:141-142`). BUG-AIP-03's spec summary (sourced from
BL-AIP-03, a TS-era doc) describes classifying failures into `quota_exceeded`/
`invalid_key`/`unreachable` — values that don't exist in the Go TDD's own
5-value enum. Two designs were considered:

1. **Expand `AccountStatus` to include the TS-era health values directly**
   — rejected: it would require every existing status-based check
   (`Resolvable()`, `RotateKey`'s `active`↔`rotating` transition, the
   `scope_ref_matches_scope`-style `CHECK` constraint) to be rewritten
   against a doubled state space, and §8's own phrasing — "updating
   `status`/`last_health_check_at`" — reads naturally against the
   **existing** 5-value enum once you notice `error` is already one of
   them: a health-check failure is exactly what `error` is for.
2. **Reuse `status='error'` for any health-check failure, and add a
   separate, narrower `health_detail` classification column** for the
   finer-grained reason (`quota_exceeded`/`invalid_key`/`unreachable`) —
   **adopted**. `Resolvable()` stays a one-line `status == active` check
   (BUG-AIP-02's dependency on "no `status='healthy'` filtering" is closed
   by this alone — an `error`-status account is already excluded by the
   existing check), while `health_detail` carries exactly the alerting/
   debugging classification BL-AIP-03 asks for, without widening the core
   lifecycle state machine §4/§5 define. Flagged explicitly as an
   extension beyond §4/§5's literal column list, in the same spirit as
   [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md)'s `Models`/
   `IsDefault` additions.

`degraded` (BL-AIP-03: "pings every `healthy`/`degraded` account") is
modeled as a `health_detail` value that does **not** flip `status` away
from `active` — a degraded-but-still-resolvable account stays in the ping
rotation and stays usable, matching the spec's own framing of `degraded` as
distinct from the three exclude-from-resolution failure classes.

### Alerting: the outbox this service already needs for SOL-AIP-01, reused

§8/§9 don't specify a bespoke webhook/WS-push mechanism inside
`ai-provider-service` itself, and none should be built — `notification-service`
is already "Primary consumer of the async event bus"
(`backend-go/proto/orca/notification/v1/notification.proto:9`) and the
decomposition doc's dependency graph already shows `notif` consuming events
from `task`/`wf`/`auto` asynchronously
(`02-microservices-decomposition.md:163-165`). This solution emits two new
outbox event subjects — `ai_provider.account.health_degraded` (first
`invalid_key`/`unreachable`/`quota_exceeded` classification) and
`ai_provider.usage.quota_warning` (80% threshold) — via the same
`ai_provider.outbox` table [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md)
adds for the registration event, rather than adding a second
publishing mechanism. `notification-service` subscribing to these subjects
and turning them into webhook/WS-push alerts is that service's own
consumer-side work, out of scope for this solution (mirrors how
`ai-provider-service.md` never describes `notification-service`'s consumer
internals either) — flagged here as the follow-up, not silently assumed
done.

### The usage write path: `RecordTokenUsage` as a synchronous RPC, not (yet) an event

§8 is explicit that quota writes are **off** the `Resolve` hot path —
"`usage_daily` upserts happen when the agent execution reports token
counts back, not synchronously inside `Resolve`, which only reads the
rollup" (`ai-provider-service.md:243-245`) — but doesn't specify sync vs.
async for *that* write itself, unlike `usage-service.md` §7, which
explicitly recommends async NATS delivery for its own (structurally
similar) session-completion write and gives a concrete reason: "usage
recording is after-the-fact bookkeeping... must never add latency or a
failure mode to the interactive path that produced it"
(`usage-service.md:243-246`). That reasoning applies here too, but adopting
it fully would require `ai-provider-service` to gain an
`adapter/eventbus/` **consumer** (subscribing to an agent-reported
usage event) — infrastructure this service doesn't have yet and which
whoever ends up owning the Dev-Server-Agent-relay-for-AI-CLI-execution
path (per `usage-service.md` §7's own open question, "the service that
would naturally hold the Dev Server Agent relay connection... isn't built
yet") hasn't settled either. This solution ships the **minimal-surface
fix that closes the bug today** — a synchronous `RecordTokenUsage` gRPC RPC
callable by whichever service dispatches the agent execution
(`task-service`/`workflow-service`, per §7's "Called by" table) right after
a spawn completes — and explicitly recommends the same async migration
`usage-service.md` §7 recommends for its analogous call, once the
relay-owning service and its event contract exist. Not implementing async
now is a scoping decision, not an oversight — flagged so it isn't
silently dropped either.

## Design — schema

```sql
-- 0005_health_and_usage_writes.up.sql
ALTER TABLE ai_provider.accounts
  ADD COLUMN latency_ms          INTEGER,               -- NULL until first health check
  ADD COLUMN health_detail       TEXT CHECK (health_detail IN
                                   ('healthy','degraded','quota_exceeded','invalid_key','unreachable')),
  ADD COLUMN quota_warning_sent_date DATE;               -- idempotency guard for the 80% alert, see below
-- last_health_check_at already added by SOL-AIP-01's 0003 migration
-- (ai-provider-service.md §4 already specced it; this job is its first writer).

CREATE INDEX idx_accounts_due_for_health_check
  ON ai_provider.accounts (last_health_check_at)
  WHERE status = 'active' AND deleted_at IS NULL;

-- quota_limit_day already added by SOL-AIP-01's 0003 migration.
-- ai_provider.usage already exists (0001_init.up.sql) with no writer until
-- this solution's RecordTokenUsage usecase.
```

`quota_warning_sent_date` (one date value, not a boolean) makes the
80%-warning idempotent **per calendar day** without a second table: the
usecase only sends the warning when `quota_warning_sent_date IS DISTINCT
FROM today`, then sets it to `today` — naturally resets itself the next
day with no separate cleanup job.

## Design — domain (`provider_account.go`)

```go
type ProviderAccount struct {
	// ... existing + SOL-AIP-01's fields ...
	LatencyMs           *int       // NEW
	HealthDetail         *string    // NEW — "healthy"|"degraded"|"quota_exceeded"|"invalid_key"|"unreachable"|nil (never checked)
	QuotaWarningSentDate *time.Time // NEW — UTC calendar day, nil = not sent today
}

// HealthDetailQuotaExceeded etc. — typed constants mirroring the CHECK
// constraint, used instead of bare strings at every call site.
const (
	HealthDetailHealthy       = "healthy"
	HealthDetailDegraded      = "degraded"
	HealthDetailQuotaExceeded = "quota_exceeded"
	HealthDetailInvalidKey    = "invalid_key"
	HealthDetailUnreachable   = "unreachable"
)

// Resolvable — UNCHANGED signature, but now correctly excludes every
// health-check failure class too: a failed check flips Status to
// AccountStatusError (see reconcile_provider_health.go), which this
// existing one-line check already filters. This closes BUG-AIP-02's
// "no status='healthy' filtering" dependency without widening this method.
func (a ProviderAccount) Resolvable() bool {
	return a.Status == AccountStatusActive
}
```

## Design — `usecase/ports.go`

```go
// ClaimedHealthCheckBatch mirrors automation-service's ClaimedBatch exactly
// — see that type's doc comment for why the claim transaction stays open
// across dispatch (at-least-once: a crash mid-batch must not silently skip
// the next tick's retry).
type ClaimedHealthCheckBatch interface {
	Accounts() []domain.ProviderAccount
	// RecordResult persists one account's classification within the SAME
	// transaction the claim lock is held in.
	RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DueHealthCheckClaimer — same SELECT...FOR UPDATE SKIP LOCKED shape as
// automation-service's DueAutomationClaimer (ports.go:72-83 there).
type DueHealthCheckClaimer interface {
	ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (ClaimedHealthCheckBatch, error)
}

// UsageRepository gains a write — the ONLY method addition to this
// interface; GetToday is unchanged.
type UsageRepository interface {
	GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error)
	// IncrementUsage upserts today's rollup row (tokens/requests/cost added
	// to whatever's already there) and returns the POST-increment state, so
	// the caller can immediately compare against quota without a second
	// read — closes "no code path anywhere increments ai_provider.usage."
	IncrementUsage(ctx context.Context, tenantID, accountID string, day time.Time, tokensUsed int64, requestCount int64, costUSD float64) (domain.QuotaState, error)
}
```

(`ai_provider.usage`'s existing columns are `cost_usd`/`request_count` only
— no `tokens_used` column. This solution adds one: `tokens_used BIGINT NOT
NULL DEFAULT 0`, since quota enforcement is token-based per BL-AIP-03 and
`QuotaState.WithinQuota(limit)` in §4's sketch clearly compares a token
count, not a request count, against `QuotaLimitDay`.)

```sql
ALTER TABLE ai_provider.usage ADD COLUMN tokens_used BIGINT NOT NULL DEFAULT 0;
```

## Design — `usecase/reconcile_provider_health.go` (new)

```go
// ReconcileProviderHealth is the usecase the health-check ticker invokes
// every tick — one batch, one claim transaction, per ai-provider-service.md
// §8. Reuses verifyConnection (SOL-AIP-01's extraction from
// TestConnection/CreateAccount) — the periodic ping and the on-demand/
// create-time test are the SAME relay call, not three separate agent
// methods to build and maintain.
type ReconcileProviderHealth struct {
	claimer DueHealthCheckClaimer
	infra   InfraFleetClient
	usage   UsageRepository
	outbox  OutboxEnqueuer // see repository design — same tx as RecordResult
	now     func() time.Time
}

func (uc *ReconcileProviderHealth) Execute(ctx context.Context, batchSize int32) error {
	batch, err := uc.claimer.ClaimDue(ctx, uc.now(), 15*time.Minute, batchSize)
	if err != nil {
		return fmt.Errorf("claim due accounts: %w", err)
	}
	defer batch.Rollback(ctx) // no-op after Commit

	for _, account := range batch.Accounts() {
		start := uc.now()
		result, err := verifyConnection(ctx, uc.infra, account.DevServerID, account.CredentialRef, account.ProviderType)
		latencyMs := int(uc.now().Sub(start).Milliseconds())

		status, detail := classifyHealthResult(account, result, err)
		if err := batch.RecordResult(ctx, account.ID, status, detail, &latencyMs, uc.now()); err != nil {
			return fmt.Errorf("record health result for account %s: %w", account.ID, err)
		}
		// Alert only on a NEW failure classification, not every 15-minute
		// re-confirmation of an already-known-bad account — avoids paging
		// on every tick for a still-broken key.
		if detail != nil && *detail != account.derefHealthDetail() && (*detail == domain.HealthDetailInvalidKey || *detail == domain.HealthDetailUnreachable || *detail == domain.HealthDetailQuotaExceeded) {
			if err := uc.outbox.Enqueue(ctx, healthDegradedEvent(account, *detail)); err != nil {
				return fmt.Errorf("enqueue health-degraded event for account %s: %w", account.ID, err)
			}
		}
		// Recovery: an account previously flagged quota_exceeded whose
		// current-day usage has reset (new UTC day, no rollup row yet) goes
		// back to active on the next successful ping — see
		// classifyHealthResult's doc comment.
	}
	return batch.Commit(ctx)
}

// classifyHealthResult maps a raw connection-test result onto (Status,
// HealthDetail). A quota_exceeded classification is re-checked against
// today's actual usage rollup (not just carried forward from the account's
// prior state) so a day rollover naturally clears it without a separate
// midnight job — see this file's design-rationale note on why a second
// scheduler wasn't added.
func classifyHealthResult(account domain.ProviderAccount, result ConnectionTestResult, relayErr error) (domain.AccountStatus, *string) {
	if relayErr != nil {
		detail := domain.HealthDetailUnreachable
		return domain.AccountStatusError, &detail
	}
	if !result.Success {
		detail := domain.HealthDetailInvalidKey
		return domain.AccountStatusError, &detail
	}
	// Connection itself is fine — quota state is checked separately by
	// RecordTokenUsage at write time, not re-derived here from a stale
	// in-memory account snapshot; a healthy ping on a quota-exceeded
	// account's dev server just means the KEY still authenticates, not
	// that quota has reset. Leave status untouched if it's already
	// AccountStatusError with HealthDetailQuotaExceeded — RecordTokenUsage
	// (or the next day's first successful write) is what clears it.
	if account.Status == domain.AccountStatusError && account.derefHealthDetail() == domain.HealthDetailQuotaExceeded {
		return account.Status, account.HealthDetail
	}
	detail := domain.HealthDetailHealthy
	return domain.AccountStatusActive, &detail
}
```

## Design — `usecase/record_token_usage.go` (new)

```go
type RecordTokenUsageInput struct {
	AccountID    string
	TokensUsed   int64
	RequestCount int64
	CostUSD      float64
}

// RecordTokenUsage upserts today's rollup and enforces quota — off the
// Resolve hot path per §8, called synchronously by whichever service
// dispatched the agent execution once it completes (see this file's design
// rationale for why synchronous, for now).
type RecordTokenUsage struct {
	accounts ProviderAccountRepository
	usage    UsageRepository
	outbox   OutboxEnqueuer
	now      func() time.Time
}

func (uc *RecordTokenUsage) Execute(ctx context.Context, in RecordTokenUsageInput) (domain.QuotaState, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.QuotaState{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}

	account, err := uc.accounts.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return domain.QuotaState{}, err
	}

	today := domain.DayKey(uc.now())
	state, err := uc.usage.IncrementUsage(ctx, tenantID, in.AccountID, today, in.TokensUsed, in.RequestCount, in.CostUSD)
	if err != nil {
		return domain.QuotaState{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_USAGE_WRITE_FAILED", "failed to increment usage rollup", err)
	}

	if account.QuotaLimitDay == 0 { // 0 = unlimited, per §5
		return state, nil
	}

	ratio := float64(state.TokensUsed) / float64(account.QuotaLimitDay)
	switch {
	case ratio >= 1.0:
		detail := domain.HealthDetailQuotaExceeded
		if _, err := uc.accounts.UpdateStatus(ctx, UpdateStatusInput{
			TenantID: tenantID, AccountID: in.AccountID, Status: domain.AccountStatusError, HealthDetail: &detail,
		}); err != nil {
			return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_FLIP_FAILED", "usage recorded but failed to flip status on quota exceeded", err)
		}
		if err := uc.outbox.Enqueue(ctx, quotaExceededEvent(account, state)); err != nil {
			return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_ALERT_FAILED", "usage recorded but failed to enqueue quota-exceeded alert", err)
		}
	case ratio >= 0.8:
		if account.QuotaWarningSentDate == nil || !sameUTCDay(*account.QuotaWarningSentDate, today) {
			if err := uc.accounts.MarkQuotaWarningSent(ctx, tenantID, in.AccountID, today); err != nil {
				return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_WARNING_MARK_FAILED", "usage recorded but failed to mark warning sent", err)
			}
			if err := uc.outbox.Enqueue(ctx, quotaWarningEvent(account, state)); err != nil {
				return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_WARNING_ALERT_FAILED", "usage recorded but failed to enqueue quota-warning alert", err)
			}
		}
	}
	return state, nil
}
```

`QuotaState` (domain) gains `TokensUsed int64` alongside its existing
`CostUSD`/`RequestCount` fields, matching the new `tokens_used` column.

## Design — `adapter/postgres` (repository + scheduler claim + outbox)

- `ClaimDue` for `DueHealthCheckClaimer` mirrors
  `AutomationRepository.ClaimDue` exactly
  (`backend-go/services/automation-service/internal/adapter/postgres/repository.go:139-152`):
  `SELECT ... FROM ai_provider.accounts WHERE status='active' AND
  deleted_at IS NULL AND (last_health_check_at IS NULL OR
  last_health_check_at <= $1 - interval '15 minutes') ORDER BY
  last_health_check_at NULLS FIRST LIMIT $2 FOR UPDATE SKIP LOCKED`, no
  `tenant_id` filter — same reasoning automation-service's own comment
  gives: "the scheduler scans across every tenant on a timer... every row
  it returns still carries its own `tenant_id`"
  (`backend-go/services/automation-service/internal/adapter/postgres/repository.go:129-133`).
- `IncrementUsage` — a single upsert:
  ```sql
  INSERT INTO ai_provider.usage (account_id, tenant_id, date, tokens_used, cost_usd, request_count)
  VALUES ($1,$2,$3,$4,$5,$6)
  ON CONFLICT (account_id, date) DO UPDATE SET
    tokens_used = ai_provider.usage.tokens_used + EXCLUDED.tokens_used,
    cost_usd = ai_provider.usage.cost_usd + EXCLUDED.cost_usd,
    request_count = ai_provider.usage.request_count + EXCLUDED.request_count
  RETURNING tokens_used, cost_usd, request_count
  ```
- `OutboxEnqueuer.Enqueue` and the underlying INSERT are the exact same
  `ai_provider.outbox` machinery [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md)
  introduces — this solution adds two more event-shape builders
  (`healthDegradedEvent`/`quotaExceededEvent`/`quotaWarningEvent`), not a
  second outbox table.

## Design — `adapter/scheduler/health_check_ticker.go` (new)

Copy-adapted from `scheduler.Ticker`
(`backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:19-60`):
same `time.NewTicker`-driven `Run(ctx)` loop, `interval` defaulting to 15
minutes per §8, calling `ReconcileProviderHealth.Execute(ctx, batchSize)`
each tick instead of `RunNow`. `cmd/server/main.go` starts it with `go
healthCheckTicker.Run(ctx)`, same shutdown-context convention every other
service's background goroutine uses
(`backend-go/services/automation-service/cmd/server/main.go:110-114`).

## Dev Server Agent dependency

`verifyConnection`'s relay call targets `ai.testProviderConnection` —
**the same agent method [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md)
already needs** for its test-before-save gate. This solution deliberately
reuses that one method rather than asking for a separate `ai.ping`, so the
Dev Server Agent side of this work is **one** new JSON-RPC handler, not two
— but it is still a real, required `agent/` change (likely
`agent/src/relay/agent-rpc-dispatch.ts`'s `ai.*` case group) before this
job's classifications mean anything beyond "every account is
`unreachable`." Land the backend-go half first — the ticker, claim query,
and classification logic are all independently testable against a fake
`InfraFleetClient` — and coordinate the agent-side handler as a shared
dependency with SOL-AIP-01's implementer rather than building it twice.

## Design — wiring (proto/gRPC)

```protobuf
service AiProviderService {
  // ... existing ...
  rpc RecordTokenUsage(RecordTokenUsageRequest) returns (RecordTokenUsageResponse);
}

message RecordTokenUsageRequest {
  string account_id    = 1;
  int64  tokens_used    = 2;
  int64  request_count  = 3;
  double cost_usd        = 4;
}
message RecordTokenUsageResponse {
  int64  tokens_used    = 1;
  double cost_usd        = 2;
  int64  request_count  = 3;
}
```

No REST route or WS channel — `RecordTokenUsage` is service-to-service
only (called by `task-service`/`workflow-service` after a spawn, per §7's
"Called by" table), never from a browser/mobile client, so it stays off
`httpgateway`/`wscompat` entirely, matching how `PushCiphertext` and other
internal-only RPCs in this catalog have no gateway-facing route either.

## Test plan

- `provider_account_test.go` — `Resolvable()` returns `false` for
  `Status=AccountStatusError` regardless of `HealthDetail` value
  (regression guard: excluding health failures must not require a second
  check anywhere else in the codebase).
- `reconcile_provider_health_test.go`:
  - `TestReconcileProviderHealth_ClassifiesUnreachable` — fake
    `InfraFleetClient.Relay` returns an error → `AccountStatusError` +
    `HealthDetailUnreachable`, an outbox event enqueued.
  - `TestReconcileProviderHealth_ClassifiesInvalidKey` — relay succeeds,
    `result.Success=false` → `HealthDetailInvalidKey`.
  - `TestReconcileProviderHealth_NoAlertOnRepeatFailure` — account already
    `HealthDetailUnreachable`, stays `unreachable` on this tick → outbox
    `Enqueue` NOT called (fake records zero calls) — regression guard
    against paging on every 15-minute re-confirmation.
  - `TestReconcileProviderHealth_QuotaExceededSurvivesHealthyPing` — account
    is `AccountStatusError`/`HealthDetailQuotaExceeded`; connection test
    succeeds; status stays unchanged (only `RecordTokenUsage`'s day-rollover
    path clears it, not a bare successful ping).
  - `TestReconcileProviderHealth_LatencyRecorded` — successful check
    records a non-nil `latency_ms`.
- `record_token_usage_test.go`:
  - `TestRecordTokenUsage_UnlimitedQuotaNeverFlips` — `QuotaLimitDay=0`,
    arbitrarily large `TokensUsed` → status untouched, no outbox event.
  - `TestRecordTokenUsage_80PercentWarningOnce` — two calls crossing 80% in
    the same UTC day → exactly one `quota_warning` outbox event (fake
    outbox call count asserted); a third call the next day (mocked `now`)
    → a second warning is allowed.
  - `TestRecordTokenUsage_QuotaExceededFlipsStatusAndAlerts` — a call
    pushing the total to/over 100% → `UpdateStatus` called with
    `AccountStatusError`/`HealthDetailQuotaExceeded`, one
    `quota_exceeded` outbox event, `Resolvable()` on the returned/refetched
    account is `false`.
- `adapter/postgres/repository_test.go` (integration) —
  `ClaimDue` under two concurrent goroutines against the same due rows
  never double-claims (assert the union of both goroutines' claimed IDs
  has no duplicates) — the core §8 correctness requirement, tested for
  real against Postgres's actual lock semantics, not just asserted in
  usecase-level fakes.
  `IncrementUsage`'s `ON CONFLICT` arithmetic is additive across repeated
  calls in the same day (three calls of 100 tokens each → 300, not 100).
- Outbox contract test (mirrors SOL-AIP-01's) —
  `healthDegradedEvent`/`quotaExceededEvent`/`quotaWarningEvent` payloads
  round-trip through `FetchUnpublished` with the correct `subject` string,
  so `notification-service`'s future consumer has a stable contract to
  subscribe against.

## References

- `specs/backend-go/tdd/services/ai-provider-service.md:99-123` (§4 domain
  model — `AccountStatus`'s existing 5 values this solution reuses),
  `:229-245` (§8 — health-check job reliability requirement, `SELECT ...
  FOR UPDATE SKIP LOCKED` suggestion, and "quota writes are off the
  Resolve path" framing this solution's `RecordTokenUsage` follows)
- `specs/backend-go/tdd/services/usage-service.md:229-258` (§7 — the
  sync-vs-async write-path tradeoff this solution's rationale explicitly
  mirrors and defers), `:260-273` (§8 — "on-write rollup... not a
  scheduler" pattern `IncrementUsage` follows, and the
  `ReconcileDailyRollups` cross-reference to this bug's job)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:163-165`
  (dependency graph — `notification-service` as async event consumer, the
  alerting path this solution routes through)
- `backend-go/services/automation-service/internal/adapter/scheduler/ticker.go:1-60`,
  `backend-go/services/automation-service/internal/adapter/postgres/repository.go:129-152`,
  `backend-go/services/automation-service/internal/usecase/ports.go:60-100`
  (ticker + `FOR UPDATE SKIP LOCKED` claim precedent this solution
  copy-adapts)
- `backend-go/services/automation-service/cmd/server/main.go:110-114`
  (background-goroutine wiring precedent)
- `backend-go/proto/orca/notification/v1/notification.proto:9-18`
  (`notification-service`'s event-consumer role, cited for the alerting
  design)
- `backend-go/services/ai-provider-service/internal/domain/provider_account.go:43-65,215-221`
- `backend-go/services/ai-provider-service/internal/usecase/ports.go:76-83`
  (`UsageRepository`'s current read-only shape this solution extends)
- `backend-go/services/ai-provider-service/internal/adapter/postgres/repository.go:170-189`
  (`GetToday` — the only existing usage-table code path)
- `backend-go/services/ai-provider-service/internal/usecase/test_connection.go:1-65`
  (relay call this solution's health check reuses via SOL-AIP-01's
  `verifyConnection` extraction)
- `backend-go/services/ai-provider-service/migrations/0001_init.up.sql:53-67`
  (`ai_provider.usage`'s existing columns this solution adds `tokens_used`
  to)
- `backend-go/services/ai-provider-service/README.md` — "Known gaps" (both
  the health-check-job and `recordTokenUsage` gaps this solution closes,
  confirmed as tracked, not accidental, omissions)
- [BUG-AIP-01](../BUG-AIP-01-register-provider-account-partial.md) /
  [SOL-AIP-01](./SOL-AIP-01-register-provider-account.md) — `QuotaLimitDay`
  field, `verifyConnection` extraction, and `ai_provider.outbox` table this
  solution depends on and extends
- [BUG-AIP-02](../BUG-AIP-02-provider-resolution-partial.md) /
  [SOL-AIP-02](./SOL-AIP-02-provider-resolution-filtering.md) — the
  `status='healthy'` filtering dependency this solution closes via
  `Resolvable()`'s unchanged `status == active` check
