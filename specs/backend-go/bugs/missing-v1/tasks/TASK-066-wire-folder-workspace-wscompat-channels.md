# TASK-066: Wire `folderWorkspace.*` wscompat channels

**From Solution:** SOL-010 (Design — `wscompat` wiring)
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-065
**Status:** `[x]` DONE — implemented as `registerFolderWorkspaceChannels` in `channels_emulator_folderworkspace_host.go` (new file, per this pass's cross-group convention). Worktree `agent-abbc42cb9786d6743`, commit `a329ce7d9`. **Integration note:** needs `projectClient projectv1.ProjectServiceClient` threaded into `RegisterRealChannels`/`main.go` — see the group's final report for the exact one-line change.

---

## Context

Straightforward CRUD dispatch, mirrors `registerAnnotationChannels`'s
existing four-method shape (this namespace is almost the same size — 5
methods vs. annotation's 4). `project-service`'s existing RPCs take
`tenant_id` via the inbound ctx the same way `git.status`/`git.diff` do
today — **verify this against `project-service`'s actual interceptor
behavior before wiring**; if it turns out to require explicit metadata
like `infra-fleet-service` does, add the same `gatewaygrpc.AttachIdentity`
call `registerDevServerChannels` uses (see channels.go:390-432 for that
pattern).

---

## Changes to make

**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`

### Step 1: Add `registerFolderWorkspaceChannels`

Add near `registerAnnotationChannels` (or at the end of the file, either
placement compiles — colocating with `registerAnnotationChannels` matches
this file's grouping-by-similar-shape convention more closely):

```go
// ── folderWorkspace.* ────────────────────────────────────────────────

func registerFolderWorkspaceChannels(r *Registry, client projectv1.ProjectServiceClient) {
	r.Register("folderWorkspace.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID string `json:"devServerId"`
			Path        string `json:"path"`
			Name        string `json:"name"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CreateFolderWorkspace(ctx, &projectv1.CreateFolderWorkspaceRequest{
			DevServerId: in.DevServerID, Path: in.Path, Name: in.Name,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("folderWorkspace.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.UpdateFolderWorkspace(ctx, &projectv1.UpdateFolderWorkspaceRequest{Id: in.ID, Name: in.Name})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("folderWorkspace.delete", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if _, err := client.DeleteFolderWorkspace(ctx, &projectv1.DeleteFolderWorkspaceRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("folderWorkspace.list", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		resp, err := client.ListFolderWorkspaces(ctx, &projectv1.ListFolderWorkspacesRequest{})
		if err != nil {
			return nil, err
		}
		return resp.GetFolderWorkspaces(), nil
	})

	r.Register("folderWorkspace.getPathStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type statusArgs struct {
			DevServerID string `json:"devServerId"`
			Path        string `json:"path"`
		}
		in, err := decodeArg[statusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetFolderWorkspacePathStatus(ctx, &projectv1.GetFolderWorkspacePathStatusRequest{
			DevServerId: in.DevServerID, Path: in.Path,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

None of these calls set an `rpcTimeout` deadline explicitly, matching
`registerAnnotationChannels`'s existing convention for `project-service`
calls (unlike `registerDevServerChannels`'s explicit
`context.WithTimeout(ctx, rpcTimeout)` for `infra-fleet-service` calls) —
if `project-service` turns out to need the same per-RPC deadline pattern,
apply it uniformly across all 5 handlers above, not selectively.

### Step 2: Wire into `RegisterRealChannels`

`RegisterRealChannels` does not currently take a `projectv1.ProjectServiceClient`
parameter (it's only used today by `api-gateway`'s REST routes, not
wscompat). Add it:

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
	registerFolderWorkspaceChannels(r, projectClient)
	// registerEmulatorChannels(r) / registerFilesChannels(r, gitClient) —
	// keep alongside if TASK-046/TASK-058 already landed.
}
```

### Step 3: Update the call site in `cmd/server/main.go`

Find `wscompat.RegisterRealChannels(...)`'s call and add the already-
constructed `projectClient` (used today for `/v1/projects` REST routes —
`NewProjectServiceClient` is already dialed in this file) as the new
argument, in the position matching Step 2's signature.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./...
go vet ./...
```

Expected: clean build across `api-gateway`, including `cmd/server/main.go`'s
updated call site.
