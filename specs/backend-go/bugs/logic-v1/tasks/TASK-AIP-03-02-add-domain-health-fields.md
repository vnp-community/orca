# TASK-AIP-03-02: Add `LatencyMs`/`HealthDetail`/`QuotaWarningSentDate` to domain, `TokensUsed` to `QuotaState`

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/domain/provider_account.go`
**Depends on:** TASK-AIP-03-01, TASK-AIP-01-03 (this task's struct literal builds on TASK-AIP-01-03's field additions)
**Status:** `[ ]` TODO

---

## Context

`ProviderAccount` needs the 3 new health/quota-alert fields
`TASK-AIP-03-01`'s migration adds columns for, and `QuotaState` needs a
`TokensUsed` field since quota enforcement (`TASK-AIP-03-05`) is
token-based, not request-count-based. `Resolvable()`'s existing one-line
`status == active` check needs no change — reusing `status='error'` for
every health-check failure class (rather than adding new lifecycle states)
is exactly what makes that method correct without modification; this
closes BUG-AIP-02's `status='healthy'`-filtering dependency for free.

## Changes to make

In
`backend-go/services/ai-provider-service/internal/domain/provider_account.go`,
extend `ProviderAccount` (after the fields `TASK-AIP-01-03` added):

```go
type ProviderAccount struct {
	// ... existing + TASK-AIP-01-03's fields ...
	LatencyMs            *int       // NEW
	HealthDetail         *string    // NEW — "healthy"|"degraded"|"quota_exceeded"|"invalid_key"|"unreachable"|nil (never checked)
	QuotaWarningSentDate *time.Time // NEW — UTC calendar day, nil = not sent today
}

// HealthDetail* — typed constants mirroring the CHECK constraint added in
// TASK-AIP-03-01, used instead of bare strings at every call site.
const (
	HealthDetailHealthy       = "healthy"
	HealthDetailDegraded      = "degraded"
	HealthDetailQuotaExceeded = "quota_exceeded"
	HealthDetailInvalidKey    = "invalid_key"
	HealthDetailUnreachable   = "unreachable"
)

// derefHealthDetail returns a.HealthDetail's value, or "" if nil — a small
// convenience so callers comparing against the constants above don't each
// need their own nil-check.
func (a ProviderAccount) derefHealthDetail() string {
	if a.HealthDetail == nil {
		return ""
	}
	return *a.HealthDetail
}

// Resolvable — UNCHANGED. A health-check failure flips Status to
// AccountStatusError (see reconcile_provider_health.go, TASK-AIP-03-04),
// which this existing one-line check already filters — closes BUG-AIP-02's
// "no status='healthy' filtering" dependency without widening this method.
func (a ProviderAccount) Resolvable() bool {
	return a.Status == AccountStatusActive
}
```

Extend `QuotaState`:

```go
type QuotaState struct {
	AccountID    string
	Date         time.Time
	CostUSD      float64
	RequestCount int64
	TokensUsed   int64 // NEW — matches ai_provider.usage.tokens_used (TASK-AIP-03-01)
}
```

Update `NewProviderAccount`'s signature (extended by `TASK-AIP-01-03`) if
this task lands after it — add `latencyMs *int, healthDetail *string,
quotaWarningSentDate *time.Time` as trailing parameters, defaulting to
`nil` at every existing call site outside the health-check/quota code
paths (those two usecases are the only writers).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/domain/... -run TestProviderAccount_Resolvable
```

Add `TestProviderAccount_Resolvable_ExcludesAllHealthFailures` —
`Status=AccountStatusError` with every `HealthDetail` value (including
`nil`) all return `false` from `Resolvable()` — regression guard that
excluding health failures never needs a second check anywhere else in the
codebase.
