# TASK-AT-02-02: 30-record retention — `PruneOldRuns` (BR-AT-07)

**From Solution:** SOL-AT-02
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/postgres/repository.go`, `backend-go/services/automation-service/internal/usecase/ports.go`, `backend-go/services/automation-service/internal/usecase/run_now.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`automation_runs` has an index shaped for "find the Nth-most-recent row"
(`idx_automation_runs_automation (automation_id, created_at DESC)`) but no
pruning logic uses it. BR-AT-07 caps retained runs at 30 per automation,
best-effort (never fails the run itself).

## Changes to make

Add to the `AutomationRunRepository` port in `internal/usecase/ports.go`:

```go
// PruneOldRuns deletes every automation_runs row for automationID beyond
// the `keep` most recent (by created_at DESC) — BR-AT-07.
PruneOldRuns(ctx context.Context, tenantID, automationID string, keep int) error
```

Implement in `repository.go`:

```go
func (r *Repository) PruneOldRuns(ctx context.Context, tenantID, automationID string, keep int) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM automation.automation_runs
		WHERE tenant_id = $1 AND automation_id = $2
		  AND id NOT IN (
		    SELECT id FROM automation.automation_runs
		    WHERE tenant_id = $1 AND automation_id = $2
		    ORDER BY created_at DESC
		    LIMIT $3
		  )`,
		tenantID, automationID, keep,
	)
	return err
}
```

In `run_now.go`, add `const keepRuns = 30` and call `PruneOldRuns` after the
run reaches a terminal status (`succeeded`/`failed`/timeout), best-effort —
log on error, never fail the run:

```go
if err := uc.runs.PruneOldRuns(ctx, tenantID, automation.ID, keepRuns); err != nil {
	uc.logger.Warn("failed to prune old automation runs", "error", err, "automation_id", automation.ID)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/usecase/... -run TestRunNow
go test ./services/automation-service/internal/adapter/postgres/... -run TestPruneOldRuns
```

Expected: 31 prior runs for one automation, `keep=30` → after a new
dispatch, `PruneOldRuns` reduces the stored count to exactly 30, newest-first
(unit test with fake repo, plus an integration test against real Postgres if
the existing test suite runs one).
