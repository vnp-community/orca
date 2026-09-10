# TASK-AIP-03-05: Add `RecordTokenUsage` usecase (quota enforcement write path)

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/record_token_usage.go` (new)
**Depends on:** TASK-AIP-03-03, TASK-AIP-03-02
**Status:** `[x] DONE — record_token_usage.go added, MarkQuotaWarningSent + UpdateStatusInput.HealthDetail added; all 3 requested tests pass.`

---

## Context

§8: quota writes are off the `Resolve` hot path — "`usage_daily` upserts
happen when the agent execution reports token counts back, not
synchronously inside `Resolve`." No code path anywhere calls
`IncrementUsage` today (this bug's own finding: "no code path anywhere
increments `ai_provider.usage`"). This adds the synchronous
`RecordTokenUsage` entry point — called by whichever service dispatched
the agent execution (`task-service`/`workflow-service`, per §7's "Called
by" table) right after a spawn completes — with 80%-warning and
100%-quota-exceeded alerting.

## Changes to make

Create
`backend-go/services/ai-provider-service/internal/usecase/record_token_usage.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

type RecordTokenUsageInput struct {
	AccountID    string
	TokensUsed   int64
	RequestCount int64
	CostUSD      float64
}

// RecordTokenUsage upserts today's rollup and enforces quota — off the
// Resolve hot path per §8, called synchronously by whichever service
// dispatched the agent execution once it completes.
type RecordTokenUsage struct {
	accounts ProviderAccountRepository
	usage    UsageRepository
	outbox   OutboxEnqueuer
	now      func() time.Time
}

func NewRecordTokenUsage(accounts ProviderAccountRepository, usage UsageRepository, outbox OutboxEnqueuer, now func() time.Time) *RecordTokenUsage {
	if now == nil {
		now = time.Now
	}
	return &RecordTokenUsage{accounts: accounts, usage: usage, outbox: outbox, now: now}
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

	if account.QuotaLimitDay == 0 { // 0 = unlimited
		return state, nil
	}

	ratio := float64(state.TokensUsed) / float64(account.QuotaLimitDay)
	switch {
	case ratio >= 1.0:
		detail := domain.HealthDetailQuotaExceeded
		if _, err := uc.accounts.UpdateStatus(ctx, UpdateStatusInput{
			TenantID: tenantID, AccountID: in.AccountID, Status: domain.AccountStatusError,
		}); err != nil {
			return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_FLIP_FAILED", "usage recorded but failed to flip status on quota exceeded", err)
		}
		payload := map[string]any{"account_id": in.AccountID, "tokens_used": state.TokensUsed, "quota_limit_day": account.QuotaLimitDay}
		if err := uc.outbox.Enqueue(ctx, "ai_provider.usage.quota_exceeded", tenantID, payload); err != nil {
			return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_ALERT_FAILED", "usage recorded but failed to enqueue quota-exceeded alert", err)
		}
		_ = detail // set via a dedicated MarkHealthDetail-style repo call if UpdateStatusInput doesn't carry HealthDetail yet — see TASK-AIP-03-03's port shape
	case ratio >= 0.8:
		if account.QuotaWarningSentDate == nil || !account.QuotaWarningSentDate.Equal(today) {
			if err := uc.accounts.MarkQuotaWarningSent(ctx, tenantID, in.AccountID, today); err != nil {
				return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_WARNING_MARK_FAILED", "usage recorded but failed to mark warning sent", err)
			}
			payload := map[string]any{"account_id": in.AccountID, "tokens_used": state.TokensUsed, "quota_limit_day": account.QuotaLimitDay}
			if err := uc.outbox.Enqueue(ctx, "ai_provider.usage.quota_warning", tenantID, payload); err != nil {
				return state, apperrors.New(apperrors.KindInternal, "AIPROVIDER_QUOTA_WARNING_ALERT_FAILED", "usage recorded but failed to enqueue quota-warning alert", err)
			}
		}
	}
	return state, nil
}
```

`MarkQuotaWarningSent` is a new one-off method needed on
`ProviderAccountRepository` (`ports.go`) — add it alongside the other
interface methods in this same task:

```go
// MarkQuotaWarningSent implements the 80%-warning idempotency guard — see
// TASK-AIP-03-01's quota_warning_sent_date column.
MarkQuotaWarningSent(ctx context.Context, tenantID, accountID string, day time.Time) error
```

`UpdateStatusInput` (`ports.go`) needs a `HealthDetail *string` field
added so the quota-exceeded flip can set `health_detail='quota_exceeded'`
in the same call as the status flip — extend it in this task:

```go
type UpdateStatusInput struct {
	TenantID           string
	AccountID          string
	Status             domain.AccountStatus
	HealthDetail       *string // NEW — nil = leave unchanged
	CredentialRef      string
	RotationGraceUntil *time.Time
}
```
Then set `HealthDetail: &detail` in the `UpdateStatus` call above instead
of the placeholder `_ = detail` line.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestRecordTokenUsage
```

Add to a new `record_token_usage_test.go`:
- `TestRecordTokenUsage_UnlimitedQuotaNeverFlips` — `QuotaLimitDay=0`,
  arbitrarily large `TokensUsed` → status untouched, no outbox event.
- `TestRecordTokenUsage_80PercentWarningOnce` — two calls crossing 80% in
  the same UTC day → exactly one `quota_warning` outbox event; a third
  call the next day (mocked `now`) → a second warning is allowed.
- `TestRecordTokenUsage_QuotaExceededFlipsStatusAndAlerts` — a call
  pushing the total to/over 100% → `UpdateStatus` called with
  `AccountStatusError`/`HealthDetailQuotaExceeded`, one `quota_exceeded`
  outbox event, `Resolvable()` on the refetched account is `false`.
