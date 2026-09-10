# TASK-TG-001-03: `RecalculateProgress` — one-query bottom-up ancestor cascade

**From Solution:** BE-SOL-001
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/recalculate_progress.go` (new), `backend-go/services/task-service/internal/adapter/postgres/subtree.go` (new), `backend-go/services/task-service/internal/usecase/ports.go` (extend `TaskRepository`), `backend-go/proto/orca/task/v1/task.proto` (`RecalculateProgress` RPC), `backend-go/services/task-service/internal/adapter/grpc/server.go` (handler), `backend-go/services/task-service/internal/usecase/execute_task.go` and `update_task.go` (call site)
**Depends on:** TASK-TG-001-02 (`Task.DoneSubtasks`/`TotalSubtasks` fields, and the migration's `done_subtasks`/`total_subtasks` columns from TASK-TG-001-01)
**Status:** `[ ]` TODO

---

## Context

BE-SOL-001 §"Design — `RecalculateProgress`" gives the exact shape:
a usecase whose `Execute` calls a single new repository method,
`RecalculateAncestorProgress`, backed by one `WITH RECURSIVE` UPDATE. This
task file expands that sketch into the concrete Go/SQL, `ports.go` interface
addition, proto RPC, and (the part BE-SOL-001's design snippet doesn't spell
out) where it's actually called from.

No such method exists on `TaskRepository` today — confirmed,
`internal/usecase/ports.go:17-47`'s `TaskRepository` interface has no
progress-related method, and `grep -rn "RecalculateProgress"
internal/` finds nothing. This is new surface, not a widen of existing code.

`task-service.md` §3's RPC sketch already names `RecalculateProgress` as an
intended RPC (per BE-SOL-001's own citation, `task-service.md:49,55-57`) — use
that name as-is.

## Changes to make

**1. `internal/usecase/ports.go`** — add to `TaskRepository`:

```go
// RecalculateAncestorProgress recalculates done_subtasks/total_subtasks for
// every ancestor of taskID (including taskID's own parent chain, per
// BE-SOL-001) in one WITH RECURSIVE UPDATE — not a per-ancestor round trip.
RecalculateAncestorProgress(ctx context.Context, tenantID, taskID string) error
```

(Widened with `tenantID` versus BE-SOL-001's bare `taskID` sketch — every
other `TaskRepository` method in this file takes `tenantID` explicitly as
the primary tenant-scoping enforcement, per `repository.go`'s own header
comment that RLS is a backstop, not the primary filter. Follow that
convention here too.)

**2. `internal/usecase/recalculate_progress.go`** (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type RecalculateProgress struct {
	tasks TaskRepository
}

func NewRecalculateProgress(tasks TaskRepository) *RecalculateProgress {
	return &RecalculateProgress{tasks: tasks}
}

func (uc *RecalculateProgress) Execute(ctx context.Context, taskID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.tasks.RecalculateAncestorProgress(ctx, tenantID, taskID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_RECALCULATE_PROGRESS_FAILED", "failed to recalculate ancestor progress", err)
	}
	return nil
}
```

**3. `internal/adapter/postgres/subtree.go`** (new file — `GetSubtree`,
TASK-TG-001-05, also lives here):

```go
func (r *Repository) RecalculateAncestorProgress(ctx context.Context, tenantID, taskID string) error {
	_, err := r.db.Exec(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM task.tasks WHERE tenant_id = $1 AND id = $2
			UNION ALL
			SELECT t.id, t.parent_id FROM task.tasks t
			JOIN ancestors a ON t.id = a.parent_id
			WHERE t.tenant_id = $1
		)
		UPDATE task.tasks SET
			done_subtasks  = (SELECT count(*) FROM task.tasks c WHERE c.tenant_id = $1 AND c.parent_id = task.tasks.id AND c.status = 'done'),
			total_subtasks = (SELECT count(*) FROM task.tasks c WHERE c.tenant_id = $1 AND c.parent_id = task.tasks.id)
		WHERE tenant_id = $1 AND id IN (SELECT id FROM ancestors)
	`, tenantID, taskID)
	if err != nil {
		return fmt.Errorf("postgres: recalculate ancestor progress: %w", err)
	}
	return nil
}
```

(Tenant-scoped throughout, unlike BE-SOL-001's bare sketch — same correction
as GetAncestors's real precedent, `repository.go:106-122`, which filters
`tenant_id` in both the anchor and recursive terms.)

**4. Proto + handler** — add
`rpc RecalculateProgress(RecalculateProgressRequest) returns (RecalculateProgressResponse);`
to `task.proto` (request: `task_id`; response: empty or echoes the
recalculated root — either is fine, BE-SOL-001 doesn't specify, default to
empty `google.protobuf.Empty`-style response unless the frontend solution
(FE-SOL-001) needs the updated counts back synchronously — check that
solution before finalizing the response shape). Add the `Server` handler
following the existing `GetDependencies`/`AIDecompose` handler pattern
(`internal/adapter/grpc/server.go:198-210`).

**5. Call site** — per BE-SOL-001: *"Called from `UpdateTask`/`ExecuteTask`
completion path whenever a task with a non-null `parent_id` changes to
`done`/`cancelled`"* — NOT on every field edit. `execute_task.go` has no
completion callback today (see TASK-TG-005-01/-02 for that gap); until that
lands, wire this call into `UpdateTask`'s existing status-transition path
(`domain.Task.SetStatus`, `task.go:125-137`) — when `SetStatus` succeeds
into `StatusDone`/`StatusCancelled` AND the task has a non-empty
`ParentID`, call `RecalculateProgress.Execute` after the `Update` write
commits. Re-wire into `ExecuteTask`'s own completion path once
TASK-TG-005-01/-02's callback exists — flag this as a near-term follow-up
in the PR description if `ExecuteTask` still has no completion callback at
implementation time, rather than silently leaving the execute-path cascade
unimplemented.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestRecalculateProgress -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: 4-level tree fixture, mark a leaf `done`, one `Execute` call
updates every ancestor's `done_subtasks`/`total_subtasks` correctly in a
single repository call (assert via a mock/fake counting exactly one SQL
round trip, or via a real DB test asserting the resulting counts); a task
with no parent is a no-op (recursive CTE anchors on itself, updates zero
ancestor rows beyond itself if applicable — assert no error, no crash).
