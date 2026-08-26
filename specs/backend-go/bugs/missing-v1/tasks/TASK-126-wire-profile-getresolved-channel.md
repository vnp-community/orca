# TASK-126: Wire `profile.getResolved` (wiring-only — RPC + REST already exist)

**From Solution:** SOL-019
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`, `services/api-gateway/cmd/server/main.go`
**Depends on:** none
**Status:** `[x]` DONE (verified) — implemented in a NEW file `channels_tenant_project.go` (not `channels.go`, per repo-wide parallel-worktree convention) as `registerProfileChannels`, called from `registerTenantProjectChannels`. Not yet spliced into `RegisterRealChannels`/`main.go` — see that task's own note for the exact one-line integration-pass wiring. `go build`/`go vet`/`go test` all green in api-gateway.

---

## Context

`profile.getResolved` is the one BUG-019 channel that needs no service-side
work — `TenantService.GetResolvedProfile` and its REST proxy
(`tenant_routes.go`'s `handleGetResolvedProfile`) both already exist. This
task only registers it in `wscompat`, following `registerDevServerChannels`'s
exact pattern (`channels.go:390-433`).

The other 5 `profile.*` channels (`getUserProfile`/`listDepts`/
`updateCompany`/`updateDept`/`updateUser`) need new RPCs first — see
TASK-127/128/129. This task's `registerProfileChannels` function is the one
those later tasks extend with more `r.Register(...)` calls; do not create a
second function.

`main.go` already dials a `TenantServiceClient` (`tenantClient`, used today
only by `mountTenantRoutes`) — this task threads that same client into
`wscompat.RegisterRealChannels`, not a new dial.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add `tenantv1` import

```go
import (
	"context"
	"encoding/json"
	"time"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)
```

### Step 2: Add `tenantClient` param to `RegisterRealChannels`

Find:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
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
}
```

Replace with:

```go
func RegisterRealChannels(
	r *Registry,
	annotationClient annotationv1.AnnotationServiceClient,
	taskClient taskv1.TaskServiceClient,
	gitClient gitgatewayv1.GitGatewayServiceClient,
	automationClient automationv1.AutomationServiceClient,
	infraFleetClient infrafleetv1.InfraFleetServiceClient,
	tenantClient tenantv1.TenantServiceClient,
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
	registerProfileChannels(r, tenantClient)
}
```

### Step 3: Add `registerProfileChannels` (append to end of file)

```go
// ── profile.* ──────────────────────────────────────────────────────────
//
// profile.getResolved: RPC + REST already exist (tenant_routes.go's
// handleGetResolvedProfile) — wiring-only. The other 5 profile.* channels
// (getUserProfile/listDepts/updateCompany/updateDept/updateUser) are added
// to this SAME function by TASK-129, once TASK-127/128 land the RPCs they
// call.
func registerProfileChannels(r *Registry, client tenantv1.TenantServiceClient) {
	r.Register("profile.getResolved", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetResolvedProfile(rpcCtx, &tenantv1.GetResolvedProfileRequest{UserId: id.UserID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

**File:** `services/api-gateway/cmd/server/main.go`

### Step 4: Pass `tenantClient` into `RegisterRealChannels`

Find:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

Replace with:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, tenantClient, rateLimiter)
```

`tenantClient` is already constructed earlier in `run()` (dialed for
`mountTenantRoutes`) — no new `Dial` call needed.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build. `profile.getResolved` now resolves through
`wscompat.Registry` instead of `notImplementedHandler`.
