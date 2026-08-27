# BUG-TG-01: Task graph CRUD is real but structural management (subtree load, progress, auto-block, most fields) is missing

**Business Logic:** [BL-TG-01](../../../../docs/logic/task-graph/BL-TG-01-task-graph-crud.md) — Task Graph CRUD & Structural Management
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** A Developer/Lead can create, read, list, update (title/status only), and delete a task, and can add a `depends_on`/`parent_child` edge with real cycle rejection. But they cannot: see any task field beyond `id/title/status/parent_id/project_id` (no description, priority, labels, assignee, due date, estimates, prompt template, AI context, visibility, worktree/agent-session linkage); load a task's full subtree in one call (no `GetSubtree`/`loadTaskTree` RPC); see a computed progress percentage anywhere; or get automatic `blocked` status when a dependency isn't done — none of that exists.

---

## Spec summary

BL-TG-01 defines the `orca_tasks` table with ~20 fields (description, type, priority, labels, assignee/reporter/owner, due_date, estimated/actual hours, prompt_template, ai_context, ai_plan_json, visibility, worktree_id, agent_session_id, workflow_exec_id) plus `orca_task_edges`, `orca_task_grants`, `orca_task_comments`. It specifies four algorithms: (1) create-task validation + parent/project membership checks, (2) add-dependency-edge with cycle detection and auto-block of the dependent task, (3) `loadTaskTree(rootId)` — a BFS that walks the whole subtree batching 50 IDs at a time, filtering by per-task view access, and collecting the dependency edges within the subtree, and (4) `calculateProgress()` from subtask completion percentage, cascading up to parent tasks.

## What backend-go has

- **Domain model** (`backend-go/services/task-service/internal/domain/task.go:52-69`): `Task{ID, TenantID, Title, Status, ParentID, ProjectID}` — 6 fields total, matching the generated proto (`backend-go/proto/orca/task/v1/task.proto:59-65`) exactly. `Status` is a 4-value enum (`open/in_progress/done/cancelled`), not the spec's 7-value one (`backlog/todo/in_progress/blocked/review/done/cancelled`) — critically, there is **no `blocked` status at all**.
- **Real CRUD usecases**, all tenant-scoped and gRPC/REST-wired: `CreateTask` (`internal/usecase/create_task.go:36-62`, validates parent existence), `GetTask` (`internal/usecase/get_task.go:19-30`), `ListTasks` (`internal/usecase/list_tasks.go:33-43`, cursor-paginated by project), `UpdateTask` (`internal/usecase/update_task.go:35-62`, title/status only, blocks transition into `in_progress`), `DeleteTask` (`internal/usecase/delete_task.go:24-36`). All wired in `internal/adapter/grpc/server.go:70-191` and `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go:18-27` (REST) — though only `create/get/edges/grants/permission/execute/active-executions` are mounted there; `list/update/delete` have no REST route, and `wscompat`'s `channels.go:222-260` only wires `task.create`/`task.get` over WebSocket (see BUG-034).
- **Real dependency-edge logic**: `AddEdge` usecase (`internal/usecase/add_edge.go:30-60`) calls `domain.DetectCycle` (`internal/domain/task_edge.go:81-113`, a genuine BFS reachability check, unit-tested per-kind) before persisting a `depends_on` edge, rejecting cycles with `TASK_CYCLIC_DEPENDENCY`. `GetDependencies` (`internal/usecase/get_dependencies.go:32-56`) reads `depends_on` edges forward and hydrates full `Task` rows.
- **Ancestor walk**: `Repository.GetAncestors` (`internal/adapter/postgres/repository.go:101-143`) is a real `WITH RECURSIVE` query up the `parent_id` chain — used internally by `ResolvePermission`, not exposed as its own RPC.
- **`task.task_comments` table exists** in `migrations/0001_init.up.sql:82-93` (with RLS) but is dead schema — no repository, usecase, or RPC touches it anywhere in the service.

## What's missing

- **No `GetSubtree`/`loadTaskTree`-equivalent RPC.** The proto only exposes an ancestor-direction walk (`GetAncestors`, internal-only) — there is no descendant/subtree read, so a client cannot load "this epic + all its subtasks" in one call the way the spec's BFS does. Confirmed absent from `task.proto` (`backend-go/proto/orca/task/v1/task.proto:10-27`, 13 RPCs total, none subtree-shaped) and from every file under `internal/usecase/`.
- **No access filtering during any tree/list read** — `loadTaskTree`'s per-task `hasTaskAccess(userId, task, 'view')` filter has no equivalent; `ListTasks` (`internal/usecase/list_tasks.go`) returns all tasks in a project/tenant with no per-task grant check at all.
- **No `calculateProgress()` and no cascade.** No field, function, or query anywhere in `backend-go/services/task-service/` computes a subtask-completion percentage or propagates it to a parent (confirmed via grep — zero hits for `progress`/`CalculateProgress` outside the "in_progress" status string).
- **No auto-block on unmet dependency.** Adding a `depends_on` edge never inspects or changes the dependent task's status; there is no `blocked` status value in `domain.Task`'s enum at all (`internal/domain/task.go:12-17`), so "Task A → status='blocked' because Task B isn't done" cannot even be represented, let alone computed.
- **Missing ~15 of ~20 spec fields**: `description`, `type`, `priority`, `labels`, `assignee_id`, `reporter_id`, `owner_id`, `due_date`, `estimated_hours`, `actual_hours`, `prompt_template`, `ai_context`, `ai_plan_json`, `visibility`, `worktree_id`, `agent_session_id`, `workflow_exec_id` — none exist on `domain.Task` or the generated proto message (`task.proto:59-65`). This also means BL-TG-04's task→agent linkage (`worktree_id`/`agent_session_id`) has nowhere to be stored even if execution were fully wired.
- **`task_comments` (spec's comment thread) is entirely unimplemented** — table exists, nothing reads/writes it.
- **`AddEdge`'s cycle-check-then-write is not atomic** (`internal/usecase/add_edge.go:41-54`'s own comment) — a race window exists between the `ListByKind` read and the `Add` write, unlike the spec's implicit single-transaction expectation.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-034-task-channels-not-implemented.md` — documents the WS-channel wiring gap for `task.list`/`task.update`/`task.delete`/`task.getDependencies` (now stale in its RPC-existence claims: all 4 RPCs exist server-side today, contrary to that bug's "no backing RPC anywhere" finding — only the WS wiring gap it describes is still current).

## References

- `docs/logic/task-graph/BL-TG-01-task-graph-crud.md` — full spec (data model, create/add-edge/loadTaskTree/calculateProgress flows)
- `backend-go/proto/orca/task/v1/task.proto:10-27,59-65` — `TaskService` RPC surface and `Task` message (6 fields)
- `backend-go/services/task-service/internal/domain/task.go:12-17,52-69` — `Task` struct, status enum (no `blocked`)
- `backend-go/services/task-service/internal/domain/task_edge.go:81-113` — `DetectCycle`
- `backend-go/services/task-service/internal/usecase/create_task.go`, `get_task.go`, `list_tasks.go`, `update_task.go`, `delete_task.go`, `add_edge.go`, `get_dependencies.go`
- `backend-go/services/task-service/internal/adapter/postgres/repository.go:101-143` — `GetAncestors` (recursive CTE, internal-only)
- `backend-go/services/task-service/migrations/0001_init.up.sql:13-93` — `task.tasks`/`task.task_edges`/`task.task_grants`/`task.task_comments` schema (comments table unused)
- `backend-go/services/task-service/README.md:139-157` — "Known deviations" section, confirms field-set and RPC-surface gaps against the design doc
- `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go:18-27` — REST surface (partial)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:222-260` — WS surface (only `create`/`get` wired)
