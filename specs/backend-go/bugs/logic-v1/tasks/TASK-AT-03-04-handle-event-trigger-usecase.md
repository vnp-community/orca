# TASK-AT-03-04: `HandleEventTrigger` usecase — match + dispatch on incoming events

**From Solution:** SOL-AT-03
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/usecase/handle_event_trigger.go` (new), `backend-go/services/automation-service/internal/domain/automation_run.go`
**Depends on:** TASK-AT-03-03
**Status:** `[ ]` TODO

---

## Context

This is the core dispatch logic: given an event name/payload, list matching
enabled event-triggered automations, apply BR-AT-09's filter, and dispatch
via the existing `RunNow` usecase with a deterministic idempotency key so
JetStream's at-least-once redelivery is harmless.

## Changes to make

In `automation_run.go`, add a fourth `RunTrigger` value alongside the
existing `manual`/`scheduled`/`external`:

```go
const RunTriggerEvent RunTrigger = "event"
```

Create `backend-go/services/automation-service/internal/usecase/handle_event_trigger.go`:

```go
package usecase

type HandleEventTriggerInput struct {
	EventID   string
	TenantID  string
	EventName domain.EventName
	Payload   string // raw JSON
}

type HandleEventTrigger struct {
	automations AutomationRepository // ListByTrigger
	runNow      *RunNow
	logger      *slog.Logger
}

func (uc *HandleEventTrigger) Execute(ctx context.Context, in HandleEventTriggerInput) error {
	tenantCtx := tenant.WithTenantID(ctx, in.TenantID)
	matches, err := uc.automations.ListByTrigger(tenantCtx, in.TenantID, in.EventName)
	if err != nil {
		return err
	}
	var payload map[string]any
	_ = json.Unmarshal([]byte(in.Payload), &payload)
	for _, automation := range matches {
		if !automation.Enabled {
			continue // BR-AT-03
		}
		if automation.TriggerFilter != nil && !automation.TriggerFilter.Matches(payload) {
			continue // BR-AT-09
		}
		// Deterministic request_id from (event ID, automation ID) — idempotent
		// under JetStream's at-least-once redelivery, same mechanism as
		// scheduler.scheduledRequestID.
		requestID := fmt.Sprintf("event:%s:%s", in.EventID, automation.ID)
		if _, err := uc.runNow.Execute(tenantCtx, RunNowInput{
			AutomationID: automation.ID, RequestID: requestID, Trigger: domain.RunTriggerEvent,
		}); err != nil {
			uc.logger.Error("event-triggered dispatch failed", "error", err, "automation_id", automation.ID, "event_id", in.EventID)
			// Log and continue — one automation's dispatch failure must not
			// block dispatching the rest of this tenant's matching automations.
		}
	}
	return nil
}
```

Check `RunNowInput`'s exact field names (`Trigger` field may not exist yet —
add it if `RunNow` doesn't already accept a trigger source) before wiring.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestHandleEventTrigger
```

Expected: fake `AutomationRepository` + fake `RunNow` dependencies — an
event matching 3 automations (1 disabled, 1 filter-mismatched, 1 matching)
dispatches exactly once; redelivery of the same `EventID` is a no-op via
`RunNow`'s existing `request_id` idempotency (assert executor called once
total across two `Execute` calls with the same `EventID`).
