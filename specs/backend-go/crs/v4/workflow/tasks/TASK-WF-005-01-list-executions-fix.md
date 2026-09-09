# TASK-WF-005-01: `ListExecutions` — fix `WorkflowMonitor.tsx`'s broken-in-production dependency

**From Solution:** BE-SOL-005
**Priority:** P0 — higher than every other BE-SOL-005 task. `WorkflowMonitor.tsx` (the one workflow component actually mounted in the app, per `WorkspaceLayout.tsx`) calls `workflow.listExecutions`, which does not exist on backend-go today — the Workflow tab fails to load for every user on a backend-go target right now. This is a live production bug fix, not new-feature scope, and should ship ahead of BE-SOL-005's sharing/library RPCs (TASK-WF-005-02/-03), which are larger and need a security review pass.
**Service:** `workflow-service` + `api-gateway`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`ListExecutions` RPC), `backend-go/services/workflow-service/internal/usecase/ports.go` (`ExecutionRepository.ListExecutions`), `backend-go/services/workflow-service/internal/usecase/list_executions.go` (new), `backend-go/services/workflow-service/internal/adapter/grpc/server.go`, `backend-go/services/workflow-service/internal/adapter/postgres/repository.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_workflow.go`
**Depends on:** None
**Status:** `[ ]` TODO

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
