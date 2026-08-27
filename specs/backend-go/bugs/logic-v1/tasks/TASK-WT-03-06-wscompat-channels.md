# TASK-WT-03-06: `worktree.checkDeleteSafety` channel + `worktree.rm` threads `stopAgents`

**From Solution:** SOL-WT-03
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`
**Depends on:** TASK-WT-03-04, TASK-WT-03-05
**Status:** `[x]` DONE — worktree.checkDeleteSafety channel added; worktree.rm already threaded stopAgents from Phase 1's earlier commit; builds clean

---

## Context

Wires the new `CheckWorktreeDeleteSafety` RPC to the WS surface and updates `worktree.rm` (`channels_worktree.go:76-90`) to pass `stopAgents` through and return the real `RemoveWorktreeResponse` instead of the current `map[string]bool{"ok": true}` placeholder.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`, add a new channel:

```go
	r.Register("worktree.checkDeleteSafety", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type checkArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[checkArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.CheckWorktreeDeleteSafety(ctx, &gitgatewayv1.CheckWorktreeDeleteSafetyRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

Replace the existing `worktree.rm` registration (`channels_worktree.go:76-90`):

```go
	r.Register("worktree.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type rmArgs struct {
			WorktreeID string `json:"worktreeId"`
			Force      bool   `json:"force"`
			StopAgents bool   `json:"stopAgents"`
		}
		in, err := decodeArg[rmArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{
			WorktreeId: in.WorktreeID, Force: in.Force, StopAgents: in.StopAgents,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
```

No `channels.go`/`main.go` signature changes needed — `registerWorktreeChannels` already receives `gitClient`, and this task adds no new dependency to it.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
```

Expected: clean build. Channel tests land in [TASK-WT-03-07](./TASK-WT-03-07-tests.md).
