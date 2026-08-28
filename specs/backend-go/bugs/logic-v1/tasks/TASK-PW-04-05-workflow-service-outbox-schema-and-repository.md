# TASK-PW-04-05: `workflow-service` outbox table + `UpdateExecution` writes both atomically

**From Solution:** SOL-PW-04
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/migrations/0007_workflow_outbox.up.sql`, `backend-go/services/workflow-service/internal/adapter/postgres/repository.go`, `backend-go/services/workflow-service/internal/domain/outbox.go` (new)
**Depends on:** none
**Status:** `[x]` DONE — migration 0007 (workflow.outbox_events); domain.OutboxEvent added; UpdateExecution signature extended with an event param (single *domain.OutboxEvent, not a slice) + tx-wrapped; FetchUnpublished/MarkPublished added; all 5 existing call sites (cancel/pause/resume/ad-hoc/recover) updated; integration tests pass

---

## Context

`workflow-service.md` §7 already names `workflow.execution.started`/
`completed`, `workflow.step_failed` as "Publishes... (async, NATS) —
consumed by notification-service" and §5 already sketches the outbox
pattern in its schema section — this task builds the schema half, same
shape as TASK-PW-04-02's `task-service` counterpart and
`usage-service`'s already-proven precedent
(`services/usage-service/migrations/0002_outbox.up.sql`,
`internal/adapter/postgres/repository.go`'s `SaveSession`/
`FetchUnpublished`/`MarkPublished`). Independent of task-service's outbox
work — no shared code, can land in parallel.

## Changes to make

New migration (numbered after the existing `0006_template_version`):

```sql
-- backend-go/services/workflow-service/migrations/0007_workflow_outbox.up.sql
-- Transactional outbox table — same shape as usage.outbox_events
-- (usage-service/migrations/0002_outbox.up.sql). workflow.executions
-- writes and this table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.UpdateExecution); common/outbox.Relay
-- polls unpublished rows and publishes them to NATS JetStream.
CREATE TABLE workflow.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_workflow_outbox_events_unpublished
    ON workflow.outbox_events (created_at)
    WHERE published_at IS NULL;
```

Write the matching `.down.sql`.

Add `internal/domain/outbox.go` (identical shape to `usage-service`'s and
TASK-PW-04-02's `task-service` version):

```go
package domain

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON json.RawMessage
}
```

Extend `ExecutionRepository.UpdateExecution`'s signature in
`internal/usecase/ports.go`:

```go
// UpdateExecution persists an execution's mutable fields (status,
// paused_at) and, when event is non-nil, an outbox row — in the SAME
// transaction, matching task-service.TaskRepository.Update's identical
// extension (TASK-PW-04-02/03).
UpdateExecution(ctx context.Context, exec domain.WorkflowExecution, event *domain.OutboxEvent) error
```

Implement in `internal/adapter/postgres/repository.go` — this method
currently uses `r.pool.Exec` directly (no transaction); wrap it in one,
following `usage-service.Repository.SaveSession`'s exact shape:

```go
func (r *Repository) UpdateExecution(ctx context.Context, exec domain.WorkflowExecution, event *domain.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE workflow.executions
		SET status = $1, paused_at = $2, updated_at = now()
		WHERE tenant_id = $3 AND id = $4
	`, string(exec.Status), exec.PausedAt, exec.TenantID, exec.ID)
	if err != nil {
		return fmt.Errorf("postgres: update execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExecutionNotFound
	}

	if event != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO workflow.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, event.ID, exec.TenantID, event.Subject, event.OccurredAt, 1, event.PayloadJSON)
		if err != nil {
			return fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	// identical query/scan shape to usage-service's FetchUnpublished,
	// against workflow.outbox_events.
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	// identical to usage-service's MarkPublished, against workflow.outbox_events.
}
```

Every existing `UpdateExecution` call site (`cancel_execution.go`,
`pause_execution.go`, `resume_execution.go`, `execute_ad_hoc_step.go`,
`recover_executions.go`) must be updated to pass `nil` for `event` except
TASK-PW-04-06's `execute.go` call site — grep for `UpdateExecution(ctx,`
across `internal/usecase/*.go` and update every call, not just the
compiler errors that surface first.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/postgres/... -run 'TestUpdateExecution|TestFetchUnpublished|TestMarkPublished' -v
go test ./services/workflow-service/internal/usecase/... -v
```

Expected: clean build across every existing `UpdateExecution` call site
(`nil` event = no behavior change for Pause/Resume/Cancel/ad-hoc-step/
recovery); new outbox tests pass.
