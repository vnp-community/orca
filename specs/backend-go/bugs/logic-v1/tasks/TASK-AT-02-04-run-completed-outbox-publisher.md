# TASK-AT-02-04: `orca.automation.run.completed` outbox publisher on terminal transitions

**From Solution:** SOL-AT-02
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/eventbus/publisher.go` (new), `backend-go/services/automation-service/internal/adapter/postgres/repository.go`
**Depends on:** none
**Status:** `[x]` DONE — adapter/eventbus/publisher.go + migration 0007_outbox; UpdateStatus now tx-wrapped, writes the outbox row only on Terminal(); integration-tested (0 rows non-terminal, 1 row terminal).

---

## Context

`notification-service` already subscribes to `orca.automation.run.completed`
but nothing publishes it — `automation-service.md` documents the
`eventbus/` package but it doesn't exist. Per the transactional-outbox
convention, the publish must happen in the same transaction as the terminal
`UpdateStatus` write, never as a bare post-hoc publish call.

## Changes to make

Create `backend-go/services/automation-service/internal/adapter/eventbus/publisher.go`:

```go
package eventbus

type RunCompletedPublisher struct {
	outbox commoneventbus.OutboxWriter
}

type runCompletedPayload struct {
	AutomationID string `json:"automation_id"`
	RunID        string `json:"run_id"`
	Status       string `json:"status"`
}

// PublishRunCompleted writes the run-completed outbox entry inside tx — the
// same transaction as the terminal status UPDATE, per the transactional
// outbox rule (never a direct publish call inside a request handler).
func (p *RunCompletedPublisher) PublishRunCompleted(ctx context.Context, tx pgx.Tx, run domain.AutomationRun) error {
	return p.outbox.Write(ctx, tx, commoneventbus.OutboxEntry{
		Subject:  "orca.automation.run.completed",
		TenantID: run.TenantID,
		Payload:  runCompletedPayload{AutomationID: run.AutomationID, RunID: run.ID, Status: string(run.Status)},
	})
}
```

Change `AutomationRunRepository.UpdateStatus` in `repository.go` from a bare
`pool.Exec` to a `pool.Begin`-wrapped transaction that performs the status
`UPDATE` and (only for terminal transitions, checked via
`run.Status.Terminal()` from `automation_run.go`) the outbox `INSERT`
together:

```go
func (r *Repository) UpdateStatus(ctx context.Context, run domain.AutomationRun) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE automation.automation_runs SET status = $1, ... WHERE id = $2`, run.Status, run.ID); err != nil {
		return err
	}
	if run.Status.Terminal() {
		if err := r.publisher.PublishRunCompleted(ctx, tx, run); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
```

Check the current `UpdateStatus` implementation's exact SET column list
before editing, and wire `RunCompletedPublisher` (or its `OutboxWriter`) into
`Repository`'s constructor.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/automation-service/...
go test ./services/automation-service/internal/adapter/eventbus/...
go test ./services/automation-service/internal/adapter/postgres/... -run TestUpdateStatus
```

Expected: `PublishRunCompleted` writes exactly one outbox row per terminal
transition, zero for `pending`→`running`; a rolled-back transaction leaves
no outbox row (verifies the same-tx requirement).
