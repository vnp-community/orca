# TASK-160: Wire `repo.clone`/`baseRefDefault`/`searchRefs`/`create`/`hooksCheck`/`issueCommandRead`/`issueCommandWrite`/`setupScriptImports` channels (Bucket 3)

**From Solution:** SOL-023 (Bucket 3)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-151 (`registerRepoChannels` must exist), TASK-159 (git-gateway-service RPCs must exist)
**Status:** `[ ]` TODO

---

## Context

`repo.clone`'s deadline needs an override above `rpcTimeout` (8s) —
cloning a real repo, especially relayed to a remote host, can legitimately
exceed that. Follow `infra-fleet-service.md` §8's "every outbound call to
the Dev Server Agent has an explicit timeout distinct from the default"
rule; a 30-60s per-call context is more appropriate here, documented at
the call site the way `fleet.health.checkAll`'s comment documents its own
8s choice (`channels.go:462-465`).

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

In `registerRepoChannels` (TASK-151), replace:

```go
	// repo.baseRefDefault / repo.clone / repo.searchRefs / repo.create /
	// repo.hooksCheck / repo.issueCommandRead / repo.issueCommandWrite /
	// repo.setupScriptImports join here, against `git`, once TASK-159's
	// RPCs exist (TASK-160). `git` is accepted as a parameter now so this
	// function's signature does not change again later.
	_ = git
```

with:

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
		// Longer-than-rpcTimeout deadline: cloning a real repo, especially
		// relayed to a remote host, can legitimately exceed rpcTimeout's 8s.
		// Same reasoning as ssh.connect's 20s override (SOL-024).
		rpcCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		resp, err := git.Clone(rpcCtx, &gitgatewayv1.CloneRequest{
			DevServerId: in.DevServerID, Url: in.URL, DestPath: in.DestPath,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.baseRefDefault", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type baseRefDefaultArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[baseRefDefaultArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := git.BaseRefDefault(rpcCtx, &gitgatewayv1.BaseRefDefaultRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.searchRefs", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type searchRefsArgs struct {
			WorktreeID string `json:"worktreeId"`
			Query      string `json:"query"`
		}
		in, err := decodeArg[searchRefsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := git.SearchRefs(rpcCtx, &gitgatewayv1.SearchRefsRequest{WorktreeId: in.WorktreeID, Query: in.Query})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID   string `json:"devServerId"`
			DestPath      string `json:"destPath"`
			DefaultBranch string `json:"defaultBranch"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := git.InitRepo(rpcCtx, &gitgatewayv1.InitRepoRequest{
			DevServerId: in.DevServerID, DestPath: in.DestPath, DefaultBranch: in.DefaultBranch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.hooksCheck", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type hooksCheckArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[hooksCheckArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := git.CheckHooks(rpcCtx, &gitgatewayv1.CheckHooksRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.issueCommandRead", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type issueCommandReadArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[issueCommandReadArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := git.ReadIssueCommand(rpcCtx, &gitgatewayv1.ReadIssueCommandRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("repo.issueCommandWrite", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type issueCommandWriteArgs struct {
			WorktreeID string `json:"worktreeId"`
			Content    string `json:"content"`
		}
		in, err := decodeArg[issueCommandWriteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err = git.WriteIssueCommand(rpcCtx, &gitgatewayv1.WriteIssueCommandRequest{WorktreeId: in.WorktreeID, Content: in.Content})
		return nil, err
	})

	r.Register("repo.setupScriptImports", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type setupScriptImportsArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[setupScriptImportsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := git.ScanSetupScriptImports(rpcCtx, &gitgatewayv1.ScanSetupScriptImportsRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

This removes the `_ = git` placeholder — `git` is now used by every
handler above.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```
