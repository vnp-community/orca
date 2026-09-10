# TASK-AIP-03-03: Add `DueHealthCheckClaimer`/`ClaimedHealthCheckBatch` ports + `UsageRepository.IncrementUsage`

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/ports.go`
**Depends on:** TASK-AIP-03-02
**Status:** `[x] DONE — DueHealthCheckClaimer/ClaimedHealthCheckBatch/OutboxEnqueuer + UsageRepository.IncrementUsage + ProviderAccountRepository.MarkQuotaWarningSent added; go build/vet clean (implementers land in 03-06).`

---

## Context

`ai-provider-service.md` §8 requires the health-check job to be "safe
under multiple replicas... `SELECT ... FOR UPDATE SKIP LOCKED` over due
accounts." `automation-service`'s `DueAutomationClaimer`/`ClaimedBatch`
already implement exactly this pattern against a different table
(`automation-service/internal/usecase/ports.go:72-83`) — this task adds
the same shape against `ai_provider.accounts`. `UsageRepository` also
needs a write method: it's currently read-only (`GetToday` only,
`ports.go:81-83`), so "no code path anywhere increments
`ai_provider.usage`" (this bug's own finding).

## Changes to make

In
`backend-go/services/ai-provider-service/internal/usecase/ports.go`, add:

```go
// ClaimedHealthCheckBatch mirrors automation-service's ClaimedBatch — the
// claim transaction stays open across dispatch (at-least-once: a crash
// mid-batch must not silently skip the next tick's retry).
type ClaimedHealthCheckBatch interface {
	Accounts() []domain.ProviderAccount
	// RecordResult persists one account's classification within the SAME
	// transaction the claim lock is held in.
	RecordResult(ctx context.Context, accountID string, status domain.AccountStatus, healthDetail *string, latencyMs *int, checkedAt time.Time) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DueHealthCheckClaimer — same SELECT...FOR UPDATE SKIP LOCKED shape as
// automation-service's DueAutomationClaimer.
type DueHealthCheckClaimer interface {
	ClaimDue(ctx context.Context, now time.Time, staleness time.Duration, limit int32) (ClaimedHealthCheckBatch, error)
}
```

Extend `UsageRepository`:

```go
type UsageRepository interface {
	GetToday(ctx context.Context, tenantID, accountID string, day time.Time) (domain.QuotaState, error)
	// IncrementUsage upserts today's rollup row (tokens/requests/cost added
	// to whatever's already there) and returns the POST-increment state, so
	// the caller can immediately compare against quota without a second read.
	IncrementUsage(ctx context.Context, tenantID, accountID string, day time.Time, tokensUsed int64, requestCount int64, costUSD float64) (domain.QuotaState, error)
}
```

Add an `OutboxEnqueuer` port (used by both `TASK-AIP-03-04` and
`TASK-AIP-03-05`):

```go
// OutboxEnqueuer is the narrow write port both ReconcileProviderHealth and
// RecordTokenUsage use to write into ai_provider.outbox within their own
// already-open transaction — same table SOL-AIP-01's CreateAccount writes
// its registration event into (TASK-AIP-01-07).
type OutboxEnqueuer interface {
	Enqueue(ctx context.Context, subject string, tenantID string, payload map[string]any) error
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go vet ./services/ai-provider-service/...
```

Expected: clean build. No implementers exist yet — `TASK-AIP-03-06` adds
the Postgres implementation; that's expected and resolved there.
