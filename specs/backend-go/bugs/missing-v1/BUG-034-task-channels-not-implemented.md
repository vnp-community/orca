# BUG-034: `task.*` — 7 of 7 methods the frontend actually calls are unimplemented in backend-go

**Service:** `task-service` (via `api-gateway`'s `wscompat` WS layer)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** High — task graph execution (including AI decompose/apply) is a headline feature, and it is currently 100% unreachable from the frontend's real call surface.
**Symptom:** Every `task.*` call the frontend actually makes resolves through `notImplementedHandler` and errors out (see BUG-002 for the general "unregistered channel" failure mode).
**Status:** ✅ Resolved — see TASK-222–226 (5 task(s), all DONE) for implementation evidence.

---

## Description

**Lead finding — naming drift, read this first:** `wscompat` *does*
register two channels under the `task.*` prefix —
`registerTaskChannels` (`channels.go:182-215`) wires `task.create`
(`channels.go:183`) and `task.get` (`channels.go:201`), both calling real
`task-service` gRPC methods (`CreateTask`/`GetTask`). **But neither
`task.create` nor `task.get` appears anywhere in the frontend's actual
`task.*` call list** (`specs/frontend/api/rpc-catalog.md:435-445` — the
full 7-method table below). So the 2 "wired" task channels are dead code
from this audit's methodology: no call site in the rpc-catalog reaches them
via this WS protocol. It's possible they're reached some other way this
audit doesn't cover (e.g. an older/parallel call path), but that could not
be confirmed here — flagging as an open uncertainty, not a settled fact.

Net effect: **all 7 methods the frontend actually calls
(`task.aiApply`, `task.aiDecompose`, `task.delete`, `task.execute`,
`task.getDependencies`, `task.list`, `task.update`) are unimplemented**,
despite `channels.go` appearing at first glance to have "2 of 9 task
channels" wired. This is effectively a 0-of-7 gap dressed up as "2
channels wired" — worth calling out as a finding in its own right, since it
suggests naming drift between an earlier task-service design (create/get,
presumably matching an earlier proto) and the current frontend contract.

Of the 7 methods actually missing:

- **1 method has a complete, real backing RPC already implemented and
  exposed over REST — it just isn't wired into `wscompat`:** `task.execute`
  → `Execute` (`TaskServiceExecuteRequest`/`Response`). Defined in
  `task.proto` (`backend-go/proto/orca/task/v1/task.proto:16`), backed by a
  real usecase (`internal/usecase/execute_task.go`) and gRPC server method
  (`internal/adapter/grpc/server.go:114`), with a REST equivalent at
  `internal/adapter/httpgateway/task_routes.go:25`
  (`POST /v1/tasks/{id}/execute`). Adding this to `wscompat` is a thin
  wrapper — **but see the caveat below**: the usecase's branch decision is
  real, its two executors are stubs.
- **6 methods have no backing RPC anywhere in `task-service`:**
  `task.aiApply`, `task.aiDecompose`, `task.delete`, `task.getDependencies`,
  `task.list`, `task.update`. `task.proto` defines only 7 RPCs total:
  `CreateTask`, `GetTask`, `AddEdge`, `Grant`, `ResolvePermission`,
  `Execute`, `HasActiveExecutions` (`task.proto:10-27`) — none is a
  list-all/update/delete/get-dependencies/AI-decompose/AI-apply RPC.
  Genuinely unbuilt server-side, not just unwired.

---

## Already wired (do not re-report as missing — but note the dead-code finding above)

| Method | What it does | File:line | Frontend call site? |
|---|---|---|---|
| `task.create` | Calls `TaskServiceClient.CreateTask` | `channels.go:183-199` | **None found in rpc-catalog.md's `task.*` table** — likely dead from this WS surface |
| `task.get` | Calls `TaskServiceClient.GetTask` | `channels.go:201-214` | **None found in rpc-catalog.md's `task.*` table** — likely dead from this WS surface |

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `task.execute` | `TaskDetail.tsx`, `TaskPromptEditor.tsx` | Backing RPC exists: `Execute` (`task.proto:16`, `server.go:114`, REST at `task_routes.go:25`). Wrapper-only gap for the *dispatch decision* — but `execute_task.go`'s doc comment (lines ~15-30) states both the `SimpleExecutor` (infra-fleet-service) and `ComplexExecutor` (orchestration-service) it dispatches to **are themselves stubs in this scaffold** — only the simple-vs-complex branch logic and the `StatusInProgress` persistence are real. Wiring this channel makes the call reachable but not necessarily functional end-to-end. |
| `task.aiDecompose` | `useTask.ts` | No backing RPC. Not in `task.proto`. Genuinely unbuilt server-side. |
| `task.aiApply` | `useTask.ts` | No backing RPC. Not in `task.proto`. Genuinely unbuilt server-side. |
| `task.delete` | `useTask.ts` | No backing RPC. Not in `task.proto`. No `Delete`-shaped method anywhere in `internal/usecase/`. Genuinely unbuilt server-side. |
| `task.getDependencies` | `TaskDetail.tsx` | No backing RPC. Not in `task.proto`. `AddEdge`/`ResolvePermission` exist for writing/checking the DAG but nothing reads a task's dependency list back out. Genuinely unbuilt server-side. |
| `task.list` | `useTasks.ts` | No backing RPC. Not in `task.proto` — only single-task `GetTask` exists, no list-all-tasks-for-tenant/project RPC. Genuinely unbuilt server-side. |
| `task.update` | `useTask.ts` | No backing RPC. Not in `task.proto`. Per `execute_task.go`'s own doc comment, task-service currently has **no `UpdateTask`/`SetStatus` RPC at all** — even internally, nothing can transition a task's status back out of `in_progress` once `Execute` sets it. Genuinely unbuilt server-side. |

---

## Dispatch model

🟢 **Postgres** for CRUD/grants (old TS backend's
`orca_tasks`/`orca_task_edges`/`orca_task_comments`/`orca_task_grants`).

⚠️ **Two exceptions, important for implementers:**

- `task.aiDecompose` relays to the Dev Server Agent for AI inference
  (`relay.call('ai.complete', ...)`) — **not** a DB read despite looking
  like one.
- `task.execute` branches by complexity: simple tasks relay through the
  same `agent.exec` path as project-spawn; complex tasks (with
  subtasks/dependencies) hand off to a `TaskOrchestrationBridge` → the
  relational orchestration DB, whose worker dispatch also eventually
  reaches the Dev Server Agent. backend-go's `Execute` RPC
  (`task.proto:102-103`'s doc comment: *"branches by complexity: simple
  tasks relay to infra-fleet-service (agent.exec-equivalent), complex ones
  route through orchestration-service"*) mirrors this shape structurally,
  confirming the branch design carried over — but as noted above, both
  target executors remain stubs in the current scaffold.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:182-215` — `registerTaskChannels` (create/get — dead code from the frontend's real call surface)
- `backend-go/proto/orca/task/v1/task.proto:10-27` — `TaskService` (full 7-RPC surface)
- `backend-go/proto/orca/task/v1/task.proto:102-111` — `Execute`/`TaskServiceExecuteRequest` doc comment (complexity-branch design)
- `backend-go/services/task-service/internal/adapter/grpc/server.go:50,64,72,84,97,114,125` — `CreateTask`/`GetTask`/`AddEdge`/`Grant`/`ResolvePermission`/`Execute`/`HasActiveExecutions`
- `backend-go/services/task-service/internal/usecase/execute_task.go` (doc comment ~lines 15-30) — confirms `SimpleExecutor`/`ComplexExecutor` are both stubs; no completion callback exists
- `backend-go/services/api-gateway/internal/adapter/httpgateway/task_routes.go:18-27` — REST equivalents already calling all 7 RPCs (including `/execute`)
- `backend-go/services/api-gateway/internal/domain/registry.go:95` — `/v1/tasks` → `task-service`, `RouteWired`
- `specs/frontend/api/rpc-catalog.md:435-445` — full `task.*` frontend call-site table (7 methods; note `task.create`/`task.get` absent)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug-report format this follows
