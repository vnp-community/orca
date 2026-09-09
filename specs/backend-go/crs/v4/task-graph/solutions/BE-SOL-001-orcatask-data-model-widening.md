# BE-SOL-001: Widen `Task` domain/proto, `GetSubtree`, `RecalculateProgress`, auto-block, atomic `AddEdge`

**Resolves:** [CR-TG-001](../../../../../../docs/crs/v4/task-graph/CR-TG-001-orcatask-data-model-widening.md)
**Service:** `task-service` only (proto, domain, usecase, postgres) — no other service touched
**Affected files (proposed):**
- `backend-go/proto/orca/task/v1/task.proto` (widen `Task`; the RPCs `GetSubtree`/`RecalculateProgress`/`AddComment`/`ListComments` are already named in `task-service.md` §3's sketch — this adds them to the real proto, it doesn't invent new names)
- `backend-go/services/task-service/internal/domain/task.go` (new fields, `StatusBacklog`/`StatusTodo`/`StatusBlocked`/`StatusReview`)
- `backend-go/services/task-service/internal/domain/task_comment.go` (new)
- `backend-go/services/task-service/internal/domain/progress.go` (new — pure `CalculateProgress`)
- `backend-go/services/task-service/internal/usecase/get_subtree.go`, `recalculate_progress.go`, `add_comment.go`, `list_comments.go` (new)
- `backend-go/services/task-service/internal/usecase/add_edge.go` (auto-block + atomic cycle-check-then-write)
- `backend-go/services/task-service/internal/usecase/ports.go` (extend `TaskRepository`/`EdgeRepository`, add `CommentRepository`, `TxRunner`)
- `backend-go/services/task-service/internal/adapter/postgres/repository.go`, new `comments.go`, `subtree.go`
- `backend-go/services/task-service/internal/adapter/grpc/server.go`
- `backend-go/services/task-service/migrations/0004_task_fields_and_comments.{up,down}.sql` (new — next free number after `0003_task_workflow_template_id`)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** §"Design — auto-block on
> unmet dependency + atomic AddEdge" bên dưới phác thảo 1 shape
> `ports.Tx`/method hậu tố `Tx` không tồn tại trong codebase. Đã xác nhận có
> sẵn `usecase.TxRunner` port thật (`ports.go:167-184`), và `AIApply` đã
> dùng đúng primitive này (đã implement thật, không phải stub). Xem
> [TASK-TG-001-04](../tasks/TASK-TG-001-04-auto-block-atomic-add-edge.md)
> để lấy design đã sửa đúng theo `TxRunner` thật — đừng implement theo code
> sketch bên dưới nguyên văn.

---

## Design rationale (grounded in TDD + real code)

`task-service.md` §3's RPC sketch already lists exactly the surface this
solution builds:

```
rpc GetSubtree(GetSubtreeRequest) returns (GetSubtreeResponse);
rpc RecalculateProgress(RecalculateProgressRequest) returns (RecalculateProgressResponse);
rpc AddComment(AddCommentRequest) returns (AddCommentResponse);
rpc ListComments(ListCommentsRequest) returns (ListCommentsResponse);
```

— `task-service.md:49,55-57`. §4's domain-model paragraph also already
names the missing structural fields as intended: *"`Task` (id, project
scope, title, status, complexity marker, assignee; status transitions
enforced in methods)"* (`task-service.md:101-103`). The real
`internal/domain/task.go:52-69` struct has none of `description`/
`assignee`/`due_date`/etc — confirmed directly. This solution is not
inventing scope; it is building what the TDD already sketched but the
generated proto never picked up, plus the ~14 additional fields
`docs/features/F37-task-graph-management.md` (dòng 36-91) requires that the
TDD's own §4 paragraph doesn't itemize exhaustively (labels, priority,
estimated/actual hours, AI fields, visibility, execution-tracking IDs) —
those are flagged explicitly below as a genuine extension beyond the TDD's
literal text, though clearly within its intent (§5's own schema section
already reserves room for a wide `Task` row).

Also grounded directly in the CR's own file:line citations
([CR-TG-001](../../../../../../docs/crs/v4/task-graph/CR-TG-001-orcatask-data-model-widening.md) §1) — this solution does not re-derive those, only turns them into concrete migration/code.

## Design — schema widening (additive-only)

```sql
-- 0004_task_fields_and_comments.up.sql
ALTER TABLE task.tasks
  ADD COLUMN description      TEXT,
  ADD COLUMN type              TEXT,
  ADD COLUMN priority          TEXT,
  ADD COLUMN labels            TEXT[] DEFAULT '{}',
  ADD COLUMN assignee_id       TEXT,
  ADD COLUMN reporter_id       TEXT,
  ADD COLUMN owner_id          TEXT,
  ADD COLUMN due_date          TIMESTAMPTZ,
  ADD COLUMN estimated_hours   NUMERIC,
  ADD COLUMN actual_hours      NUMERIC,
  ADD COLUMN prompt_template   TEXT,
  ADD COLUMN ai_context        JSONB,
  ADD COLUMN ai_plan_json      JSONB,
  ADD COLUMN visibility        TEXT NOT NULL DEFAULT 'private',
  ADD COLUMN worktree_id       TEXT,
  ADD COLUMN agent_session_id  TEXT,
  ADD COLUMN workflow_exec_id  TEXT,
  ADD COLUMN done_subtasks     INT NOT NULL DEFAULT 0,
  ADD COLUMN total_subtasks    INT NOT NULL DEFAULT 0;

ALTER TABLE task.tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('backlog','todo','open','in_progress','blocked','review','done','cancelled'));
-- 'open' is kept alongside 'backlog'/'todo' for backward compatibility with
-- existing rows written under the current 4-value enum — not removed, so
-- this migration needs zero data backfill to stay valid.

CREATE TABLE task.task_comments_index (); -- placeholder note: task.task_comments already
-- exists with RLS (migrations/0001_init.up.sql:82-93) — this migration does NOT
-- recreate it, only wires Go code to the table that already exists unused.
```

`owner_id` defaults NULL on this migration (no backfill) — [BE-SOL-003](../../task-graph/solutions/BE-SOL-003-task-access-control-team-scope-and-sharing.md)'s
owner-short-circuit treats a NULL `owner_id` as "no intrinsic owner",
falling through to normal grant resolution; a follow-up data migration to
backfill `owner_id = created_by` for existing rows is a product decision
for whoever runs the migration in production, not part of this solution's
code change.

## Design — `domain.Task` struct + `Status` enum

```go
// internal/domain/task.go
type Status string
const (
    StatusBacklog    Status = "backlog"
    StatusTodo       Status = "todo"
    StatusInProgress Status = "in_progress"
    StatusBlocked    Status = "blocked"
    StatusReview     Status = "review"
    StatusDone       Status = "done"
    StatusCancelled  Status = "cancelled"
)
func validStatus(s Status) bool { /* extend switch to the 7 values above */ }

type Task struct {
    ID, TenantID, Title    string
    Status                 Status
    ParentID               *string
    ProjectID              string
    Description            string
    Type                   string
    Priority               string
    Labels                 []string
    AssigneeID, ReporterID *string
    OwnerID                *string
    DueDate                *time.Time
    EstimatedHours, ActualHours *float64
    PromptTemplate         string
    AIContext              string
    AIPlanJSON             json.RawMessage
    Visibility             string
    WorktreeID, AgentSessionID, WorkflowExecID *string
    DoneSubtasks, TotalSubtasks int
}
```

## Design — `RecalculateProgress`: 1-query bottom-up cascade

```go
// internal/usecase/recalculate_progress.go
func (u *RecalculateProgress) Execute(ctx context.Context, taskID string) error {
    return u.repo.RecalculateAncestorProgress(ctx, taskID)
}
```

```sql
-- postgres/subtree.go — RecalculateAncestorProgress, one WITH RECURSIVE
WITH RECURSIVE ancestors AS (
  SELECT id, parent_id FROM task.tasks WHERE id = $1
  UNION ALL
  SELECT t.id, t.parent_id FROM task.tasks t JOIN ancestors a ON t.id = a.parent_id
)
UPDATE task.tasks SET
  done_subtasks  = (SELECT count(*) FROM task.tasks c WHERE c.parent_id = task.tasks.id AND c.status = 'done'),
  total_subtasks = (SELECT count(*) FROM task.tasks c WHERE c.parent_id = task.tasks.id)
WHERE id IN (SELECT id FROM ancestors);
```

Called from `UpdateTask`/`ExecuteTask` completion path whenever a task with
a non-null `parent_id` changes to `done`/`cancelled` — not called on every
field edit (per §5 CR-TRACE-000-style rule against over-instrumenting
single-row updates, reused here for progress recalculation: only status
transitions that change a subtree's completion count trigger the cascade).

## Design — auto-block on unmet dependency + atomic `AddEdge`

```go
// internal/usecase/add_edge.go
func (u *AddEdge) Execute(ctx context.Context, fromID, toID string, kind domain.EdgeKind) error {
    return u.tx.RunInTx(ctx, func(tx ports.Tx) error {
        wouldCycle, err := u.validator.WouldCreateCycleTx(ctx, tx, fromID, toID)
        if err != nil { return err }
        if wouldCycle { return domain.ErrCyclicDependency }
        if err := u.repo.InsertEdgeTx(ctx, tx, fromID, toID, kind); err != nil { return err }
        if kind == domain.EdgeKindDependsOn {
            fromTask, err := u.repo.GetTx(ctx, tx, fromID)
            if err != nil { return err }
            if fromTask.Status != domain.StatusDone {
                return u.repo.UpdateStatusTx(ctx, tx, toID, domain.StatusBlocked)
            }
        }
        return nil
    })
}
```

`WouldCreateCycleTx` runs the same BFS `CycleDetector` the TDD already
names (§4: *"same algorithm as TS `TaskDAGValidator`, carried forward as-is"*,
`task-service.md:106-110`) but against a transaction-scoped read (`SELECT
... FOR UPDATE` on the affected subtree) instead of a bare `SELECT`, closing
the race the current code's own comment admits
(`internal/usecase/add_edge.go:41-54`).

## Design — `GetSubtree` (server-side, replaces client-side filter)

```sql
-- postgres/subtree.go
WITH RECURSIVE subtree AS (
  SELECT * FROM task.tasks WHERE id = $1
  UNION ALL
  SELECT t.* FROM task.tasks t JOIN subtree s ON t.parent_id = s.id
)
SELECT * FROM subtree;
```

Single round trip, replacing the frontend's current "load `task.list` for
the whole project, filter client-side by `parentId`" pattern (see
[FE-SOL-001](../../../../../frontend/crs/v4/task-graph/solutions/FE-SOL-001-task-crud-board-grant-ui.md) §2.1, which consumes this RPC).

## Test plan

- `Status` enum: all 7 values accepted, anything else rejected (extend
  existing `validStatus` table test).
- `RecalculateProgress`: 4-level tree, mark a leaf `done`, assert every
  ancestor's `done_subtasks`/`total_subtasks` updates in one call.
- `AddEdge` race: two goroutines calling `AddEdge` concurrently on edges
  that together would form a cycle — exactly one succeeds, the other gets
  `ErrCyclicDependency`.
- Auto-block: add `depends_on` edge to a not-done task → target flips to
  `blocked`; source later reaches `done` → target's revaluation (existing
  `UpdateTask` path, unchanged this CR) is not itself in scope here — CR-TG-001
  only owns the initial auto-block write, not the auto-unblock reconciliation
  (flag as follow-up if product wants it — not silently assumed done).
- `GetSubtree` on a 5-node tree, 2 branches, matches full expected set.

## Not in scope (per the CR)

- AI decompose consuming these new fields — [BE-SOL-002](./BE-SOL-002-ai-decompose-context-and-dependency-edges.md).
- Grant/owner short-circuit logic — [BE-SOL-003](./BE-SOL-003-task-access-control-team-scope-and-sharing.md) (this solution only adds the `owner_id` column it needs).
- UI for any new field — [FE-SOL-001](../../../../../frontend/crs/v4/task-graph/solutions/FE-SOL-001-task-crud-board-grant-ui.md).
- Data backfill of `owner_id` for pre-existing rows — a production rollout
  decision, not a code-design one.

## References

- [CR-TG-001](../../../../../../docs/crs/v4/task-graph/CR-TG-001-orcatask-data-model-widening.md)
- [SOL-TG-01](../../../../bugs/logic-v1/solutions/SOL-TG-01-task-graph-structural-management.md) — original design this CR/solution adopts near-verbatim
- `specs/backend-go/tdd/services/task-service.md` §3, §4
