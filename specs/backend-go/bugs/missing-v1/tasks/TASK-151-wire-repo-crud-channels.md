# TASK-151: Wire `repo.add`/`repo.list`/`repo.reorder`/`repo.rm` to `project-service` (Bucket 1 — pure wiring)

**From Solution:** SOL-023 (Bucket 1)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`, `services/api-gateway/cmd/server/main.go`
**Depends on:** none
**Status:** `[partial]` `registerRepoChannels` implemented (repo.add/list/reorder/rm) in a NEW file `channels_repo_ssh_status_workspace.go` (not `channels.go`, per instructions — the other group is editing that file). Builds/tests green in isolation. NOT wired into `RegisterRealChannels`/`main.go` — that requires editing `channels.go`, out of scope here. Integration pass needs one line: `registerRepoSshStatusWorkspaceChannels(r, projectClient, gitClient, infraFleetClient)` inside `RegisterRealChannels`, plus a `projectClient` param on that function.

---

## Context

`ProjectService.AddRepo`/`ListRepos`/`ReorderRepos`/`RemoveRepo` already
exist (`backend-go/proto/orca/project/v1/project.proto:26-29,136-173`) with
a matching `Repo{id, project_id, url, display_name, position}` message.
`main.go:168` already dials `project-service` and holds `projectClient`,
but `RegisterRealChannels` (`channels.go:64-73`) is never given it, so
`repo.add`/`repo.list`/`repo.reorder`/`repo.rm` fall through to
`notImplementedHandler`. This task is wiring only — no new RPC, no new
usecase.

`repo.update` (Bucket 2, needs a new `UpdateRepo` RPC) and the 8
git-gateway-service-owned methods (Bucket 3: `repo.clone`,
`repo.baseRefDefault`, `repo.searchRefs`, `repo.create`,
`repo.hooksCheck`, `repo.issueCommandRead`, `repo.issueCommandWrite`,
`repo.setupScriptImports`) are **not** in scope here — see TASK-152
through TASK-155 (project-service group) and TASK-156 through TASK-161
(git-gateway-service group).

---

## Changes to make

### `channels.go` — add `projectClient` param to `RegisterRealChannels`

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
	projectClient projectv1.ProjectServiceClient, // NEW
	rateLimits rateLimitReader,
) {
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerRepoChannels(r, projectClient, gitClient) // NEW
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

### `channels.go` — add the `projectv1` import

In the import block:

```go
import (
	"context"
	"encoding/json"
	"time"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1" // NEW
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)
```

### `channels.go` — new `registerRepoChannels`, appended near `registerGitChannels`

```go
// ── repo.* ───────────────────────────────────────────────────────────────
//
// repo.add/list/reorder/rm map 1:1 onto ProjectService's existing
// AddRepo/ListRepos/ReorderRepos/RemoveRepo — pure catalog CRUD against the
// repos Postgres table, per project-service.md §4's "Repo — pure metadata,
// no working-tree state." repo.update (TASK-154) and the 8
// git-gateway-service-owned methods (TASK-160) join this function once
// their respective RPCs exist — see SOL-023 for the full bucket split.
func registerRepoChannels(r *Registry, project projectv1.ProjectServiceClient, git gitgatewayv1.GitGatewayServiceClient) {
	r.Register("repo.add", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type addArgs struct {
			ProjectID   string `json:"projectId"`
			URL         string `json:"url"`
			DisplayName string `json:"displayName"`
		}
		in, err := decodeArg[addArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := project.AddRepo(rpcCtx, &projectv1.AddRepoRequest{
			ProjectId: in.ProjectID, Url: in.URL, DisplayName: in.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetRepo(), nil
	})

	r.Register("repo.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := project.ListRepos(rpcCtx, &projectv1.ListReposRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		return resp.GetRepos(), nil
	})

	r.Register("repo.reorder", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type reorderArgs struct {
			ProjectID      string   `json:"projectId"`
			RepoIDsInOrder []string `json:"repoIdsInOrder"`
		}
		in, err := decodeArg[reorderArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err = project.ReorderRepos(rpcCtx, &projectv1.ReorderReposRequest{
			ProjectId: in.ProjectID, RepoIdsInOrder: in.RepoIDsInOrder,
		})
		return nil, err
	})

	r.Register("repo.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			RepoID string `json:"repoId"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		// NOTE: RemoveRepo does NOT detach worktrees first —
		// project-service.md §3's comment on RemoveRepoRequest ("caller
		// must detach worktrees via git-gateway-service first") means this
		// handler, or the frontend action that calls it, is responsible
		// for that ordering. Flag as a follow-up if the frontend doesn't
		// already enforce it.
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err = project.RemoveRepo(rpcCtx, &projectv1.RemoveRepoRequest{RepoId: in.RepoID})
		return nil, err
	})

	// repo.update joins here once TASK-153's UpdateRepo RPC exists (TASK-154).
	// repo.baseRefDefault / repo.clone / repo.searchRefs / repo.create /
	// repo.hooksCheck / repo.issueCommandRead / repo.issueCommandWrite /
	// repo.setupScriptImports join here, against `git`, once TASK-159's
	// RPCs exist (TASK-160). `git` is accepted as a parameter now so this
	// function's signature does not change again later.
	_ = git
}
```

### `main.go` — pass `projectClient` at the call site

Find (`main.go:241`):

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

Replace with:

```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, projectClient, rateLimiter)
```

`projectClient` is already declared at `main.go:168` — no new dial needed.

---

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/... && go vet ./services/api-gateway/...
```

Expected: clean build. `git` parameter in `registerRepoChannels` is
unused beyond the `_ = git` placeholder until TASK-160 — that line keeps
`go vet`/the compiler quiet without wiring Bucket 3 prematurely; remove it
in TASK-160 once real handlers use `git`.
