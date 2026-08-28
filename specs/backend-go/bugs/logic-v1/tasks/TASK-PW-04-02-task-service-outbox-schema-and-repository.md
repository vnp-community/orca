# TASK-PW-04-02: `task-service` outbox table + `task_number` sequence + `Update` writes both atomically

**From Solution:** SOL-PW-04
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/migrations/0003_task_outbox_and_number.up.sql`, `backend-go/services/task-service/internal/adapter/postgres/repository.go`, `backend-go/services/task-service/internal/domain/outbox.go` (new)
**Depends on:** TASK-PW-04-01
**Status:** `[x]` DONE — migration 0003 (task_number/worktree_id/pr_url columns, task_number_seq, task.outbox_events); domain.OutboxEvent added; Repository.Update/Create/FindByNumber/FetchUnpublished/MarkPublished implemented; integration tests pass (TestRepository_Create_AssignsNonZeroTaskNumber, TestRepository_Update_WithEvents_WritesOutboxRowsInSameTransaction, etc.)

---

## Context

`task-service.md` §6's package layout already names `adapter/eventbus/ #
task.created/task.completed/... via outbox` — this is the schema half of
actually building it. Follow **`usage-service`'s already-real,
already-proven pattern** exactly
(`backend-go/services/usage-service/migrations/0002_outbox.up.sql`,
`internal/adapter/postgres/repository.go`'s `SaveSession`/
`FetchUnpublished`/`MarkPublished`) — do not invent a new shape; that
service already solved "domain write + outbox row in one transaction"
for this exact codebase's conventions. `common/outbox.Store`'s port
(`FetchUnpublished(ctx, limit) ([]Record, error)`, `MarkPublished(ctx,
ids) error`) is what this task's new repository methods implement.

## Changes to make

New migration (numbered after the existing `0002_task_project_execution_tracking`):

```sql
-- backend-go/services/task-service/migrations/0003_task_outbox_and_number.up.sql
ALTER TABLE task.tasks ADD COLUMN task_number BIGINT;
ALTER TABLE task.tasks ADD COLUMN worktree_id UUID;
ALTER TABLE task.tasks ADD COLUMN pr_url TEXT;

-- One sequence per project would require dynamic sequence creation;
-- instead use a single global sequence and enforce project-scoped
-- uniqueness via a composite unique index — task_number's *value* need
-- not be contiguous per project, only unique-per-project and monotonic,
-- which nextval() on one shared sequence still guarantees.
CREATE SEQUENCE task.task_number_seq;
CREATE UNIQUE INDEX idx_tasks_project_task_number
    ON task.tasks (project_id, task_number)
    WHERE task_number IS NOT NULL;

-- Transactional outbox table — same shape as usage.outbox_events
-- (usage-service/migrations/0002_outbox.up.sql). task.tasks writes and
-- this table's INSERT happen in the same Postgres transaction
-- (internal/adapter/postgres.Repository.Update); common/outbox.Relay
-- polls unpublished rows and publishes them to NATS JetStream.
CREATE TABLE task.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_task_outbox_events_unpublished
    ON task.outbox_events (created_at)
    WHERE published_at IS NULL;
```

Write the matching `.down.sql` (drop table, drop index, drop sequence,
drop the three columns).

Add `internal/domain/outbox.go` (mirrors `usage-service`'s file exactly):

```go
package domain

import (
	"encoding/json"
	"time"
)

// OutboxEvent is a pre-built event UpdateTask asks its repository to
// persist in the same transaction as the task row it describes — see
// usage-service's identical OutboxEvent for the precedent this mirrors.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON json.RawMessage
}
```

Extend `TaskRepository.Update`'s signature in `internal/usecase/ports.go`
to take an optional event (nil = no event to publish, e.g. a title-only
edit):

```go
// Update persists a partial field update (title/status) and, when event
// is non-nil, an outbox row — in the SAME transaction, so a status
// transition and its published fact are never observed inconsistently.
Update(ctx context.Context, tenantID string, task domain.Task, event *domain.OutboxEvent) error
```

Implement in `internal/adapter/postgres/repository.go`, following
`usage-service.Repository.SaveSession`'s exact transaction shape (begin →
exec task update → exec outbox insert if event != nil → commit; `defer
tx.Rollback` for the error path):

```go
func (r *Repository) Update(ctx context.Context, tenantID string, task domain.Task, event *domain.OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE task.tasks SET title = $1, status = $2, worktree_id = $3, pr_url = $4
		WHERE tenant_id = $5 AND id = $6
	`, task.Title, task.Status, nullableUUID(task.WorktreeID), nullableString(task.PRURL), tenantID, task.ID)
	if err != nil {
		return fmt.Errorf("postgres: update task: %w", err)
	}

	if event != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO task.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, event.ID, tenantID, event.Subject, event.OccurredAt, 1, event.PayloadJSON)
		if err != nil {
			return fmt.Errorf("postgres: insert outbox event: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	// identical query/scan shape to usage-service's FetchUnpublished,
	// against task.outbox_events.
}

func (r *Repository) MarkPublished(ctx context.Context, ids []string) error {
	// identical to usage-service's MarkPublished, against task.outbox_events.
}
```

Add `WorktreeID`/`PRURL` fields to `domain.Task` (`internal/domain/task.go`),
and a `nullableString` helper if this package doesn't already have one
(check `nullableUUID`'s neighborhood first).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/postgres/... -run 'TestUpdate|TestFetchUnpublished|TestMarkPublished' -v
```

Expected: clean build; a status-changing `Update` call with a non-nil
`event` writes both rows in one transaction (assert via a real Postgres
integration test, same harness this package's existing tests use); a
`nil`-event `Update` writes no outbox row.
