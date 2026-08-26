# TASK-195: Wire all 8 `worktree.*` `wscompat` channels — `worktree.detectedList` is a cross-service aggregation

**From Solution:** SOL-031 (design part 4: "`wscompat` wiring")
**Priority:** P1
**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/channels_worktree.go` (new), `internal/adapter/wscompat/channels.go`, `cmd/server/main.go`
**Depends on:** TASK-194
**Status:** `[ ]` TODO

---

## Context

7 of the 8 channels are direct 1:1 unary wrappers, same shape as
`registerGitChannels`. `worktree.list`/`worktree.set` are pure
bookkeeping wrappers over `ProjectServiceClient.ListWorktrees`/
`SetWorktreeActivation` (already-real RPCs, per BUG-031 — no
`git-gateway-service` involvement). `worktree.detectedList` is the one
aggregation: `git-gateway-service.DetectWorktrees`'s raw on-disk paths
merged against `project-service.ListWorktrees`'s bookkeeping, computed at
`api-gateway`'s edge layer per `05-data-architecture.md`'s explicit
prescription for a multi-service view — parallel gRPC calls via
`errgroup`, merged here, not inside either owning service.

`api-gateway`'s `cmd/server/main.go` already dials both `gitgateway-service`
(`gitClient`) and `project-service` (`projectClient`) — `projectClient` is
not currently passed into `RegisterRealChannels` (only used by REST
routes), so this task adds it there, same pattern TASK-178/190 used for
`tenantClient`/`workflowClient`.

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_worktree.go`

```go
// ── worktree.* (git-gateway-service + project-service) ──────────────────
package wscompat

import (
	"context"
	"encoding/json"

	"golang.org/x/sync/errgroup"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerWorktreeChannels(r *Registry, gitClient gitgatewayv1.GitGatewayServiceClient, projectClient projectv1.ProjectServiceClient) {
	r.Register("worktree.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			ProjectID string `json:"projectId"`
			RepoID    string `json:"repoId"`
			Branch    string `json:"branch"`
			BaseRef   string `json:"baseRef"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.CreateWorktree(ctx, &gitgatewayv1.CreateWorktreeRequest{
			ProjectId: in.ProjectID, RepoId: in.RepoID, Branch: in.Branch, BaseRef: in.BaseRef,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			WorktreeID string `json:"worktreeId"`
			Force      bool   `json:"force"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: in.WorktreeID, Force: in.Force}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("worktree.forceDeleteBranch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type forceDeleteArgs struct {
			WorktreeID string `json:"worktreeId"`
			Branch     string `json:"branch"`
		}
		in, err := decodeArg[forceDeleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		if _, err := gitClient.ForceDeleteBranch(ctx, &gitgatewayv1.ForceDeleteBranchRequest{WorktreeId: in.WorktreeID, Branch: in.Branch}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	r.Register("worktree.prefetchCreateBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type prefetchArgs struct {
			RepoID  string `json:"repoId"`
			BaseRef string `json:"baseRef"`
		}
		in, err := decodeArg[prefetchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.PrefetchCreateBase(ctx, &gitgatewayv1.PrefetchCreateBaseRequest{RepoId: in.RepoID, BaseRef: in.BaseRef})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.resolvePrBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			RepoID   string `json:"repoId"`
			PrNumber int32  `json:"prNumber"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.ResolvePrBase(ctx, &gitgatewayv1.ResolvePrBaseRequest{RepoId: in.RepoID, PrNumber: in.PrNumber})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.resolveMrBase", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			RepoID   string `json:"repoId"`
			MrNumber int32  `json:"mrNumber"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.ResolveMrBase(ctx, &gitgatewayv1.ResolveMrBaseRequest{RepoId: in.RepoID, MrNumber: in.MrNumber})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("worktree.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := projectClient.ListWorktrees(ctx, &projectv1.ListWorktreesRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		return resp.GetWorktrees(), nil
	})

	r.Register("worktree.set", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type setArgs struct {
			WorktreeID string `json:"worktreeId"`
			Active     bool   `json:"active"`
		}
		in, err := decodeArg[setArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := projectClient.SetWorktreeActivation(ctx, &projectv1.SetWorktreeActivationRequest{WorktreeId: in.WorktreeID, Active: in.Active})
		if err != nil {
			return nil, err
		}
		return resp.GetWorktree(), nil
	})

	// worktree.detectedList — the one aggregation. Parallel calls, merged
	// at the edge, per 05-data-architecture.md's explicit prescription for
	// a multi-service view: neither owning service reaches into the
	// other's data.
	r.Register("worktree.detectedList", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type detectedListArgs struct {
			ProjectID string `json:"projectId"`
			RepoID    string `json:"repoId"`
		}
		in, err := decodeArg[detectedListArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})

		var onDisk *gitgatewayv1.DetectWorktreesResponse
		var known *projectv1.ListWorktreesResponse
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() (err error) {
			onDisk, err = gitClient.DetectWorktrees(gctx, &gitgatewayv1.DetectWorktreesRequest{RepoId: in.RepoID})
			return
		})
		g.Go(func() (err error) {
			known, err = projectClient.ListWorktrees(gctx, &projectv1.ListWorktreesRequest{ProjectId: in.ProjectID})
			return
		})
		if err := g.Wait(); err != nil {
			return nil, err
		}

		knownPaths := make(map[string]bool, len(known.GetWorktrees()))
		for _, w := range known.GetWorktrees() {
			knownPaths[w.GetPath()] = true
		}
		var orphaned []string
		for _, p := range onDisk.GetOnDiskPaths() {
			if !knownPaths[p] {
				orphaned = append(orphaned, p)
			}
		}
		return map[string]any{"orphanedPaths": orphaned}, nil
	})
}
```

Confirm `golang.org/x/sync/errgroup` is already a dependency of this module
(`go.mod`) — if not, add it via `go get golang.org/x/sync/errgroup` before
this file will build.

### `channels.go`: grow `RegisterRealChannels`

Add `projectClient projectv1.ProjectServiceClient` as a new parameter (if
not already present from an earlier, unrelated task pass — check before
adding a duplicate parameter) and add
`registerWorktreeChannels(r, gitClient, projectClient)`. `gitClient
gitgatewayv1.GitGatewayServiceClient` is already a parameter of this
function (used by `registerGitChannels`) — reuse it, do not add a second
one. Add the `projectv1` import to `channels.go` if not already present.

### `cmd/server/main.go`: pass `projectClient` through

`projectClient` is already dialed (`projectConn`/`projectClient` block,
used by `httpgateway.NewRouter`'s `Deps.ProjectClient`). Update the
`wscompat.RegisterRealChannels(...)` call site to append `projectClient` as
the final argument (after TASK-190's `workflowClient`, matching whatever
order TASK-190 left the parameter list in).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
