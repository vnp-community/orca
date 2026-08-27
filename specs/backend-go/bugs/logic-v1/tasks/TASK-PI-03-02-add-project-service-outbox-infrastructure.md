# TASK-PI-03-02: `project-service`'s first outbox table + relay wiring

**From Solution:** SOL-PI-03
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/migrations/0011_outbox_events.up.sql` (new), `backend-go/services/project-service/internal/adapter/eventbus/publisher.go` (new), `backend-go/services/project-service/internal/adapter/postgres/outbox_repository.go` (new), `backend-go/services/project-service/cmd/server/main.go`
**Depends on:** TASK-PI-02-06 (this task's migration number follows `0010_issue_status_sync_enabled`)
**Status:** `[ ]` TODO

---

## Context

`project-service` has no outbox infrastructure yet — its adapter layout
lists only a bare `eventbus:` comment placeholder
(`project-service.md:289`). This task is a direct copy of
`issue-tracking-service`'s existing outbox setup (`outbox.go`,
`internal/adapter/postgres/repository.go`'s `Enqueue`/`FetchUnpublished`/
`MarkPublished`, `common/outbox.Relay`), reused verbatim — no new relay
mechanism, just this service's first table feeding it.

## Changes to make

`migrations/0011_outbox_events.up.sql` (new):

```sql
CREATE TABLE project.outbox_events (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    subject       TEXT NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    version       INT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

CREATE INDEX idx_project_outbox_events_unpublished
    ON project.outbox_events (created_at)
    WHERE published_at IS NULL;

ALTER TABLE project.outbox_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON project.outbox_events
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

`internal/adapter/postgres/outbox_repository.go` (new) — copy
`issue-tracking-service/internal/adapter/postgres/repository.go`'s
`Enqueue`/`FetchUnpublished`/`MarkPublished` methods verbatim, retargeted at
`project.outbox_events`:

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// Enqueue is called from CreateWorktreeWithEvent (TASK-PI-03-03) inside the
// SAME transaction as the worktrees INSERT/DELETE — pass tx, not r.pool,
// when called from within one (see that task for the exact call shape).
func (r *OutboxRepository) EnqueueTx(ctx context.Context, tx pgx.Tx, tenantID string, event domain.OutboxEvent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO project.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, tenantID, event.Subject, event.OccurredAt, event.PayloadJSON)
	if err != nil {
		return fmt.Errorf("postgres: insert outbox event: %w", err)
	}
	return nil
}

func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	// identical shape to issue-tracking-service/internal/adapter/postgres/repository.go's FetchUnpublished
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	// identical shape to issue-tracking-service/internal/adapter/postgres/repository.go's MarkPublished
}
```

`internal/domain/outbox.go` (new) — copy
`issue-tracking-service/internal/domain/outbox.go`'s `OutboxEvent` type
verbatim.

`cmd/server/main.go` — wire `eventbus.Connect`, `outbox.NewRelay(outboxRepo, pub, outbox.DefaultConfig, logger)`,
and `relay.Run(ctx)` in a goroutine, following
`issue-tracking-service/cmd/server/main.go`'s existing wiring exactly
(lines ~119-146 per that file — `EnsureStream` for
`orca.project.worktree.created`/`orca.project.worktree.deleted` before the
relay starts).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
go vet ./services/project-service/...
```
