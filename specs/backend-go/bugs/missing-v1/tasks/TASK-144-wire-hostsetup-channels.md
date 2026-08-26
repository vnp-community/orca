# TASK-144: Wire `projectHostSetup.create`/`list`/`update`/`delete`/`setupExistingFolder`

**From Solution:** SOL-022
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-131 (adds the `projectClient` param this task reuses), TASK-143 (generated `projectv1` stubs for the new RPCs)
**Status:** `[x]` DONE (verified) — implemented as `registerProjectHostSetupChannels` in `channels_tenant_project.go` (new file), called from `registerTenantProjectChannels`. `setupExistingFolder` uses the explicit 30s deadline. `go build`/`go vet`/`go test` green.

---

## Context

Unlike `project.*`/`projectGroup.*`, none of `projectHostSetup.*`'s 5
channels are wiring-only — all 5 call brand-new RPCs (TASK-143). This task
adds a new `registerProjectHostSetupChannels` function and its single call
site, reusing the same `projectv1.ProjectServiceClient` TASK-131 already
threaded into `RegisterRealChannels` as `projectClient`.

`setupExistingFolder` gets the same longer, explicit 30s deadline
`projectGroup.scanNested` (TASK-139) uses — a remote path-check-then-finalize
round-trip can exceed the standard `rpcTimeout`.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add the `registerProjectHostSetupChannels` call

Find (as left by TASK-136, or TASK-131 if TASK-136 hasn't landed):

```go
	registerProjectChannels(r, projectClient)
	registerProjectGroupChannels(r, projectClient)
}
```

Replace with:

```go
	registerProjectChannels(r, projectClient)
	registerProjectGroupChannels(r, projectClient)
	registerProjectHostSetupChannels(r, projectClient)
}
```

(If TASK-136 hasn't landed yet, add the line directly after
`registerProjectChannels(r, projectClient)` instead.)

### Step 2: Add `registerProjectHostSetupChannels` (append to end of file)

```go
// ── projectHostSetup.* ─────────────────────────────────────────────────
func registerProjectHostSetupChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("projectHostSetup.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID string `json:"devServerId"`
			FolderPath  string `json:"folderPath"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateHostSetup(rpcCtx, &projectv1.CreateHostSetupRequest{
			DevServerId: in.DevServerID, FolderPath: in.FolderPath, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetSetup(), nil
	})

	r.Register("projectHostSetup.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListHostSetups(rpcCtx, &projectv1.ListHostSetupsRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetSetups(), nil
	})

	r.Register("projectHostSetup.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID          string `json:"id"`
			FolderPath  string `json:"folderPath"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateHostSetup(rpcCtx, &projectv1.UpdateHostSetupRequest{
			Id: in.ID, FolderPath: in.FolderPath, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetSetup(), nil
	})

	r.Register("projectHostSetup.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.DeleteHostSetup(rpcCtx, &projectv1.DeleteHostSetupRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("projectHostSetup.setupExistingFolder", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type setupArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[setupArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		// Same reasoning as projectGroup.scanNested (TASK-139) — a remote
		// path-check-then-finalize round-trip can exceed rpcTimeout.
		rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		resp, err := client.SetupExistingFolder(rpcCtx, &projectv1.SetupExistingFolderRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build. All 5 `projectHostSetup.*` channels now resolve
through `wscompat.Registry`.
