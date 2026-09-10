# BUG-018: `orchestration.*` channels not implemented in backend-go

**Service:** `api-gateway` (WS compat layer) / `orchestration-service` (owning gRPC service)
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** Low-Medium
**Symptom:** `orchestration.dispatchShow` falls through to `registry.go`'s `notImplementedHandler` and returns an error immediately.
**Status:** ✅ Resolved — see TASK-110–112 (3 task(s), all DONE) for implementation evidence.

---

## Description

`wscompat/channels.go` registers 13 real channels total (`annotation.*`, `task.{create,get}`,
`git.{status,diff}`, `automation.runNow`, `preflight.check`, `devServer.{list,add}`,
`fleet.health.checkAll`, `crashReports.getLatestPending`, `rateLimits.get` — file header comment
at `channels.go:16-19`). None of them is an `orchestration.*` channel:

```
grep -n '"orchestration\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
```

returns zero matches. `orchestration.dispatchShow`, the only `orchestration.*` method this
frontend's `callRuntimeRpc` catalog actually calls
(`specs/frontend/api/rpc-catalog.md:349`), falls through to `notImplementedHandler`.

**Scope note:** this is narrower than the full `orchestration.*`/`orchestration-gates.*` surface
the old TS backend exposes (`specs/frontend/api/backend-agent-execution-boundary.md:140`) — only
`dispatchShow` is confirmed reachable via `callRuntimeRpc` from this frontend, so only that one
method is reported here as missing.

`orchestration-service` genuinely exists in backend-go and is REST-wired: `/v1/orchestration` →
`orchestration-service`, `orca.orchestration.v1`, `RouteWired`
(`backend-go/services/api-gateway/internal/domain/registry.go`), backed by 4 RPCs —
`CreateDispatchContext`, `CreateGate`, `ResolveGate`, `UpdateTaskStatusAndPromote`
(`backend-go/proto/orca/orchestration/v1/orchestration.proto:12-18`), each with a REST handler in
`backend-go/services/api-gateway/internal/adapter/httpgateway/orchestration_routes.go:19-25`
(`POST /dispatch-contexts`, `POST /gates`, `POST /gates/{id}/resolve`, `PUT /tasks/{id}/status`).

None of these four is a read/"show" RPC. `orchestration.dispatchShow`'s frontend usage
(`terminal-orchestration-task-links.ts:59-61`) looks up an existing dispatch by
`orchestration_task_id` and reads back `dispatch.assignee_handle` to know which terminal a task
was dispatched to (used to focus that terminal). The closest proto message,
`DispatchContext` (`orchestration.proto:20-27`), only has `id`, `handle`,
`coordinator_run_id`, and `orchestration_task_id` — there is no `assignee_handle` field, and no
RPC to fetch a `DispatchContext` by task ID (only `CreateDispatchContext`, which is a write, not a
lookup). So this is a genuinely unusual method name with no matching read RPC in the current
proto at all, not just a param-shape mismatch.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `orchestration.dispatchShow` | `components/terminal-pane/terminal-orchestration-task-links.ts:59-61` | No matching RPC. `orchestration-service` has only `CreateDispatchContext`/`CreateGate`/`ResolveGate`/`UpdateTaskStatusAndPromote` (`orchestration.proto:12-18`) — no read/"show" RPC exists to look up a dispatch context by task ID, and `DispatchContext` has no `assignee_handle` field for the frontend to read back. |

---

## Dispatch model

Old TS backend (`specs/frontend/api/backend-agent-execution-boundary.md:140`): 🟢 **relational** —
entirely against `PgOrchestrationDb` (message bus, task queue, gates, coordinator state). No relay
for this namespace's own logic; one side effect, `sendTerminalAgentPrompt`, reaches into the
terminal/PTY layer where relay may occur, but that's a different namespace's concern, not
`dispatchShow`'s.

backend-go's `orchestration-service` already follows the same relational shape (a real Postgres
repository behind the 4 existing RPCs, per `orchestration.proto`'s doc comment: "a distinct id
space from task-service's tasks table"). Adding `dispatchShow` is in scope for that same service —
it needs a new read RPC (e.g. `GetDispatchContext`/`GetDispatchContextByTask`) plus an
`assignee_handle` field (or equivalent) on `DispatchContext`, not a new service or a relay path.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:1-19` — registered-channel inventory (no `orchestration.*`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go` — `/v1/orchestration` → `orchestration-service`, `RouteWired`
- `backend-go/proto/orca/orchestration/v1/orchestration.proto:1-83` — `OrchestrationService`'s full RPC/message surface (4 RPCs; no read/"show" RPC, no `assignee_handle` field)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/orchestration_routes.go:1-26` — the real `/v1/orchestration` REST proxy and its 4 handlers
- `backend-go/services/orchestration-service/internal/usecase/` — `create_dispatch_context.go`, `create_gate.go`, `resolve_gate.go`, `update_task_status_and_promote.go` (no dispatch-read usecase)
- `frontend/src/renderer/src/components/terminal-pane/terminal-orchestration-task-links.ts:50-72` — the only frontend call site, showing the `dispatch.assignee_handle` read
- `specs/frontend/api/backend-agent-execution-boundary.md:140` — old backend's `orchestration.*`/`orchestration-gates.*` dispatch model
- `specs/frontend/api/rpc-catalog.md:349` — authoritative RPC catalog (`orchestration.dispatchShow`)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — original example of this bug shape
