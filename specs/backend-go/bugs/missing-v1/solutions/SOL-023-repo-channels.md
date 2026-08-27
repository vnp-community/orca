# SOL-023: Wire `repo.*` — 4 methods to `project-service` (design already exists), split the remaining 9 between `project-service` and `git-gateway-service`

**Resolves:** [BUG-023](../BUG-023-repo-channels-not-implemented.md)
**Service:** `project-service` (catalog CRUD) + `git-gateway-service` (git-shaped ops) + `api-gateway` (wiring)
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
- `backend-go/services/api-gateway/cmd/server/main.go` (signature change to `RegisterRealChannels`)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto`
- `backend-go/proto/orca/project/v1/project.proto`
- `backend-go/services/git-gateway-service/internal/usecase/*.go` (new use cases)
- `backend-go/services/project-service/internal/usecase/update_repo.go` (new)
- `backend-go/services/project-service/internal/adapter/grpc/server.go`
**Status:** ✅ Implemented — all 11 task(s) (TASK-151–161) DONE; see each task file's own Status/Verify section for evidence.

---

## Two different problems under one namespace — don't force one fix

BUG-023 already separated `repo.*`'s 13 methods into three buckets. This
solution keeps that separation instead of collapsing it into one owner:

1. **4 methods, design already exists, pure wiring gap** — `repo.add` /
   `repo.list` / `repo.reorder` / `repo.rm` map 1:1 onto
   `ProjectService.AddRepo` / `ListRepos` / `ReorderRepos` / `RemoveRepo`,
   already defined at `project.proto:26-29,144-173` with a matching `Repo`
   message. Nothing new to design — `wscompat` just needs to call them, and
   `main.go` needs to hand `projectClient` to `RegisterRealChannels`.
2. **1 method, catalog metadata, needs a new RPC on `project-service`** —
   `repo.update`. `Repo{url, display_name, position}` (`project.proto:136-141`)
   is a plain Postgres row with no host/connection field, per
   `project-service.md` §4's "Repo — ... Pure metadata, no working-tree
   state." An edit to `display_name`/`url` is exactly the shape of
   `UpdateProject`'s field-mask pattern (`project.proto:22`), not a git
   operation — it belongs next to `AddRepo`/`RemoveRepo`, not on
   `git-gateway-service`.
3. **8 methods, no RPC anywhere, git-working-tree-shaped, belong on
   `git-gateway-service`** — `repo.clone`, `repo.baseRefDefault`,
   `repo.searchRefs` are explicitly called out by BUG-023 as clearly
   git-shaped. The remaining 5 (`repo.create`, `repo.hooksCheck`,
   `repo.issueCommandRead`, `repo.issueCommandWrite`,
   `repo.setupScriptImports`) are less obvious at a glance but land the
   same place on inspection: every one of them reads or writes files inside
   a repo's working tree (git hooks, `.orca`/setup-script imports, an
   issue-command config file) or runs `git init` against a host path
   (`repo.create` — see below) — none of them touch the `repos` Postgres
   table's `url`/`display_name`/`position` columns. `git-gateway-service`'s
   own charter is exactly this: "resolve which host owns the [repo], send
   the operation there" (`git-gateway-service.md` §1) for anything that
   touches a working tree, whether or not it's git plumbing in the strict
   sense.

None of these 8 RPCs are in `git-gateway-service.md`'s own §3 API-surface
sketch either — this is a genuine scope addition beyond the TDD, not a gap
in an RPC the TDD already specified (unlike buckets 1-2 above). Flagged
here explicitly, same posture as SOL-001's `GetAdminStats` addition.

---

## Bucket 1 — Design: wiring (`repo.add`/`list`/`reorder`/`rm`)

`RegisterRealChannels` (`channels.go:64-73`) is called with
`(annotationClient, taskClient, gitClient, automationClient,
infraFleetClient, rateLimiter)` — no `projectClient`, even though
`main.go:168` already dials `project-service` and holds
`projectClient := projectv1.NewProjectServiceClient(projectConn)` for the
REST layer (`main.go:270`'s `ProjectClient: projectClient`). Add it as a
new parameter:

```go
// channels.go
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

```go
// main.go:241 — add projectClient to the call
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, projectClient, rateLimiter)
```

New `registerRepoChannels`, following `registerGitChannels`'s exact shape
(`channels.go:221-252`) — decode args, call the client, return the
response verbatim where the shape already matches:

```go
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
        // NOTE: RemoveRepo does NOT detach worktrees first — project-service.md
        // §3's comment on RemoveRepoRequest ("caller must detach worktrees via
        // git-gateway-service first") means this handler, or the frontend
        // action that calls it, is responsible for that ordering. Flag as a
        // follow-up if the frontend doesn't already enforce it.
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
        defer cancel()
        _, err = project.RemoveRepo(rpcCtx, &projectv1.RemoveRepoRequest{RepoId: in.RepoID})
        return nil, err
    })

    // repo.baseRefDefault / repo.clone / repo.searchRefs / repo.create /
    // repo.hooksCheck / repo.issueCommand{Read,Write} /
    // repo.setupScriptImports registered below, against `git`, once
    // Bucket 3's RPCs exist.
}
```

---

## Bucket 2 — Design: `repo.update` on `project-service`

Add a field-masked update, mirroring `UpdateProject`'s own convention
(`project.proto:22`'s "empty string means no change" rule):

```protobuf
// project.proto — next to AddRepo/RemoveRepo
rpc UpdateRepo(UpdateRepoRequest) returns (UpdateRepoResponse);

message UpdateRepoRequest {
  string repo_id = 1;
  string url = 2;          // empty = no change
  string display_name = 3; // empty = no change
}

message UpdateRepoResponse {
  Repo repo = 1;
}
```

```go
// internal/usecase/update_repo.go
type UpdateRepoInput struct {
    RepoID      string
    URL         string
    DisplayName string
}

func (uc *UpdateRepo) Execute(ctx context.Context, in UpdateRepoInput) (domain.Repo, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil {
        return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
    }
    repo, err := uc.repos.Get(ctx, tenantID, in.RepoID)
    if err != nil {
        return domain.Repo{}, err
    }
    if in.URL != "" {
        repo.URL = in.URL
    }
    if in.DisplayName != "" {
        repo.DisplayName = in.DisplayName
    }
    return uc.repos.Update(ctx, repo)
}
```

`wscompat` wiring for `repo.update` joins the block above once
`UpdateRepo` exists:

```go
r.Register("repo.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type updateArgs struct {
        RepoID      string `json:"repoId"`
        URL         string `json:"url"`
        DisplayName string `json:"displayName"`
    }
    in, err := decodeArg[updateArgs](args, 0)
    if err != nil {
        return nil, err
    }
    ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
    rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
    defer cancel()
    resp, err := project.UpdateRepo(rpcCtx, &projectv1.UpdateRepoRequest{
        RepoId: in.RepoID, Url: in.URL, DisplayName: in.DisplayName,
    })
    if err != nil {
        return nil, err
    }
    return resp.GetRepo(), nil
})
```

---

## Bucket 3 — Design: 8 new RPCs on `git-gateway-service`

Follows `git-gateway-service.md` §2's resolve→dispatch→translate model
exactly — every request carries a worktree- or repo-scoped path/id, never
a raw client-supplied filesystem path (`git-gateway-service.md` §3's
"never trust a client-supplied host path" rule), and every op resolves the
owning host via `infra-fleet-service.ResolveConnection` before deciding
local-exec vs. relay.

```protobuf
// gitgateway.proto additions — scope addition beyond the TDD's §3 sketch,
// same resolve-dispatch model as GetStatus/GetDiff.
service GitGatewayService {
  // ... existing 6 RPCs ...

  rpc Clone(CloneRequest) returns (CloneResponse);
  rpc BaseRefDefault(BaseRefDefaultRequest) returns (BaseRefDefaultResponse);
  rpc SearchRefs(SearchRefsRequest) returns (SearchRefsResponse);
  rpc InitRepo(InitRepoRequest) returns (InitRepoResponse);          // repo.create
  rpc CheckHooks(CheckHooksRequest) returns (CheckHooksResponse);     // repo.hooksCheck
  rpc ReadIssueCommand(ReadIssueCommandRequest) returns (ReadIssueCommandResponse);
  rpc WriteIssueCommand(WriteIssueCommandRequest) returns (google.protobuf.Empty);
  rpc ScanSetupScriptImports(ScanSetupScriptImportsRequest) returns (ScanSetupScriptImportsResponse);
}

message CloneRequest {
  string dev_server_id = 1; // which host to clone onto — resolved via infra-fleet-service by the caller (project-service context) before this call
  string url = 2;
  string dest_path = 3;
}
message CloneResponse {
  string worktree_path = 1;
  string default_branch = 2;
}

message BaseRefDefaultRequest { string worktree_id = 1; }
message BaseRefDefaultResponse { string ref = 1; }

message SearchRefsRequest { string worktree_id = 1; string query = 2; }
message SearchRefsResponse { repeated string refs = 1; }

// InitRepo runs `git init` at dest_path on the resolved host and returns
// enough for the caller to then call ProjectService.AddRepo — mirrors
// project-service.md §2's "git-gateway-service does the git op, then
// writes back metadata" saga already established for worktrees
// (RecordWorktreeCreated), applied here to repo creation instead.
message InitRepoRequest {
  string dev_server_id = 1;
  string dest_path = 2;
  string default_branch = 3; // empty = git's own default
}
message InitRepoResponse {
  string path = 1;
  string default_branch = 2;
}

message CheckHooksRequest { string worktree_id = 1; }
message CheckHooksResponse { repeated string installed_hooks = 1; bool orca_hooks_current = 2; }

message ReadIssueCommandRequest { string worktree_id = 1; }
message ReadIssueCommandResponse { string content = 1; bool exists = 2; }

message WriteIssueCommandRequest { string worktree_id = 1; string content = 2; }

message ScanSetupScriptImportsRequest { string worktree_id = 1; }
message ScanSetupScriptImportsResponse { repeated string imported_paths = 1; }
```

Usecase layer — one file per RPC, same shape as `commit.go`/`push.go`
(resolve host via `infra-fleet-service`'s client, then either exec `git`
locally or relay through the Dev Server Agent):

```go
// internal/usecase/clone.go
type Clone struct {
    infraFleet InfraFleetClient // resolves dev-server reachability
    local      LocalGitExecutor
    relay      DevServerAgentClient
}

func (uc *Clone) Execute(ctx context.Context, in CloneInput) (CloneResult, error) {
    resolved, err := uc.infraFleet.ResolveDevServer(ctx, in.DevServerID)
    if err != nil {
        return CloneResult{}, err
    }
    if resolved.Connected {
        result, err := uc.relay.Exec(ctx, resolved.DevServer, "git.clone", map[string]any{
            "url": in.URL, "destPath": in.DestPath,
        })
        return decodeCloneResult(result), err
    }
    return uc.local.Clone(ctx, in.URL, in.DestPath)
}
```

`InitRepo`, `CheckHooks`, `ReadIssueCommand`, `WriteIssueCommand`,
`ScanSetupScriptImports`, `BaseRefDefault`, `SearchRefs` all follow the
identical resolve→(local exec | relay `git.*`/`hooks.*` agent method)
shape — omitted here for brevity, same pattern repeated per operation.

`wscompat` wiring (added to `registerRepoChannels` from Bucket 1, once
these RPCs exist):

```go
r.Register("repo.clone", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
    type cloneArgs struct {
        DevServerID string `json:"devServerId"`
        URL         string `json:"url"`
        DestPath    string `json:"destPath"`
    }
    in, err := decodeArg[cloneArgs](args, 0)
    if err != nil {
        return nil, err
    }
    ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
    resp, err := git.Clone(ctx, &gitgatewayv1.CloneRequest{
        DevServerId: in.DevServerID, Url: in.URL, DestPath: in.DestPath,
    })
    if err != nil {
        return nil, err
    }
    return resp, nil
})
// repo.baseRefDefault / repo.searchRefs / repo.create / repo.hooksCheck /
// repo.issueCommandRead / repo.issueCommandWrite /
// repo.setupScriptImports follow the same decode-call-return shape.
```

`repo.clone`'s deadline needs an override above `rpcTimeout` (8s) — cloning
a real repo, especially relayed to a remote host, can legitimately exceed
that. Follow `infra-fleet-service.md` §8's "every outbound call to the Dev
Server Agent has an explicit timeout distinct from the default" rule; a
30-60s per-call context is more appropriate here, documented at the call
site the way `fleet.health.checkAll`'s comment documents its own 8s choice
(`channels.go:462-465`).

---

## Test plan

- `services/project-service/internal/usecase/update_repo_test.go` —
  field-mask semantics (empty string leaves a field unchanged, non-empty
  overwrites), fake `RepoRepository`.
- `services/api-gateway/internal/adapter/wscompat/channels_test.go` — one
  test per Bucket-1/2 channel, fake `ProjectServiceClient`, asserting the
  gRPC call args match the decoded WS args and the response passes through.
- `services/git-gateway-service/internal/usecase/clone_test.go` (and one
  per Bucket-3 usecase) — table-driven: connected (relay path, fake
  `DevServerAgentClient.Exec` called with `"git.clone"`) vs. not-connected
  (fake `LocalGitExecutor` called instead) — same two-branch shape
  `scan_workspace_ports_test.go` already establishes for this pattern.
- Regression guard: a test asserting `RegisterRealChannels`'s new
  `projectClient` parameter is non-nil-checked the same way every other
  client param is at each call site, so a future nil dial doesn't panic on
  first `repo.*` call.

## References

- `backend-go/proto/orca/project/v1/project.proto:26-29,136-173` — `ProjectService`'s existing Repo surface (AddRepo/ListRepos/ReorderRepos/RemoveRepo/`Repo` message)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto:10-17` — `GitGatewayService`'s current 6 RPCs (no clone/baseRefDefault/searchRefs/init/hooks/issue-command/setup-script RPC)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:16-18,64-73,221-252,390-433` — channel inventory comment, `RegisterRealChannels`, `registerGitChannels`/`registerDevServerChannels` patterns to mirror
- `backend-go/services/api-gateway/cmd/server/main.go:168,241` — `projectClient` dialed but not passed to `RegisterRealChannels`
- `specs/backend-go/tdd/services/project-service.md` §2-4,§7 — Repo entity as pure metadata, `RebindDevServer`'s saga-guard precedent for cross-service checks
- `specs/backend-go/tdd/services/git-gateway-service.md` §1-3 — resolve→dispatch→translate model, "never trust a client-supplied host path" rule, §3's RPC sketch (does not include this bucket's 8 RPCs — scope addition, flagged above)
- `specs/backend-go/tdd/architecture/03-clean-architecture-guidelines.md` — usecase/port layering
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:17-62` — the resolve→(relay|local) two-branch precedent Bucket 3's usecases follow
- `specs/backend-go/bugs/missing-v1/BUG-023-repo-channels-not-implemented.md` — full 13-method inventory and the bucket split this solution implements
