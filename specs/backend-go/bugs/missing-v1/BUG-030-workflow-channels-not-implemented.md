# BUG-030: `workflow.*` channels not implemented in backend-go

**Service:** `workflow-service` (via `api-gateway`)
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** High — workflow automation is a headline feature; the backing RPCs exist and are already REST-wired, only the WS compat wrapper is missing
**Symptom:** Every `workflow.*` call the automation UI makes (`workflow.execute`, `workflow.cancel`, `workflow.template.create`, `workflow.template.update`) falls through to `notImplementedHandler` and times out client-side, even though the equivalent REST endpoints work today
**Status: ❌ Open**

---

## Description

`specs/frontend/api/rpc-catalog.md` lists 4 `workflow.*` methods, called from
`frontend/src/renderer/src/hooks/useWorkflowExecution.ts` and
`useWorkflow.ts`:

```
grep -n '"workflow\.' services/api-gateway/internal/adapter/wscompat/channels.go
```

returns **zero matches** — no `workflow.*` channel is registered in
`RegisterRealChannels` (`channels.go:79-89`). Every call reaches
`registry.go`'s `notImplementedHandler` (`registry.go:59`).

This is the strongest "just needs a wscompat wrapper" namespace in this
audit: `workflow-service` is fully real (DAG wave-dispatch, Kahn's
topological sort, bounded worker pool, `StepExecution` persistence — see
`docs/execution-plan.md:694`), already exposes the exact RPCs this namespace
needs, and is already REST-proxied at `/v1/workflows`
(`services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:28-38`):

- `workflow.execute` → `Execute` RPC (`proto/orca/workflow/v1/workflow.proto:13`), wired at `POST /v1/workflows/executions` → `handleExecuteWorkflow` (`workflow_routes.go:150-172`)
- `workflow.cancel` → `CancelExecution` RPC (`workflow.proto:23`), wired at `POST /v1/workflows/executions/{id}/cancel` → `handleCancelExecution` (`workflow_routes.go:224-236`); built in the `docs/execution-plan.md:564-568` follow-up pass alongside `ListTemplates`/`ResolveTemplate`
- `workflow.template.create` → `CreateTemplate` RPC (`workflow.proto:12`), wired at `POST /v1/workflows/templates` → `handleCreateTemplate` (`workflow_routes.go:52-74`)
- `workflow.template.update` → **no backing RPC** (see below)

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `workflow.execute` | `useWorkflow.ts:82` | Backing RPC exists and is REST-wired: `Execute` (`workflow.proto:13`), `handleExecuteWorkflow` (`workflow_routes.go:150-172`). Just needs a wscompat wrapper around the same `workflowv1.WorkflowServiceClient`. |
| `workflow.cancel` | `useWorkflowExecution.ts:39` | Backing RPC exists and is REST-wired: `CancelExecution` (`workflow.proto:23`), `handleCancelExecution` (`workflow_routes.go:224-236`). Just needs a wscompat wrapper. |
| `workflow.template.create` | `useWorkflow.ts:64` | Backing RPC exists and is REST-wired: `CreateTemplate` (`workflow.proto:12`), `handleCreateTemplate` (`workflow_routes.go:52-74`). Just needs a wscompat wrapper. |
| `workflow.template.update` | `useWorkflow.ts:56` | **No backing RPC.** `workflow.proto`'s `WorkflowService` (lines 12-42) has no `UpdateTemplate` RPC. Explicitly documented as a deliberate gap: `services/workflow-service/internal/domain/template.go:45` ("no UpdateTemplate RPC to rewire an existing template's parent after [creation]") and `services/workflow-service/README.md:233`. A template's settings/DAG/parent can only be set at creation time today — this call has nowhere to go until `UpdateTemplate` is added to the proto and implemented. |

3 of 4 methods (`execute`, `cancel`, `template.create`) are confirmed
"just needs a wscompat wrapper" — real usecases, Postgres persistence, DAG
dispatch, and REST wiring already exist and are exercised in production via
`/v1/workflows`. Only `template.update` needs new service-level work
(a new RPC, not present anywhere in the proto).

---

## Dispatch model

🟢 for the RPC call itself — writes to `workflow.templates`/
`workflow.executions`/`workflow.step_executions` (backend-go's Postgres
schema mirrors the old TS backend's `orca_workflow_templates`/
`orca_workflow_executions`/`orca_workflow_step_executions`).
`workflow.execute` returns immediately (DB-only, synchronous DAG validation)
but dispatches the actual execution asynchronously
(`docs/execution-plan.md:694`: "`Execute` now dispatches asynchronously on a
detached background goroutine after synchronous DAG validation... steps can
run up to 30 minutes per §8").

Per-step dispatch already matches the old TS backend's split, confirmed in
backend-go's own step-executor registry
(`services/workflow-service/internal/adapter/stepexecutors/registry.go`):

- `agent`/`shell` steps relay to the Dev Server Agent — `internal/adapter/infrafleetclient/agent_step_executor.go`, `shell_step_executor.go`
- `notification` steps relay to the Dev Server Agent — `internal/adapter/infrafleetclient/notification_step_executor.go`
- `webhook` steps run a native fetch on backend — `internal/adapter/stepexecutors/webhook.go`
- `condition` steps are a pure in-memory expression evaluator — `internal/domain/condition.go`

Per-step status is always written to Postgres regardless of where the step
ran, matching the old backend's behavior.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go:79-89` — `RegisterRealChannels` (no `workflow.*` registration)
- `services/api-gateway/internal/adapter/wscompat/registry.go:59` — `notImplementedHandler`
- `proto/orca/workflow/v1/workflow.proto:12-42` — `WorkflowService` RPC surface (no `UpdateTemplate`)
- `services/api-gateway/internal/adapter/httpgateway/workflow_routes.go:28-38` — existing `/v1/workflows` REST→gRPC proxy, all handler line refs above
- `services/workflow-service/internal/domain/template.go:45` — documented `UpdateTemplate` gap
- `services/workflow-service/README.md:233` — same gap, service README
- `services/workflow-service/internal/adapter/stepexecutors/registry.go` — step-type dispatch registry (webhook/condition/agent/shell/notification)
- `backend-go/docs/execution-plan.md:564-581,694` — `ListTemplates`/`ResolveTemplate`/`CancelExecution` build log; DAG wave-dispatch build log
- `frontend/src/renderer/src/hooks/useWorkflowExecution.ts:39`, `useWorkflow.ts:56,64,82` — all 4 call sites
- `specs/frontend/api/rpc-catalog.md` — `workflow.*` catalog entries
