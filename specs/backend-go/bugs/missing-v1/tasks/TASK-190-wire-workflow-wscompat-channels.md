# TASK-190: Wire `workflow.execute`/`.cancel`/`.template.create`/`.template.update` `wscompat` channels

**From Solution:** SOL-030
**Priority:** P1
**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_workflow.go` (new), `internal/adapter/wscompat/channels.go`, `cmd/server/main.go`
**Depends on:** TASK-189
**Status:** `[x]` DONE — implemented in worktree `agent-aa8bd8599a599323a` (team/terminal/workflow/worktree pass, merged into `integration/missing-v1` as commit `baa34819a`); this task doc's own Status line was never updated by that implementing pass (a task-doc-capture gap, not a missing-code gap) — verified against the current merged code+tests during a later re-audit: build/vet/test clean.

---

## Context

`api-gateway`'s `cmd/server/main.go` already dials `workflow-service`
(`workflowClient`, used by `httpgateway.NewRouter`'s REST routes at
`/v1/workflows/*`) — `RegisterRealChannels` just needs it added as a new
parameter, same shape as TASK-178 added `tenantClient`. All four handlers
use `AttachIdentity` since `Execute`/`CancelExecution`/`CreateTemplate`/
`UpdateTemplate` bind `tenant_id` from gRPC metadata, not a request field
(except `CreateTemplateRequest.tenant_id`, which the REST handler sets
explicitly via `identity.TenantID` — the gRPC metadata path via
`AttachIdentity` is the one every channel in this file uses, so set both
for consistency, matching this file's existing devServer.*/fleet.*
precedent).

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_workflow.go`

```go
// ── workflow.* (workflow-service) ────────────────────────────────────────
package wscompat

import (
	"context"
	"encoding/json"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerWorkflowChannels(r *Registry, client workflowv1.WorkflowServiceClient) {
	r.Register("workflow.execute", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type executeArgs struct {
			TemplateID  string `json:"templateId"`
			ProjectID   string `json:"projectId"`
			RootTraceID string `json:"rootTraceId"`
			RequestID   string `json:"requestId"`
		}
		in, err := decodeArg[executeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.Execute(ctx, &workflowv1.ExecuteRequest{
			TemplateId: in.TemplateID, ProjectId: in.ProjectID,
			RootTraceId: in.RootTraceID, RequestId: in.RequestID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.cancel", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type cancelArgs struct {
			ExecutionID string `json:"executionId"`
		}
		in, err := decodeArg[cancelArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.CancelExecution(ctx, &workflowv1.CancelExecutionRequest{Id: in.ExecutionID})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.template.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name             string `json:"name"`
			DAGJSON          string `json:"dagJson"`
			Scope            string `json:"scope"`
			ParentTemplateID string `json:"parentTemplateId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.CreateTemplate(ctx, &workflowv1.CreateTemplateRequest{
			TenantId: id.TenantID, Name: in.Name, DagJson: in.DAGJSON,
			Scope: in.Scope, ParentTemplateId: in.ParentTemplateID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTemplate(), nil
	})

	r.Register("workflow.template.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			DAGJSON          string `json:"dagJson"`
			Scope            string `json:"scope"`
			ParentTemplateID string `json:"parentTemplateId"`
			ExpectedVersion  int32  `json:"expectedVersion"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.UpdateTemplate(ctx, &workflowv1.UpdateTemplateRequest{
			Id: in.ID, Name: in.Name, DagJson: in.DAGJSON, Scope: in.Scope,
			ParentTemplateId: in.ParentTemplateID, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTemplate(), nil
	})
}
```

### `channels.go`: grow `RegisterRealChannels`

Add `workflowClient workflowv1.WorkflowServiceClient` as a new parameter
and add the `registerWorkflowChannels(r, workflowClient)` call, following
the same pattern TASK-178 used for `tenantClient`. Add the `workflowv1`
import to `channels.go`'s import block:

```go
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
```

### `cmd/server/main.go`: pass `workflowClient` through

`workflowClient` is already dialed (`workflowConn`/`workflowClient` block).
Update the `wscompat.RegisterRealChannels(...)` call site to append
`workflowClient` as the final argument, matching Step above's parameter
order.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
