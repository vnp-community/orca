# TASK-WF-005-01: `ListExecutions` — fix `WorkflowMonitor.tsx`'s broken-in-production dependency

**From Solution:** BE-SOL-005
**Priority:** P0 — higher than every other BE-SOL-005 task. `WorkflowMonitor.tsx` (the one workflow component actually mounted in the app, per `WorkspaceLayout.tsx`) calls `workflow.listExecutions`, which does not exist on backend-go today — the Workflow tab fails to load for every user on a backend-go target right now. This is a live production bug fix, not new-feature scope, and should ship ahead of BE-SOL-005's sharing/library RPCs (TASK-WF-005-02/-03), which are larger and need a security review pass.
**Service:** `workflow-service` + `api-gateway`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`ListExecutions` RPC), `backend-go/services/workflow-service/internal/usecase/ports.go` (`ExecutionRepository.ListExecutions`), `backend-go/services/workflow-service/internal/usecase/list_executions.go` (new), `backend-go/services/workflow-service/internal/adapter/grpc/server.go`, `backend-go/services/workflow-service/internal/adapter/postgres/repository.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
**Depends on:** None
**Status:** `[x]` DONE

## Execution notes (2026-09-09)

Re-verified live: 11 RPCs, 11 wscompat channels, `ListExecutions` missing
from both — matched the task's own findings exactly.

**Real-code divergence found (the task explicitly asked to verify this
before finalizing):** `WorkflowMonitor.tsx` calls
`callRuntimeRpc<WorkflowExecution[]>(target, 'workflow.listExecutions', {
projectId })` and does `useAppStore.getState().setExecutions(result)`
directly on the decoded result — i.e. it expects the **bare array** as the
top-level RPC result, NOT a `{executions, nextCursor}` envelope like the
task's own sketch proposed (modeled on `workflow.template.list`'s
`{templates, nextPageToken}` wrapping). Implemented to match the real,
already-broken caller: the wscompat channel returns
`[]*workflowv1.WorkflowExecution` directly (`[]` not `null` when empty,
per the established convention). The gRPC/proto layer still carries
`next_cursor` for future pagination UI, just not surfaced through this
particular channel today since nothing consumes it yet.

**Changes made:**
1. `workflow.proto`: added `ListExecutions` RPC + `ListExecutionsRequest`/
   `ListExecutionsResponse` messages; regenerated via `buf generate`.
2. `ports.go`: added `ExecutionRepository.ListExecutions`.
3. `usecase/list_executions.go` (new): modeled on `get_execution.go`'s
   tenant→repo→error-mapping pattern.
4. `adapter/grpc/server.go`: added `Server.ListExecutions` handler +
   wired the new usecase into `Server`'s struct/constructor.
5. `cmd/server/main.go`: wired `usecase.NewListExecutions(repo)` in.
6. `adapter/postgres/repository.go`: implemented `ListExecutions` —
   ordered `created_at DESC, id DESC` (NOT id-based like `ListTemplates`,
   since execution ids are random UUIDs with no chronological meaning);
   cursor is still the opaque last-seen id, resolved back to its
   `created_at` via a subquery so ordering stays correct. Added migration
   `0007_execution_list_index` (`(tenant_id, project_id, created_at DESC,
   id DESC)`) since neither existing execution index covers this
   unfiltered-by-status, newest-first scan.
7. `api-gateway/.../channels_workflow.go`: registered `workflow.listExecutions`
   returning the bare array (see divergence note above).
8. Test fakes updated to satisfy the widened `ExecutionRepository` /
   `WorkflowServiceClient` interfaces (`pause_resume_execution_test.go`'s
   `fakeExecutionRepository` gained `ListExecutions` + insertion-order
   tracking for deterministic "newest first" fake ordering;
   `httpgateway/workflow_routes_test.go`'s `fakeWorkflowServiceClient`
   gained a stub `ListExecutions`; `wscompat/channels_workflow_test.go`'s
   fake gained a real `listExecutionsFunc` hook).

**Verify output:**
```
go build ./services/workflow-service/... ./services/api-gateway/...   # clean
go vet   ./services/workflow-service/... ./services/api-gateway/...   # clean
go test  ./services/workflow-service/internal/usecase/... -run TestListExecutions -v
  # 5/5 PASS (tenant/project filter, newest-first order, cursor pagination, empty, no-tenant)
go test  ./services/api-gateway/internal/adapter/wscompat/... -run TestWorkflowListExecutions -v
  # 2/2 PASS (bare-array shape, empty-returns-[]-not-null)
go test  ./services/workflow-service/... ./services/api-gateway/...   # full suite, all ok
go test -tags=integration ./services/workflow-service/internal/adapter/postgres/... \
  -run TestRepository_ListExecutions_KeysetPaginationNewestFirst -v
  # PASS against a real testcontainers Postgres (created_at-ordered keyset pagination confirmed)
```
Docker was available in this sandbox (`docker ps` showed running
containers), so the real Postgres integration test above ran for real,
not skipped. Running the FULL `-tags=integration` postgres suite back to
back produced flaky failures (`TestRepository_CreateAndGetTemplate`,
`TestRepository_ResolveChain_NotFound`, etc.) — confirmed **pre-existing
sandbox flakiness, not a regression**: reran several of those pre-existing
tests in isolation and they passed; this environment's Docker appears to
struggle with rapid back-to-back testcontainer spin-up/teardown across a
dozen tests, not with anything this change touched.

**Not done — explicitly out of reach in this environment:** the task's
own "Test plan" calls for a "manual/E2E check against a real backend-go
target" confirming `WorkflowMonitor.tsx` loads live, since this exact bug
class shipped past unit tests once already. No running Electron app or
live backend-go deployment was available in this sandbox to drive that
check. Mitigated by reading `WorkflowMonitor.tsx` and its RPC client
(`callRuntimeRpc`) directly to confirm the wire contract instead of
assuming it — see the divergence note above — but a human/CI E2E pass
against a real target is still recommended before this is considered
fully closed.

---

## Context — both halves of this gap re-confirmed live

**1. Proto side.** `grep -n "rpc " backend-go/proto/orca/workflow/v1/workflow.proto`
confirms 11 RPCs today: `CreateTemplate, UpdateTemplate, Execute,
GetExecution, PauseExecution, ResumeExecution, ExecuteAdHocStep,
CancelExecution, ListTemplates, ResolveTemplate, HasActiveExecutions` —
`GetExecution` (singular) exists; `ListExecutions` does not.

**2. wscompat side.** `channels_workflow.go` was re-read directly:
exactly 11 channels are registered — `workflow.execute`,
`workflow.cancel`, `workflow.template.create`, `workflow.template.update`,
`workflow.getExecution`, `workflow.pause`, `workflow.resume`,
`workflow.template.list`, `workflow.template.resolve`,
`workflow.hasActiveExecutions`, `workflow.executeAdHocStep` — confirmed,
`workflow.listExecutions` is not one of them.

`ExecutionRepository` (`internal/usecase/ports.go:41-64`) has
`CreateExecution`, `GetExecution`, `UpdateExecution`,
`HasActiveExecutions`, `ListRunning` — no list-by-project/paginated
method exists; this task adds one.

`GetExecution` usecase (`internal/usecase/get_execution.go`, full file
read directly) is the closest existing precedent to model `ListExecutions`
after: `tenant.RequireTenantID` → repo call → map
`domain.ErrExecutionNotFound`/other errors to `apperrors`.

## Changes to make

**1. `workflow.proto`**:

```protobuf
service WorkflowService {
  ...
  rpc ListExecutions(ListExecutionsRequest) returns (ListExecutionsResponse);
}

message ListExecutionsRequest {
  string project_id = 1;
  int32 limit = 2;
  string cursor = 3;
}

message ListExecutionsResponse {
  repeated WorkflowExecution executions = 1;
  string next_cursor = 2;
}
```

**2. `internal/usecase/ports.go`** — add to `ExecutionRepository`:

```go
// ListExecutions keyset-paginates tenantID/projectID's executions,
// newest first — same page_token/next-token convention as
// TemplateRepository.ListTemplates (ports.go:23-27).
ListExecutions(ctx context.Context, tenantID, projectID, cursor string, limit int32) ([]domain.WorkflowExecution, string, error)
```

**3. `internal/usecase/list_executions.go`** (new), modeled directly on
`get_execution.go`'s pattern:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

const defaultListExecutionsLimit = 20 // confirm against ListTemplates' own default, if any, for consistency

type ListExecutionsInput struct {
	ProjectID string
	Cursor    string
	Limit     int32
}

type ListExecutionsOutput struct {
	Executions []domain.WorkflowExecution
	NextCursor string
}

type ListExecutions struct {
	executions ExecutionRepository
}

func NewListExecutions(executions ExecutionRepository) *ListExecutions {
	return &ListExecutions{executions: executions}
}

func (uc *ListExecutions) Execute(ctx context.Context, in ListExecutionsInput) (ListExecutionsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListExecutionsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultListExecutionsLimit
	}
	execs, next, err := uc.executions.ListExecutions(ctx, tenantID, in.ProjectID, in.Cursor, limit)
	if err != nil {
		return ListExecutionsOutput{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_LIST_EXECUTIONS_FAILED", "failed to list workflow executions", err)
	}
	return ListExecutionsOutput{Executions: execs, NextCursor: next}, nil
}
```

**4. `internal/adapter/grpc/server.go`** — add the handler, following
`GetExecution`'s exact pattern (`:89-95`):

```go
func (s *Server) ListExecutions(ctx context.Context, req *workflowv1.ListExecutionsRequest) (*workflowv1.ListExecutionsResponse, error) {
	out, err := s.listExecutions.Execute(ctx, usecase.ListExecutionsInput{
		ProjectID: req.GetProjectId(), Cursor: req.GetCursor(), Limit: req.GetLimit(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	execs := make([]*workflowv1.WorkflowExecution, len(out.Executions))
	for i, e := range out.Executions {
		execs[i] = toProtoExecution(e) // reuse Execute/GetExecution's existing conversion helper
	}
	return &workflowv1.ListExecutionsResponse{Executions: execs, NextCursor: out.NextCursor}, nil
}
```

**5. `internal/adapter/postgres/repository.go`** — implement
`ListExecutions`, following whatever keyset-pagination SQL shape
`ListTemplates`'s real implementation already uses in this same file (do
not invent a new pagination convention).

**6. `api-gateway`'s `channels_workflow.go`** — add
`workflow.listExecutions`, following `workflow.template.list`'s exact
pattern (`:179-203`, including its "list-shaped channels return `[]` not
`null` when empty" convention):

```go
r.Register("workflow.listExecutions", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type listExecutionsArgs struct {
		ProjectID string `json:"projectId"`
		Limit     int32  `json:"limit"`
		Cursor    string `json:"cursor"`
	}
	in, err := decodeArg[listExecutionsArgs](args, 0)
	if err != nil {
		return nil, err
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	resp, err := client.ListExecutions(ctx, &workflowv1.ListExecutionsRequest{
		ProjectId: in.ProjectID, Limit: in.Limit, Cursor: in.Cursor,
	})
	if err != nil {
		return nil, err
	}
	executions := resp.GetExecutions()
	if executions == nil {
		executions = []*workflowv1.WorkflowExecution{}
	}
	return map[string]any{"executions": executions, "nextCursor": resp.GetNextCursor()}, nil
})
```

Confirm the exact field name `WorkflowMonitor.tsx` expects on the
frontend (`executions`/`nextCursor` vs. some other casing/shape) before
finalizing this channel's response map — this is a wire-contract fix for
an already-broken frontend caller, so the response shape must match what
that component is already coded to consume, not an arbitrary new
convention.

## Test plan

- `ListExecutions` returns real, paginated data for a project with
  multiple executions, newest first.
- Empty project → `executions: []` (not `null`), matching the established
  list-channel convention.
- Manual/E2E check against a real backend-go target: `WorkflowMonitor.tsx`
  actually loads without error — BE-SOL-005 explicitly notes this bug was
  invisible to unit tests before, so a manual or E2E pass is required, not
  just a unit test.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go build ./services/api-gateway/...
go test ./services/workflow-service/internal/usecase/... -run TestListExecutions -v
go test ./services/workflow-service/internal/adapter/postgres/... -run TestRepository -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestChannelsWorkflow -v
```

Expected: clean build across both services; pagination test passes;
manual verification against a running backend-go target confirms
`WorkflowMonitor.tsx` loads (this is the actual bug being fixed — a green
unit-test suite alone does not confirm the fix, per BE-SOL-005's own
note that this bug shipped past unit tests once already).
