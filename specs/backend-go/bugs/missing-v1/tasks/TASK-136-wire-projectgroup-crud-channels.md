# TASK-136: Wire `projectGroup.create`/`update`/`delete`/`list` (wiring-only — RPC + REST already exist)

**From Solution:** SOL-021
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-131 (adds the `projectClient` param this task reuses)
**Status:** `[x]` DONE (verified) — implemented as `registerProjectGroupChannels` in `channels_tenant_project.go` (new file), called from `registerTenantProjectChannels`. `go build`/`go vet`/`go test` green.

---

## Context

BUG-021 confirmed `create`/`update`/`delete`/`list` are fully built
end-to-end (usecase, REST proxy at `/v1/project-groups`) and only missing a
`wscompat` registration — identical shape to TASK-131's `project.*`
wiring-only channels. No proto/usecase/repository change needed.

The 3 new channels (`moveProject`/`scanNested`/`importNested`) need new RPCs
first — see TASK-137/138/139. This task's `registerProjectGroupChannels`
function is the one TASK-139 extends.

Reuses the same `projectv1.ProjectServiceClient` TASK-131 already threaded
into `RegisterRealChannels` as `projectClient` — no new client param needed
here, just one more registration call inside `RegisterRealChannels`'s body.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add the `registerProjectGroupChannels` call

Find (as left by TASK-131):

```go
	registerRateLimitChannels(r, rateLimits)
	registerProjectChannels(r, projectClient)
}
```

Replace with:

```go
	registerRateLimitChannels(r, rateLimits)
	registerProjectChannels(r, projectClient)
	registerProjectGroupChannels(r, projectClient)
}
```

(If TASK-126's `registerProfileChannels(r, tenantClient)` call has already
landed between these two lines, keep it — just add the
`registerProjectGroupChannels` line after `registerProjectChannels`.)

### Step 2: Add `registerProjectGroupChannels` (append to end of file)

```go
// ── projectGroup.* ─────────────────────────────────────────────────────
//
// create/update/delete/list: RPC + REST already exist, wiring-only.
// moveProject/scanNested/importNested are added to this SAME function by
// TASK-139, once TASK-137/138 land the RPCs they call.
func registerProjectGroupChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("projectGroup.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name          string `json:"name"`
			ParentGroupID string `json:"parentGroupId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateProjectGroup(rpcCtx, &projectv1.CreateProjectGroupRequest{
			Name: in.Name, ParentGroupId: in.ParentGroupID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetGroup(), nil
	})

	r.Register("projectGroup.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			GroupID string `json:"groupId"`
			Name    string `json:"name"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateProjectGroup(rpcCtx, &projectv1.UpdateProjectGroupRequest{
			GroupId: in.GroupID, Name: in.Name,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetGroup(), nil
	})

	r.Register("projectGroup.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			GroupID string `json:"groupId"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.DeleteProjectGroup(rpcCtx, &projectv1.DeleteProjectGroupRequest{GroupId: in.GroupID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("projectGroup.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListProjectGroups(rpcCtx, &projectv1.ListProjectGroupsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetGroups(), nil
	})
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
