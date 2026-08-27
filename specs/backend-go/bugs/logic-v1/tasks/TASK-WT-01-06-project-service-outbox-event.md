# TASK-WT-01-06: Publish `orca.project.worktree.created` via a new transactional outbox in `project-service`

**From Solution:** SOL-WT-01
**Priority:** P1 — independent of the git-gateway-service tasks in this set; can land in parallel
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/record_worktree_created.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

Per [SOL-WT-01](../solutions/SOL-WT-01-tao-worktree.md) and `specs/backend-go/tdd/architecture/05-data-architecture.md`'s transactional-outbox prescription, `worktree.created` should be an outbox event written in the same transaction as `RecordWorktreeCreated`'s `INSERT`. **Correction to the SOL's own citation**: `project-service.md` §6 describes an `adapter/eventbus/` package in the target design, but no such package, outbox table, or `common/outbox.Relay` wiring exists in `project-service` today (confirmed: `grep -rl eventbus services/project-service/` returns nothing). This task builds that infrastructure from scratch, following the real, working pattern `usage-service` already ships (`services/usage-service/internal/domain/outbox.go`, `internal/adapter/postgres/repository.go`'s `SaveSession`, `cmd/server/main.go`'s `outbox.NewRelay` wiring) — not a new pattern, a second application of an existing one.

## Changes to make

**Migration** — `backend-go/services/project-service/migrations/0010_outbox.up.sql` (mirrors `usage-service/migrations/0002_outbox.up.sql`, `project` schema instead of `usage`):

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
```

`backend-go/services/project-service/migrations/0010_outbox.down.sql`:

```sql
DROP TABLE project.outbox_events;
```

**Domain** — `backend-go/services/project-service/internal/domain/outbox.go` (new, mirrors `usage-service/internal/domain/outbox.go`):

```go
package domain

import "time"

// OutboxEvent is a pre-built event a usecase asks its repository to
// durably enqueue in the SAME transaction as its domain write — the
// transactional-outbox pattern (05-data-architecture.md). Lives in
// domain/, not common/eventbus.Event, so usecase/ can build one without
// importing anything NATS-specific.
type OutboxEvent struct {
	ID          string
	Subject     string
	OccurredAt  time.Time
	PayloadJSON []byte
}
```

**Repository** — `backend-go/services/project-service/internal/adapter/postgres/worktree_repository.go`'s `RecordWorktreeCreated` gains an `event domain.OutboxEvent` param and writes both rows in one transaction:

```go
func (r *WorktreeRepository) RecordWorktreeCreated(ctx context.Context, wt domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO project.worktrees (
			id, project_id, repo_id, path, branch, active,
			parent_worktree_id, origin, capture_source, capture_confidence, task_id,
			orchestration_run_id, coordinator_handle, created_by_terminal_handle
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING `+worktreeColumns,
		wt.ID, wt.ProjectID, wt.RepoID, wt.Path, wt.Branch, wt.Active,
		wt.ParentWorktreeID, wt.Origin, wt.CaptureSource, wt.CaptureConfidence, wt.TaskID,
		wt.OrchestrationRunID, wt.CoordinatorHandle, wt.CreatedByTerminalHandle,
	)
	out, err := scanWorktree(row)
	if err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert worktree: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO project.outbox_events (id, tenant_id, subject, occurred_at, version, payload)
		VALUES ($1, $2, $3, $4, 1, $5)
	`, event.ID, wt.ProjectID, event.Subject, event.OccurredAt, event.PayloadJSON); err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Worktree{}, fmt.Errorf("postgres: commit: %w", err)
	}
	return out, nil
}
```

Add the outbox `Store` methods to the same repository (mirrors `usage-service/internal/adapter/postgres/repository.go`'s `FetchUnpublished`/`MarkPublished`):

```go
func (r *WorktreeRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, subject, tenant_id, occurred_at, version, payload
		FROM project.outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: query outbox events: %w", err)
	}
	defer rows.Close()

	var out []outbox.Record
	for rows.Next() {
		var rec outbox.Record
		var tenantID string
		var occurredAt time.Time
		var version int
		var payload []byte
		if err := rows.Scan(&rec.ID, &rec.Subject, &tenantID, &occurredAt, &version, &payload); err != nil {
			return nil, fmt.Errorf("postgres: scan outbox event: %w", err)
		}
		rec.Event = eventbus.Event{ID: rec.ID, TenantID: tenantID, OccurredAt: occurredAt, Version: int32(version), Payload: payload}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *WorktreeRepository) MarkPublished(ctx context.Context, ids []string) error {
	_, err := r.pool.Exec(ctx, `UPDATE project.outbox_events SET published_at = now() WHERE id = ANY($1)`, ids)
	return err
}
```

Add `"github.com/stablyai/orca-go/common/eventbus"` and `"github.com/stablyai/orca-go/common/outbox"` to the file's imports.

**Usecase** — `backend-go/services/project-service/internal/usecase/ports.go`'s `WorktreeRepository.RecordWorktreeCreated` signature gains the `event` param, matching the repository change:

```go
	RecordWorktreeCreated(ctx context.Context, worktree domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error)
```

`backend-go/services/project-service/internal/usecase/record_worktree_created.go`'s `Execute` builds the event:

```go
func (uc *RecordWorktreeCreated) Execute(ctx context.Context, in RecordWorktreeCreatedInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	wt, err := domain.NewWorktree(uuid.NewString(), in.ProjectID, in.RepoID, in.Path, in.Branch, in.Lineage)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_INVALID", err.Error(), err)
	}

	payload, err := json.Marshal(worktreeCreatedPayload{WorktreeID: wt.ID, ProjectID: wt.ProjectID, RepoID: wt.RepoID, Path: wt.Path, Branch: wt.Branch})
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_EVENT_MARSHAL_FAILED", "failed to build worktree.created event", err)
	}
	event := domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.project.worktree.created", OccurredAt: time.Now().UTC(), PayloadJSON: payload}

	created, err := uc.repo.RecordWorktreeCreated(ctx, wt, event)
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_RECORD_WORKTREE_FAILED", "failed to persist worktree", err)
	}
	return created, nil
}

type worktreeCreatedPayload struct {
	WorktreeID string `json:"worktree_id"`
	ProjectID  string `json:"project_id"`
	RepoID     string `json:"repo_id"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
}
```

Add `"encoding/json"` and `"time"` to the file's imports.

**Wiring** — `backend-go/services/project-service/cmd/server/main.go`: add the relay, mirroring `usage-service/cmd/server/main.go`'s `outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)` call plus a `go relay.Run(ctx)` goroutine started alongside the gRPC server's existing startup sequence. `pub` is a `*common/eventbus.Publisher` dialed against the same NATS connection `project-service` already needs for its other outbound calls (check `main.go`'s existing dependency wiring for the connection this service already holds, or dial one the same way `usage-service/cmd/server/main.go` does if none exists yet).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
psql "$PROJECT_SERVICE_DATABASE_URL" -f services/project-service/migrations/0010_outbox.up.sql
```

Expected: clean build; migration applies without error. A rollback check (`0010_outbox.down.sql` then re-apply `.up.sql`) is worth running once locally per `05-data-architecture.md`'s migration CI requirement. Behavior test (outbox row written in the same transaction as the worktree insert) lands in [TASK-WT-01-07](./TASK-WT-01-07-tests.md).
