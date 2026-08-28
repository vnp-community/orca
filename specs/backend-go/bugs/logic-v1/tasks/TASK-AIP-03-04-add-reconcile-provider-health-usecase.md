# TASK-AIP-03-04: Add `ReconcileProviderHealth` usecase

**From Solution:** SOL-AIP-03
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `backend-go/services/ai-provider-service/internal/usecase/reconcile_provider_health.go` (new)
**Depends on:** TASK-AIP-03-03, TASK-AIP-01-05 (reuses `verifyConnection`), TASK-AIP-SHARED-01
**Status:** `[x] DONE — reconcile_provider_health.go added (HealthDetailOrEmpty naming, per this task's own tie-break note); all 5 requested tests + a claim-error-propagation test pass.`

---

## Context

§8's 15-minute health-check job: claim a batch of due accounts, ping each
via the same relay call `TestConnection`/`CreateAccount`'s test-before-save
gate already use (`verifyConnection`, `TASK-AIP-01-05` — one agent method
to build and maintain, not three), classify the result, and alert on a
**new** failure classification only (not every re-confirmation of an
already-known-bad account, to avoid paging on every tick for a
still-broken key). A `quota_exceeded` classification is re-checked against
today's actual usage rollup rather than carried forward blindly, so a UTC
day rollover naturally clears it without a separate midnight job.

## Changes to make

Create
`backend-go/services/ai-provider-service/internal/usecase/reconcile_provider_health.go`:

```go
package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// ReconcileProviderHealth is the usecase the health-check ticker
// (TASK-AIP-03-07) invokes every tick — one batch, one claim transaction,
// per ai-provider-service.md §8. Reuses verifyConnection (TASK-AIP-01-05).
type ReconcileProviderHealth struct {
	claimer DueHealthCheckClaimer
	infra   InfraFleetClient
	outbox  OutboxEnqueuer
	now     func() time.Time
}

func NewReconcileProviderHealth(claimer DueHealthCheckClaimer, infra InfraFleetClient, outbox OutboxEnqueuer, now func() time.Time) *ReconcileProviderHealth {
	if now == nil {
		now = time.Now
	}
	return &ReconcileProviderHealth{claimer: claimer, infra: infra, outbox: outbox, now: now}
}

func (uc *ReconcileProviderHealth) Execute(ctx context.Context, batchSize int32) error {
	batch, err := uc.claimer.ClaimDue(ctx, uc.now(), 15*time.Minute, batchSize)
	if err != nil {
		return fmt.Errorf("claim due accounts: %w", err)
	}
	defer func() { _ = batch.Rollback(ctx) }() // no-op after Commit

	for _, account := range batch.Accounts() {
		start := uc.now()
		result, err := verifyConnection(ctx, uc.infra, account.DevServerID, account.CredentialRef, account.ProviderType)
		latencyMs := int(uc.now().Sub(start).Milliseconds())

		status, detail := classifyHealthResult(account, result, err)
		if err := batch.RecordResult(ctx, account.ID, status, detail, &latencyMs, uc.now()); err != nil {
			return fmt.Errorf("record health result for account %s: %w", account.ID, err)
		}

		// Alert only on a NEW failure classification — avoids paging on
		// every 15-minute re-confirmation of an already-known-bad account.
		if detail != nil && *detail != account.HealthDetailOrEmpty() &&
			(*detail == domain.HealthDetailInvalidKey || *detail == domain.HealthDetailUnreachable || *detail == domain.HealthDetailQuotaExceeded) {
			payload := map[string]any{
				"account_id":     account.ID,
				"provider_type":  string(account.ProviderType),
				"dev_server_id":  account.DevServerID,
				"health_detail":  *detail,
			}
			if err := uc.outbox.Enqueue(ctx, "ai_provider.account.health_degraded", account.TenantID, payload); err != nil {
				return fmt.Errorf("enqueue health-degraded event for account %s: %w", account.ID, err)
			}
		}
	}
	return batch.Commit(ctx)
}

// classifyHealthResult maps a raw connection-test result onto (Status,
// HealthDetail). A quota_exceeded classification carries forward as-is on a
// healthy ping — a passing connection test just means the key still
// authenticates, not that quota has reset; only RecordTokenUsage
// (TASK-AIP-03-05) or a day rollover clears it.
func classifyHealthResult(account domain.ProviderAccount, result ConnectionTestResult, relayErr error) (domain.AccountStatus, *string) {
	if relayErr != nil {
		detail := domain.HealthDetailUnreachable
		return domain.AccountStatusError, &detail
	}
	if !result.Success {
		detail := domain.HealthDetailInvalidKey
		return domain.AccountStatusError, &detail
	}
	if account.Status == domain.AccountStatusError && account.HealthDetailOrEmpty() == domain.HealthDetailQuotaExceeded {
		return account.Status, account.HealthDetail
	}
	detail := domain.HealthDetailHealthy
	return domain.AccountStatusActive, &detail
}
```

Rename the domain helper referenced here to match `TASK-AIP-03-02`'s
actual naming — this file assumes `ProviderAccount.HealthDetailOrEmpty()`;
if `TASK-AIP-03-02` named it `derefHealthDetail()` instead, use that name
here for consistency (align on one name across both tasks — whichever
lands first wins the name).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/ai-provider-service/...
go test ./services/ai-provider-service/internal/usecase/... -run TestReconcileProviderHealth
```

Add to a new `reconcile_provider_health_test.go`:
- `TestReconcileProviderHealth_ClassifiesUnreachable` — fake
  `InfraFleetClient.Relay` returns an error → `AccountStatusError` +
  `HealthDetailUnreachable`, one outbox event enqueued.
- `TestReconcileProviderHealth_ClassifiesInvalidKey` — relay succeeds,
  `result.Success=false` → `HealthDetailInvalidKey`.
- `TestReconcileProviderHealth_NoAlertOnRepeatFailure` — account already
  `HealthDetailUnreachable`, stays `unreachable` this tick → outbox
  `Enqueue` NOT called.
- `TestReconcileProviderHealth_QuotaExceededSurvivesHealthyPing` — account
  is `AccountStatusError`/`HealthDetailQuotaExceeded`; connection test
  succeeds; status stays unchanged.
- `TestReconcileProviderHealth_LatencyRecorded` — successful check records
  a non-nil `latency_ms`.
