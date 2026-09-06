# BE-SOL-001: Wire the 7 missing `WorkflowServiceClient` RPCs into wscompat

> **✅ COMPLETED (2026-09-06)** — `go build ./...` clean for `api-gateway`; full
> `go test ./...` for `api-gateway`: all packages pass, including
> `internal/adapter/wscompat` (24.5s, includes the new tests below). No proto
> change, no other backend-go service touched.

**CR:** [CR-PW-005](../../../../../../docs/crs/v3/project-workspace/CR-PW-005-wscompat-missing-workflow-rpcs.md)
**Service:** api-gateway (wscompat layer only — no change to workflow-service itself)
**Frontend counterpart:** [specs/frontend/crs/v3/project-workspace/](../../../../frontend/crs/v3/project-workspace/solutions/README.md) (CR-PW-004/006)
**Status:** ✅ COMPLETED (2026-09-06)

---

## 1. Problem

`workflowv1.WorkflowServiceClient` has 11 RPCs, all already generated and
implemented server-side (`services/workflow-service/internal/adapter/grpc/server.go`).
`api-gateway`'s `registerWorkflowChannels` (`channels_workflow.go`) only
registered 4: `workflow.execute`, `workflow.cancel`, `workflow.template.create`,
`workflow.template.update`. A Web-mode frontend call to any of the other 7 —
`GetExecution`, `PauseExecution`, `ResumeExecution`, `ListTemplates`,
`ResolveTemplate`, `HasActiveExecutions`, `ExecuteAdHocStep` — hit "unknown
channel" at the wscompat dispatch layer, even though Electron/local mode
(which talks to the legacy TS `workflow-rpc-handler.ts` directly) worked fine.

## 2. Solution — pure wiring, no proto change

`channels_workflow.go`: added 7 `r.Register(...)` blocks, one per missing RPC,
in the exact style of the 4 existing ones (decode a small camelCase args
struct via `decodeArg`, `gatewaygrpc.AttachIdentity` with `TenantID`/`UserID`
from the resolved `Identity` — never from args — then call the gRPC client
method and return its response).

- `workflow.getExecution` / `.pause` / `.resume` — all take `{executionId}`,
  map to `Id` on their respective `*Request`, return `resp.GetExecution()`.
- `workflow.template.list` — `{scope, pageToken, pageSize}` →
  `ListTemplatesRequest`; returns `{templates, nextPageToken}` with `templates`
  normalized to `[]` (never `null`) when empty, matching the established
  list-channel convention (TASK-BE-008's `devServerGroup.list` precedent).
- `workflow.template.resolve` — `{templateId}` → `ResolveTemplateRequest`;
  returns `{template, chain}` with `chain` also normalized to `[]`.
- `workflow.hasActiveExecutions` — `{projectId}` → `HasActiveExecutionsRequest`;
  returns `{hasActive}`.
- `workflow.executeAdHocStep` — `{stepType, stepConfigJson, requestId}` →
  `ExecuteAdHocStepRequest`. `TenantId` is always taken from `Identity`, never
  from args (same security rule as `workflow.template.create`'s existing
  regression test — a malicious/buggy frontend payload must not be able to
  run a step under a different tenant). `stepType` (a string like `"shell"`)
  is parsed via the **already-existing** `parseStepType` helper in
  `channels_automation_task.go` — reused, not duplicated (that helper's own
  comment already says it's shared by `task.*`/`automation.*` channels for
  the same `workflowv1.StepType` enum).

## 3. Known pre-existing issue, NOT fixed here (documented, deliberately out of scope)

All 4 pre-existing channels (and now, by mirroring their exact shape, the 7
new ones too) return the raw `*workflowv1.X` proto message directly
(`resp.GetExecution()`, `resp.GetTemplate()`) rather than an explicit
camelCase view struct. protoc-gen-go's plain `encoding/json` struct tags are
snake_case (`json:"template_id,omitempty"`; the camelCase `json=templateId`
tag is protojson-only) — and this wscompat envelope serializes `Result any`
via plain `encoding/json`, not `protojson`. This is the exact same finding
already made and fixed in `channels_dev_server_access_control.go`/
`channels_tenant_project.go` (via a dedicated camelCase view struct) — **not**
applied retroactively to `channels_workflow.go` in this pass, since CR-PW-005's
scope is "wire the missing RPCs," not "fix the response shape of the ones
that already shipped." Flagged here for a dedicated follow-up that would
touch all 11 channels at once (changing the shape of the 4 already-live ones
is a bigger, separate change than adding the missing 7).

## 4. Tests

`channels_workflow_test.go`: extended `fakeWorkflowServiceClient` with the 7
new method overrides (mirrors `fakeInfraFleetClient`'s embed-interface
pattern already used in this package), added:

- `TestWorkflowGetExecutionChannel_Success`
- `TestWorkflowPauseChannel_Success`
- `TestWorkflowResumeChannel_Success`
- `TestWorkflowTemplateListChannel_EmptyReturnsEmptyArrayNotNull` (guards the
  `[]`-not-`null` convention)
- `TestWorkflowTemplateListChannel_ForwardsScopeAndPagination`
- `TestWorkflowTemplateResolveChannel_Success` (asserts the 2-element
  inheritance `chain` round-trips)
- `TestWorkflowHasActiveExecutionsChannel_Success`
- `TestWorkflowExecuteAdHocStepChannel_TenantIDComesFromIdentityNotArgs`
  (mirrors `TestWorkflowTemplateCreateChannel_TenantIDComesFromIdentityNotArgs`'s
  existing regression-guard pattern for a different RPC)

All 13 tests in the file (4 pre-existing + 9 new) pass:

```bash
cd backend-go/services/api-gateway && go test ./internal/adapter/wscompat/... -run TestWorkflow -v
# --- PASS x13, ok
```

## 5. Verification

```bash
cd backend-go/services/api-gateway && go build ./...        # clean
cd backend-go/services/api-gateway && go test ./...          # all packages pass
gofmt -l internal/adapter/wscompat/channels_workflow.go internal/adapter/wscompat/channels_workflow_test.go
# → empty (gofmt-clean)
```

`gitnexus impact({target:"registerWorkflowChannels", direction:"upstream"})` before editing:
risk **LOW**, impactedCount 3 (1 direct caller — `api-gateway/cmd/server/main.go`'s `run`), 1
execution flow noted but 0 flagged as broken.

## 6. Checklist

- [x] 7 new wscompat channels registered, mirroring the existing 4's exact `r.Register` shape.
- [x] `workflow.executeAdHocStep`'s `TenantId` always from `Identity`, tested.
- [x] List-shaped channels (`workflow.template.list`, `.template.resolve`) return `[]` not `null`
      when empty, tested.
- [x] `parseStepType` reused from `channels_automation_task.go`, not duplicated.
- [x] No proto change; no other backend-go service touched.
- [x] `go build ./...` + `go test ./...` clean for `api-gateway`.
- [x] Pre-existing snake_case/camelCase response-shape issue documented, not silently expanded
      into this pass's scope.
