# TASK-TG-001-05: `GetSubtree` RPC — server-side recursive subtree read

**From Solution:** BE-SOL-001
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/get_subtree.go` (new), `backend-go/services/task-service/internal/adapter/postgres/subtree.go` (new — shared with TASK-TG-001-03's `RecalculateAncestorProgress`), `backend-go/services/task-service/internal/usecase/ports.go` (extend `TaskRepository`), `backend-go/proto/orca/task/v1/task.proto` (`GetSubtree` RPC), `backend-go/services/task-service/internal/adapter/grpc/server.go` (handler)
**Depends on:** TASK-TG-001-02 (widened `Task` struct — `GetSubtree` returns full `Task` rows, so its `SELECT`/`Scan` list depends on the final column set)
**Status:** `[ ]` TODO

---

## Context

`task-service.md` §3's sketch already names `GetSubtree` (per BE-SOL-001's
own citation, `task-service.md:49,55-57`). No `GetSubtree`/subtree usecase
or repository method exists today — confirmed, `grep -rn "Subtree"
internal/` finds nothing.

**Correctness catch versus BE-SOL-001's own SQL sketch**: its "Design —
`GetSubtree`" section's query —

```sql
WITH RECURSIVE subtree AS (
  SELECT * FROM task.tasks WHERE id = $1
  UNION ALL
  SELECT t.* FROM task.tasks t JOIN subtree s ON t.parent_id = s.id
)
SELECT * FROM subtree;
```

— has no `tenant_id` filter anywhere. The real, working precedent for this
exact shape in this same file, `GetAncestors`
(`internal/adapter/postgres/repository.go:97-143`, read in full), filters
`tenant_id = $1` in BOTH the anchor and the recursive term (`WHERE
a.depth + 1 < $3` is the depth guard, but `tenant_id` is bound once via `$1`
across the whole CTE via the anchor + the join predicate never crossing rows
of a different tenant since `task.tasks.id` is a global UUID PK — still,
match the file's own convention explicitly rather than relying on that
inference). Per the file's own header comment, RLS is "the secondary
backstop" — the *primary* enforcement is this explicit per-query filtering.
This task's SQL follows that convention, not BE-SOL-001's untenanted sketch.

## Changes to make

**1. `internal/usecase/ports.go`** — add to `TaskRepository`:

```go
// GetSubtree returns id and every descendant of id (id.first, breadth order
// not guaranteed) in one WITH RECURSIVE query — replaces the frontend's
// "load task.list for the whole project, filter client-side by parentId"
// pattern (see FE-SOL-001 §2.1).
GetSubtree(ctx context.Context, tenantID, id string) ([]domain.Task, error)
```

**2. `internal/usecase/get_subtree.go`** (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GetSubtree struct {
	tasks TaskRepository
}

func NewGetSubtree(tasks TaskRepository) *GetSubtree {
	return &GetSubtree{tasks: tasks}
}

func (uc *GetSubtree) Execute(ctx context.Context, id string) ([]domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	tasks, err := uc.tasks.GetSubtree(ctx, tenantID, id)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_GET_SUBTREE_FAILED", "failed to load task subtree", err)
	}
	return tasks, nil
}
```

**3. `internal/adapter/postgres/subtree.go`** (new — also holds
TASK-TG-001-03's `RecalculateAncestorProgress`):

```go
func (r *Repository) GetSubtree(ctx context.Context, tenantID, id string) ([]domain.Task, error) {
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT * FROM task.tasks WHERE tenant_id = $1 AND id = $2
			UNION ALL
			SELECT t.* FROM task.tasks t
			JOIN subtree s ON t.parent_id = s.id
			WHERE t.tenant_id = $1
		)
		SELECT id, tenant_id, title, status, COALESCE(parent_id::text, ''), COALESCE(project_id::text, ''), COALESCE(workflow_template_id::text, '') /* + every TASK-TG-001-02 column, once landed */
		FROM subtree
	`, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: query subtree: %w", err)
	}
	defer rows.Close()

	var out []domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Title, &t.Status, &t.ParentID, &t.ProjectID, &t.WorkflowTemplateID /* + new fields */); err != nil {
			return nil, fmt.Errorf("postgres: scan subtree row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

`SELECT *` from BE-SOL-001's sketch is replaced with an explicit column
list (matching `Get`/`GetAncestors`'s existing convention at
`repository.go:82,119`) — `SELECT *` inside a recursive CTE's anchor/
recursive terms is fine, but the outer `SELECT * FROM subtree` would need to
match column order to `Scan` positionally, which is fragile once
TASK-TG-001-02 adds ~18 more columns; naming them explicitly here avoids a
silent column-order break the day someone adds another `ALTER TABLE
task.tasks ADD COLUMN`.

**4. Proto + handler** — add
`rpc GetSubtree(GetSubtreeRequest) returns (GetSubtreeResponse);` (request:
`task_id`; response: `repeated Task tasks`) to `task.proto`, and a handler
in `server.go` following the `GetDependencies` pattern
(`internal/adapter/grpc/server.go:198-207`):

```go
func (s *Server) GetSubtree(ctx context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	tasks, err := s.getSubtree.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.GetSubtreeResponse{Tasks: out}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestGetSubtree -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: 5-node tree fixture with 2 branches — `GetSubtree` on the root
returns the full expected 5-node set; `GetSubtree` on a mid-tree node
returns only that node's own descendants, never a sibling subtree or the
root itself; a task belonging to a different tenant with the same `id`
never leaks into the result (regression test asserting the explicit
`tenant_id` filter, not just RLS).
