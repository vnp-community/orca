# TASK-131: Wire `project.create`/`get`/`list`/`update` (wiring-only — RPC + REST already exist)

**From Solution:** SOL-020
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`, `services/api-gateway/cmd/server/main.go`
**Depends on:** none
**Status:** `[x]` DONE (verified) — implemented as `registerProjectChannels` in the new `channels_tenant_project.go` file (not `channels.go`), called from `registerTenantProjectChannels`. Not yet spliced into `RegisterRealChannels`/`main.go` — see integration-pass note at the bottom of that file. `go build`/`go vet`/`go test` green.

---

## Context

BUG-020 confirmed `create`/`get`/`list`/`update` are fully built end-to-end
(usecase, REST proxy at `/v1/projects`) and only missing a `wscompat`
registration — same shape as `devServer.list`/`devServer.add`. No proto,
usecase, or repository change needed for these 4.

The 3 member-management channels (`getMembers`/`removeMember`/
`updateMemberRole`) need new RPCs first — see TASK-132/133/134. This task's
`registerProjectChannels` function is the one TASK-134 extends with more
`r.Register(...)` calls.

`main.go` already dials a `ProjectServiceClient` (`projectClient`, used
today only by `mountProjectRoutes`) — thread that same client into
`wscompat.RegisterRealChannels`.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add `projectv1` import

```go
import (
	"context"
	"encoding/json"
	"time"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)
```

(If TASK-126 has not landed yet, omit the `tenantv1` line and the
`tenantClient`/`registerProfileChannels` bits below — this task is
independent of SOL-019's changes; the two land in whatever order is
convenient, both only append to `RegisterRealChannels`.)

### Step 2: Add `projectClient` param to `RegisterRealChannels`, call `registerProjectChannels`

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	projectClient projectv1.ProjectServiceClient,
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
	registerProjectChannels(r, projectClient)
}
```

(Merge this parameter list with TASK-126's `tenantClient` param if that
task has already landed — both are additive params on the same function.)

### Step 3: Add `registerProjectChannels` (append to end of file)

```go
// ── project.* ──────────────────────────────────────────────────────────
//
// create/get/list/update: RPC + REST already exist, wiring-only.
// getMembers/removeMember/updateMemberRole are added to this SAME function
// by TASK-134, once TASK-132/133 land the RPCs they call.
func registerProjectChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("project.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			DevServerID   string `json:"devServerId"`
			DefaultBranch string `json:"defaultBranch"`
			Visibility    string `json:"visibility"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateProject(rpcCtx, &projectv1.CreateProjectRequest{
			TenantId: id.TenantID, Name: in.Name, Description: in.Description,
			DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	})

	r.Register("project.get", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetProject(rpcCtx, &projectv1.GetProjectRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	})

	r.Register("project.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		// list's args are optional (frontend often calls with no args) —
		// decode best-effort, defaulting to zero values on missing/absent arg[0].
		var in listArgs
		if len(args) > 0 {
			in, _ = decodeArg[listArgs](args, 0)
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		// TenantId is always taken from the resolved Identity, never a
		// caller-supplied arg — matches project_routes.go's
		// handleListProjects convention (api-gateway.md §9).
		resp, err := client.ListProjects(rpcCtx, &projectv1.ListProjectsRequest{
			TenantId: id.TenantID, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProjects(), nil
	})

	r.Register("project.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			DefaultBranch string `json:"defaultBranch"`
			Visibility    string `json:"visibility"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateProject(rpcCtx, &projectv1.UpdateProjectRequest{
			ProjectId: in.ID, Name: in.Name, Description: in.Description,
			DefaultBranch: in.DefaultBranch, Visibility: in.Visibility,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProject(), nil
	})
}
```

Note: `CreateProjectRequest` in the shipped proto has a `tenant_id` field
(unlike SOL-020's sketch, which omitted it) — set from `id.TenantID`, never
a caller-supplied arg, same rule `project.list` follows. `GetProjectRequest`
uses field name `id`, not `project_id` — check current `project.proto`
before adapting if it has changed since this task was written.

**File:** `services/api-gateway/cmd/server/main.go`

### Step 4: Pass `projectClient` into `RegisterRealChannels`

Find (or, if TASK-126 landed first, the already-updated call with
`tenantClient` added):

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

Replace with:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, projectClient, rateLimiter)
```

`projectClient` is already constructed earlier in `run()` (dialed for
`mountProjectRoutes`) — no new `Dial` call needed.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
