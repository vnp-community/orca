# SOL-TG-01: Widen `Task`'s field set, add `GetSubtree`/progress/auto-block, and land `task_comments`

**Resolves:** [BUG-TG-01](../BUG-TG-01-task-graph-structural-management-partial.md)
**Service:** `task-service` (proto, domain, usecase, postgres) — no other service touched
**Affected files (proposed):**
- `backend-go/proto/orca/task/v1/task.proto` (widen `Task`, add `GetSubtree`/`RecalculateProgress`/`AddComment`/`ListComments` RPCs)
- `backend-go/services/task-service/internal/domain/task.go` (new fields, `StatusBlocked`)
- `backend-go/services/task-service/internal/domain/task_comment.go` (new)
- `backend-go/services/task-service/internal/domain/progress.go` (new — pure `CalculateProgress`)
- `backend-go/services/task-service/internal/usecase/create_task.go`, `update_task.go` (new fields)
- `backend-go/services/task-service/internal/usecase/get_subtree.go`, `recalculate_progress.go`, `add_comment.go`, `list_comments.go` (new)
- `backend-go/services/task-service/internal/usecase/add_edge.go` (auto-block + atomic cycle-check-then-write)
- `backend-go/services/task-service/internal/usecase/ports.go` (extend `TaskRepository`/`EdgeRepository`, add `CommentRepository`)
- `backend-go/services/task-service/internal/adapter/postgres/repository.go`, new `comments.go`, `subtree.go`
- `backend-go/services/task-service/internal/adapter/grpc/server.go`
- `backend-go/services/task-service/migrations/0003_task_fields_and_comments.{up,down}.sql` (new)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go`, `internal/adapter/wscompat/channels.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`task-service.md` §3's RPC sketch already lists `GetSubtree`, `RecalculateProgress`,
`AddComment`/`ListComments` as part of this service's intended surface — this
solution is not inventing new RPCs, it is building the ones the TDD already
named but the generated proto never picked up (README's own "Known
deviations" section, `backend-go/services/task-service/README.md:148-157`,
already documents this proto/design-doc gap explicitly). §4's domain-model
paragraph also already names the missing structural fields as intended:
"`Task` (id, project scope, title, status, complexity marker, assignee;
status transitions enforced in methods)" (`task-service.md:101-103`) — the
scaffold's `Task` struct has none of `description`/`complexity`/`assignee`
today (`internal/domain/task.go:52-69`). This solution closes that gap for
the fields §4.1 (grant resolution → TG-01/TG-04) and §3.1 (execution
dispatch → TG-04) actually depend on, deferring the rest (`labels`,
`due_date`, `visibility`, `reporter_id`) as low-risk additive columns that
don't block another bug's design.

**Genuine extension beyond the TDD, flagged explicitly**: §5's sketch schema
has a single `complexity TEXT` column computed once at write time.
`README.md:144-147` already argues the *current* scaffold's dynamic
edge-based complexity check ("more directly matches the 'branching logic
must be real and tested' build goal") is better than a stored column that
the RPC surface has no way to keep in sync — this solution keeps that
deviation rather than reverting to §5's sketch, and does not add a
`complexity` column.

## Design — schema

`migrations/0003_task_fields_and_comments.up.sql` widens `task.tasks` and
activates `task.task_comments` (already exists, unused per BUG-TG-01's
finding, `migrations/0001_init.up.sql:82-93`):

```sql
ALTER TABLE task.tasks
  ADD COLUMN description        TEXT,
  ADD COLUMN task_type          TEXT NOT NULL DEFAULT 'task' CHECK (task_type IN ('task','bug','feature','epic')),
  ADD COLUMN priority            TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low','medium','high','urgent')),
  ADD COLUMN assignee_id         UUID,          -- logical FK -> tenant-service
  ADD COLUMN owner_id            UUID,          -- logical FK -> tenant-service; see SOL-TG-03 for its use in grant resolution
  ADD COLUMN due_date            TIMESTAMPTZ,
  ADD COLUMN estimated_hours     NUMERIC(6,2),
  ADD COLUMN actual_hours        NUMERIC(6,2),  -- see SOL-TG-04 (auto-advance to review)
  ADD COLUMN prompt_template     TEXT,          -- see SOL-TG-02 ("Generate Agent Prompt")
  ADD COLUMN ai_context          TEXT,
  ADD COLUMN ai_plan_json        JSONB,         -- see SOL-TG-02 (raw AI response)
  ADD COLUMN visibility          TEXT NOT NULL DEFAULT 'team' CHECK (visibility IN ('private','team','public')),
  ADD COLUMN worktree_id         UUID,          -- logical FK -> project-service worktrees; see SOL-TG-04
  ADD COLUMN agent_session_id    TEXT,          -- see SOL-TG-04
  ADD COLUMN progress_percent    SMALLINT NOT NULL DEFAULT 0 CHECK (progress_percent BETWEEN 0 AND 100);

-- StatusBlocked joins the status CHECK — see domain/task.go's new
-- StatusBlocked const and the auto-block design below.
ALTER TABLE task.tasks DROP CONSTRAINT tasks_status_check;
ALTER TABLE task.tasks ADD CONSTRAINT tasks_status_check
  CHECK (status IN ('open','blocked','in_progress','review','done','cancelled'));
-- 'review' added here too — SOL-TG-04's auto-advance-on-completion target;
-- flagged in that solution, added here so one migration covers the whole
-- status-enum widening instead of two.

CREATE INDEX idx_tasks_assignee ON task.tasks (assignee_id) WHERE assignee_id IS NOT NULL;
```

`task.task_comments` needs no DDL change — its columns
(`id, tenant_id, task_id, author_id, content, created_at`,
`migrations/0001_init.up.sql:82-93`) already match what `AddComment`/
`ListComments` need; only the repository/usecase/RPC layers are missing.

## Design — domain

```go
// internal/domain/task.go — additive fields, existing fields unchanged.
const (
    StatusOpen       = "open"
    StatusBlocked    = "blocked"     // new — see auto-block design below
    StatusInProgress = "in_progress"
    StatusReview     = "review"      // new — SOL-TG-04's completion target
    StatusDone       = "done"
    StatusCancelled  = "cancelled"
)

type Task struct {
    ID, TenantID, Title, Status, ParentID, ProjectID string // unchanged

    Description      string
    Type              string // task|bug|feature|epic
    Priority          string
    AssigneeID        string
    OwnerID           string // see SOL-TG-03 — intrinsic-owner short-circuit
    DueDate           *time.Time
    EstimatedHours    *float64
    ActualHours       *float64 // see SOL-TG-04
    PromptTemplate    string   // see SOL-TG-02
    AIContext         string
    AIPlanJSON        string   // see SOL-TG-02
    Visibility        string
    WorktreeID        string   // see SOL-TG-04
    AgentSessionID    string   // see SOL-TG-04
    ProgressPercent   int
}
```

`NewTask`'s existing invariants (tenant/title required, no self-parent,
valid status) are unchanged; the new fields are all optional at
construction (zero values are valid, matching `CreateTaskRequest`'s
proto-optional shape below) — no new required-field errors are introduced,
keeping `CreateTask` backward compatible with every existing caller/test
that only sets `Title`/`ParentID`/`ProjectID`.

`domain/task_comment.go` (new, mirrors `task_edge.go`'s minimal-invariant
shape):

```go
type TaskComment struct {
    ID, TaskID, AuthorID, Content string
    CreatedAt                     time.Time
}

var ErrEmptyCommentBody = errors.New("domain: comment content is required")

func NewTaskComment(id, taskID, authorID, content string) (TaskComment, error) {
    if content == "" {
        return TaskComment{}, ErrEmptyCommentBody
    }
    return TaskComment{ID: id, TaskID: taskID, AuthorID: authorID, Content: content}, nil
}
```

### `CalculateProgress` — pure domain function, `domain/progress.go`

Spec (`docs/logic/task-graph/BL-TG-01...md`'s §"calculateProgress()") wants
subtask-completion percentage cascading to parents. Kept a pure function of
an already-fetched subtree, per the same "no SQL, no gRPC" discipline
`task-service.md §6` requires of `grant_resolution.go`:

```go
// CalculateProgress computes, for one task, the percentage of its direct
// children marked Done — 100 for a leaf task whose OWN status is Done, 0
// for a leaf that isn't, and the average of children's own (already
// recursively computed) percentages for a task with children. The caller
// (RecalculateProgress usecase) walks bottom-up over a subtree fetched via
// GetSubtree, calling this once per task in post-order.
func CalculateProgress(task Task, childPercents []int) int {
    if len(childPercents) == 0 {
        if task.Status == StatusDone {
            return 100
        }
        return 0
    }
    sum := 0
    for _, p := range childPercents {
        sum += p
    }
    return sum / len(childPercents)
}
```

## Design — usecase

### `GetSubtree` — descendant walk + per-task access filter

Mirrors `GetAncestors`' existing shape (`repository.go:101-143`) but walks
`parent_id` in the opposite direction. Per the spec's
`hasTaskAccess(userId, task, 'view')` filter and `task-service.md §8`'s
grant-resolution hot-path NFR, this must **not** call `ResolvePermission`
once per node (an N-call fan-out against a p99-sensitive path) — instead it
batch-fetches every grant on every task in the subtree in one query
(`ListGrantsForAncestors`'s existing shape already takes `taskIDs []string`,
`grants.go:45-74`, reused as-is) and resolves each node's access from an
in-memory ancestor-chain map built from the already-fetched subtree, since
every node's ancestor chain is a prefix of the path already walked to reach
it:

```go
// internal/usecase/get_subtree.go
type GetSubtree struct {
    tasks  TaskRepository
    grants GrantRepository
    teams  TeamScopeResolver
    maxDepth int
}

func (uc *GetSubtree) Execute(ctx context.Context, in GetSubtreeInput) (GetSubtreeResult, error) {
    tenantID, _ := tenant.RequireTenantID(ctx)
    userID, _ := tenant.UserID(ctx)

    nodes, edges, err := uc.tasks.GetSubtree(ctx, tenantID, in.RootID, uc.maxDepth) // new repo method, WITH RECURSIVE down parent_id
    if err != nil { return GetSubtreeResult{}, ... }

    taskIDs := idsOf(nodes)
    grantsByTask, err := uc.grants.ListGrantsForAncestors(ctx, tenantID, taskIDs) // reused verbatim
    teamIDs, err := uc.teams.ResolveTeams(ctx, tenantID, userID)
    caller := domain.CallerIdentity{UserID: userID, TeamIDs: teamIDs, CompanyID: tenantID}

    // chainOf[nodeID] = [nodeID, parent, grandparent, ..., rootID] — built
    // once from the subtree's own parent pointers (already in `nodes`),
    // reusing domain.ResolveGrant unchanged: it only needs an ancestor
    // chain and a grant map, not a live DB call, for each node.
    visible := make([]domain.Task, 0, len(nodes))
    for _, n := range nodes {
        chain := chainOf(n.ID, nodes) // walks n.ParentID pointers within `nodes`, no extra query
        if _, ok := domain.ResolveGrant(chain, grantsByTask, caller, uc.maxDepth); ok {
            visible = append(visible, n)
        }
    }
    return GetSubtreeResult{Tasks: visible, DependsOnEdges: filterDependsOn(edges, idsOf(visible))}, nil
}
```

This reuses `domain.ResolveGrant` (`grant_resolution.go:37-67`) completely
unchanged — `GetSubtree` is a batch caller of the same pure function
`ResolvePermission` already calls once, not a second access-control
algorithm. **Owner-intrinsic short-circuit** (SOL-TG-03's fix) applies here
too automatically once wired, since both usecases share the same
`ResolveGrant` call site shape.

`Repository.GetSubtree` (new, `adapter/postgres/subtree.go`) is one
`WITH RECURSIVE` query walking `parent_id` downward (mirror image of
`GetAncestors`'s upward walk, `repository.go:106-122`), batching by 50 IDs
only if the recursive CTE itself proves too slow at realistic fan-out per
`task-service.md §8`'s benchmarking note — a single recursive query is
tried first since Postgres already batches internally; the spec's explicit
"BFS batching 50 IDs at a time" describes the *application-level* algorithm
TS used before it had a recursive-CTE option, not a hard requirement to
replicate query-shape-for-query-shape (this service already deviates from
TS's per-hop application loop for `GetAncestors`, per `task-service.md §6`'s
own "not a large enough surface... to justify" framing for a different
tradeoff in the same spirit).

### `RecalculateProgress` — bottom-up cascade over one `WITH RECURSIVE` aggregate

Per `task-service.md §8`: "use one `WITH RECURSIVE` aggregate query rather
than N+1 fetches." Design: one query returns every task in the subtree with
its depth and its direct children's current `progress_percent` values;
the usecase reduces bottom-up (deepest depth first) calling
`domain.CalculateProgress` per node, then persists every changed value in
one batched `UPDATE ... FROM (VALUES ...)`:

```go
// internal/usecase/recalculate_progress.go
func (uc *RecalculateProgress) Execute(ctx context.Context, rootID string) error {
    tenantID, _ := tenant.RequireTenantID(ctx)
    subtree, err := uc.tasks.GetSubtreeWithChildPercents(ctx, tenantID, rootID) // new repo method
    // subtree ordered deepest-first (ORDER BY depth DESC in the CTE)
    updates := map[string]int{}
    childPercentsByParent := map[string][]int{}
    for _, node := range subtree {
        childPercents := childPercentsByParent[node.ID] // filled as children are processed, deepest-first
        p := domain.CalculateProgress(node.Task, childPercents)
        updates[node.ID] = p
        if node.ParentID != "" {
            childPercentsByParent[node.ParentID] = append(childPercentsByParent[node.ParentID], p)
        }
    }
    return uc.tasks.BatchUpdateProgress(ctx, tenantID, updates) // one UPDATE...FROM(VALUES...)
}
```

Called from three places: (1) explicitly via the new `RecalculateProgress`
RPC, (2) from `UpdateTask` whenever a status transition reaches `Done`
(cascades the parent chain — reuse `GetAncestors` to find the chain, then
call this usecase on the topmost ancestor once, not once per ancestor), and
(3) from `AIApply`'s transaction after committing new subtasks (a newly
created subtree starts at 0% for its new parent, worth persisting
immediately rather than leaving stale until the next explicit call).

### Auto-block on unmet dependency — `AddEdge` extension, and atomic cycle-check

Two fixes land in the same usecase since both touch the same
read-then-write shape BUG-TG-01/§8 already flags as non-atomic
(`add_edge.go:41-54`'s own comment):

```go
func (uc *AddEdge) Execute(ctx context.Context, in AddEdgeInput) (domain.TaskEdge, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    // ...
    edge, err := domain.NewTaskEdge(in.FromTaskID, in.ToTaskID, in.Kind)
    // ...
    return uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) (domain.TaskEdge, error) {
        if edge.Kind == domain.EdgeKindDependsOn {
            // SELECT ... FOR UPDATE against the depends_on edge set (or a
            // serializable tx, per task-service.md §8) closes the race the
            // old comment flagged — see this file's original doc comment,
            // now stale once this lands.
            existing, err := edges.ListByKindForUpdate(ctx, tenantID, domain.EdgeKindDependsOn)
            if err != nil { return domain.TaskEdge{}, ... }
            if domain.DetectCycle(existing, edge) {
                return domain.TaskEdge{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_CYCLIC_DEPENDENCY", ...)
            }
        }
        if err := edges.Add(ctx, tenantID, edge); err != nil { return domain.TaskEdge{}, ... }

        // Auto-block: adding "from depends_on to" means "from must wait for
        // to" (task_edge.go:45's convention) — if `to` isn't Done, `from`
        // transitions to Blocked. Uses SetStatus's existing terminal-state
        // guard (task.go:119-131) so a Done/Cancelled `from` task can't be
        // silently reopened as Blocked by an edge add.
        if edge.Kind == domain.EdgeKindDependsOn {
            dep, err := tasks.Get(ctx, tenantID, edge.ToTaskID)
            if err != nil { return domain.TaskEdge{}, ... }
            if dep.Status != domain.StatusDone && dep.Status != domain.StatusCancelled {
                if err := tasks.UpdateStatus(ctx, tenantID, edge.FromTaskID, domain.StatusBlocked); err != nil {
                    return domain.TaskEdge{}, ...
                }
            }
        }
        return edge, nil
    })
}
```

**Un-blocking** (the flip side): when `UpdateTask` transitions a task INTO
`StatusDone`, it must re-check every task that `depends_on` it and clear
`Blocked` for any whose *every* dependency is now done. Design: `UpdateTask`
gains a post-write step — `edges.ListTo(ctx, tenantID, taskID,
EdgeKindDependsOn)` (a new, symmetric `EdgeRepository` method alongside the
existing `ListFrom`) to find dependents, then for each dependent currently
`Blocked`, re-check all of *its* `depends_on` edges via `ListFrom` and clear
to `Open` only if none remain unmet. This is a small, bounded fan-out (a
task's direct dependents, not the whole graph) — no new perf concern beyond
what `task-service.md §8` already scopes for edge mutations.

### `AddComment`/`ListComments`

Thin usecases, same shape as `CreateTask`/`ListTasks` — `AddComment` builds
a `domain.TaskComment` via `NewTaskComment`, persists via a new
`CommentRepository` port; `ListComments` is a plain tenant+task-scoped
`SELECT ... ORDER BY created_at`, cursor-paginated identically to
`ListTasks` (`list_tasks.go:33-43`).

## Design — proto additions

```protobuf
message Task {
  // ...existing 6 fields unchanged, field numbers unchanged...
  string description = 7;
  string task_type = 8;
  string priority = 9;
  string assignee_id = 10;
  string owner_id = 11;
  google.protobuf.Timestamp due_date = 12;
  google.protobuf.DoubleValue estimated_hours = 13;
  google.protobuf.DoubleValue actual_hours = 14;
  string prompt_template = 15;
  string ai_context = 16;
  string ai_plan_json = 17;
  string visibility = 18;
  string worktree_id = 19;
  string agent_session_id = 20;
  int32 progress_percent = 21;
}

message GetSubtreeRequest { string root_id = 1; }
message GetSubtreeResponse {
  repeated Task tasks = 1;
  repeated AddEdgeRequest depends_on_edges = 2; // reuses the existing edge shape rather than a new message
}

message RecalculateProgressRequest { string root_id = 1; }
message RecalculateProgressResponse { int32 progress_percent = 1; }

message AddCommentRequest { string task_id = 1; string content = 2; }
message AddCommentResponse { string id = 1; string author_id = 2; string content = 3; string created_at = 4; }
message ListCommentsRequest { string task_id = 1; string page_token = 2; int32 page_size = 3; }
message ListCommentsResponse { repeated AddCommentResponse comments = 1; string next_page_token = 2; }
```

`CreateTaskRequest`/`UpdateTaskRequest` grow matching optional fields
(wrapper types for scalars, per `UpdateTaskRequest`'s existing
`StringValue`-typed precedent, `task.proto:156-160`) for every new `Task`
field that's client-settable (`description`, `task_type`, `priority`,
`assignee_id`, `due_date`, `estimated_hours`, `prompt_template`,
`ai_context`, `visibility`) — `owner_id`/`agent_session_id`/`worktree_id`/
`ai_plan_json`/`progress_percent`/`actual_hours` are server-computed or
set only by other usecases (SOL-TG-03/SOL-TG-02/SOL-TG-04), never
client-settable via `UpdateTask` directly.

## Design — wiring (REST/wscompat)

- REST: `GET /v1/tasks/{id}/subtree`, `POST /v1/tasks/{id}/progress:recalculate`,
  `POST /v1/tasks/{id}/comments`, `GET /v1/tasks/{id}/comments` added to
  `task_routes.go`, following `handleGetTask`'s existing translation
  pattern (`task_routes.go:18-40`).
- `wscompat`: `task.getSubtree`, `task.addComment`, `task.listComments`
  added to `registerTaskChannels` (`channels.go:222-260`), same
  `decodeArg`/`client.<RPC>` shape every other channel in that file uses.
  `task.list`/`task.update`/`task.delete`/`task.getDependencies` (already
  real RPCs per BUG-TG-01's "See also" note) are wired in the same pass —
  closing BUG-034's WS-wiring gap for this domain's structural RPCs
  alongside the new ones, since both changes touch the same function.

## Test plan

- `domain/task_test.go` — `StatusBlocked`/`StatusReview` added to
  `validStatus`; `SetStatus` transition-matrix table extended with
  Blocked→Open, Open→Blocked, and the still-forbidden →InProgress case.
- `domain/progress_test.go` — leaf Done→100, leaf not-Done→0, uniform
  children average, mixed-depth cascade (grandchild 100% → child 100% →
  parent 100%; one grandchild 0% → child 50% → parent computed from real
  child percents, not a naive re-walk).
- `usecase/get_subtree_test.go` — fake `TaskRepository`/`GrantRepository`/
  `TeamScopeResolver`: a subtree with a mid-tree task the caller has no
  grant on is excluded but its own children (if independently granted) are
  not — matches spec's per-task filter, not a whole-branch cut.
- `usecase/add_edge_test.go` — new cases: adding a `depends_on` edge to a
  not-Done task sets `from` to `Blocked`; to an already-Done task leaves
  `from`'s status untouched; concurrent `AddEdge` calls racing the same
  cycle window (two goroutines, fake repo with an injected delay) — assert
  only one succeeds once the atomic tx lands.
- `usecase/update_task_test.go` — transitioning a dependency to `Done`
  un-blocks a single-dependency dependent; a multi-dependency dependent
  stays `Blocked` until every dependency clears.
- `usecase/recalculate_progress_test.go` — three-level fixture matching
  the domain test's cascade case, asserting `BatchUpdateProgress` is
  called once with every changed node, not once per node (regression guard
  against N+1).
- `adapter/postgres` integration tests — `GetSubtree` against a real
  multi-branch tree; `ListComments` cursor pagination; migration
  `0003`'s `up`→`down`→`up` cycle (per `05-data-architecture.md`'s CI
  requirement).

## References

- `docs/logic/task-graph/BL-TG-01-task-graph-crud.md` — full spec
- `specs/backend-go/tdd/services/task-service.md:39-68` (§3 RPC sketch,
  already lists `GetSubtree`/`RecalculateProgress`/`AddComment`/
  `ListComments`), `:98-116` (§4 domain model), `:155-213` (§5 data model,
  §6 query-shape note), `:286-301` (§8 NFRs — grant-resolution hot path,
  progress-recalc batching, edge-mutation atomicity)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:58-73`
  (migration/expand-contract conventions this migration follows)
- `backend-go/services/task-service/internal/domain/task.go:12-131`,
  `task_edge.go:81-113`, `grant_resolution.go:37-67`
- `backend-go/services/task-service/internal/adapter/postgres/repository.go:97-143`
  (`GetAncestors`, the shape `GetSubtree` mirrors)
- `backend-go/services/task-service/internal/usecase/add_edge.go:41-54`
  (non-atomic cycle check, the bug this solution closes)
- `backend-go/services/task-service/README.md:139-157` (known
  design-doc/proto deviations, incl. the RPC-surface gap this solution
  closes)
- `backend-go/services/task-service/migrations/0001_init.up.sql:13-93`
  (existing schema, incl. unused `task_comments`)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md`
  — format/rigor precedent this solution follows
