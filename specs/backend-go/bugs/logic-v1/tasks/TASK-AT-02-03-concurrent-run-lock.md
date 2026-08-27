# TASK-AT-02-03: Partial unique index + `FindRunning` prevent concurrent runs (BR-AT-08)

**From Solution:** SOL-AT-02
**Priority:** P0
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/postgres/repository.go` (+ migration), `backend-go/services/automation-service/internal/usecase/ports.go`, `backend-go/services/automation-service/internal/usecase/run_now.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Two dispatches of the same automation (e.g. one manual `RunNow` racing the
scheduler ticker) can both transition to `running` today — no lock exists.
BR-AT-08 requires at most one `running` row per automation, enforced with a
Postgres partial unique index (no read-then-write race window), mirroring
the existing `request_id` idempotency pattern already in `run_now.go`.

## Changes to make

Add a migration alongside TASK-AT-01-03's:

```sql
CREATE UNIQUE INDEX idx_automation_runs_one_running
  ON automation.automation_runs (automation_id)
  WHERE status = 'running';
```

Add to `AutomationRunRepository` port in `ports.go`:

```go
// FindRunning returns the currently-running run for automationID, if any.
FindRunning(ctx context.Context, tenantID, automationID string) (domain.AutomationRun, bool, error)
```

Implement in `repository.go` (`SELECT ... WHERE automation_id = $1 AND
status = 'running' LIMIT 1`, backed by the same partial index).

In `run_now.go`, wrap the existing `pending` → `running` `UpdateStatus` call
(the one that would violate this index for a second concurrent dispatch)
with the same catch pattern already used for the `request_id` unique
violation:

```go
if err := uc.runs.UpdateStatus(ctx, running); err != nil {
	if isUniqueViolation(err, "idx_automation_runs_one_running") {
		if existing, found, ferr := uc.runs.FindRunning(ctx, tenantID, automation.ID); ferr == nil && found {
			return existing, nil
		}
	}
	return domain.AutomationRun{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RUN_UPDATE_FAILED", "failed to persist run status", err)
}
```

Check the existing `isUniqueViolation`-equivalent helper used for the
`request_id` race in this file and reuse/extend it rather than writing a
new one.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestRunNow
go test ./services/automation-service/internal/adapter/postgres/... -run TestConcurrentRun
```

Expected: two concurrent `Execute` calls for the same automation (different
`request_id`s) against a fake repo simulating the partial-unique-index
violation on the second `UpdateStatus` call → second call returns the
first call's run via `FindRunning`, executor called exactly once. Integration
test (real Postgres): the partial unique index rejects a second concurrent
`running` insert for the same `automation_id`; a `succeeded`/`failed` row
does NOT conflict.
