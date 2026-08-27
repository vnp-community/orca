# TASK-WT-05-05: `worktree.merge` wscompat channel + optional cleanup composition (BR-WT-18)

**From Solution:** SOL-WT-05
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`
**Depends on:** TASK-WT-05-04
**Status:** `[ ]` TODO

---

## Context

Per [SOL-WT-05](../solutions/SOL-WT-05-merge-worktree.md), BR-WT-18 only requires cleanup to be *optional*, not a new backend capability — `RemoveWorktree` ([SOL-WT-03](../solutions/SOL-WT-03-xoa-worktree.md), now safety-checked) is real and sufficient. Composing "call `worktree.rm` for each id after a successful merge" at the edge, driven entirely by whether the client includes `cleanupWorktreeIds`, keeps this optional-by-construction and reuses [SOL-WT-02](../solutions/SOL-WT-02-fan-out-worktree.md)'s edge-composition precedent, this time for a post-success chained call rather than a fan-out.

## Changes to make

Add to `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go`:

```go
	r.Register("worktree.merge", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type mergeArgs struct {
			WorktreeID          string   `json:"worktreeId"`
			BaseBranch          string   `json:"baseBranch"`
			Strategy            string   `json:"strategy"`
			CommitMessage       string   `json:"commitMessage"`
			CleanupWorktreeIDs  []string `json:"cleanupWorktreeIds"` // BR-WT-18 — optional
		}
		in, err := decodeArg[mergeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := gitClient.MergeBranch(ctx, &gitgatewayv1.MergeBranchRequest{
			WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch, Strategy: in.Strategy, CommitMessage: in.CommitMessage,
		})
		if err != nil {
			return nil, err
		}
		if resp.GetHasConflicts() || len(in.CleanupWorktreeIDs) == 0 {
			return resp, nil // never auto-cleanup on a conflicted merge
		}

		// BR-WT-18 — optional, best-effort, per-item isolated the same way
		// SOL-WT-02's fan-out is: one cleanup failure must not mask the
		// successful merge response.
		cleanupResults := make(map[string]string, len(in.CleanupWorktreeIDs))
		for _, wtID := range in.CleanupWorktreeIDs {
			if _, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{WorktreeId: wtID, Force: false}); err != nil {
				cleanupResults[wtID] = err.Error()
			} else {
				cleanupResults[wtID] = "removed"
			}
		}
		return map[string]any{"merge": resp, "cleanup": cleanupResults}, nil
	})
```

Note: this call site uses `gitClient.RemoveWorktree`'s current signature — if [TASK-WT-03-01](./TASK-WT-03-01-proto-delete-safety-and-stop-agents.md) has already landed, `RemoveWorktreeRequest` also has a `StopAgents` field (default `false` here, matching this cleanup's best-effort, non-destructive-to-running-agents posture) and the response type is `*gitgatewayv1.RemoveWorktreeResponse` instead of `*emptypb.Empty` — adjust the `cleanupResults` loop's error handling accordingly (the shape above already treats it as `(resp, error)`, so no change needed beyond the return type).

No `channels.go`/`main.go` signature changes needed — `registerWorktreeChannels` already receives `gitClient`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
```

Expected: clean build. Channel tests land in [TASK-WT-05-07](./TASK-WT-05-07-tests-wscompat.md).
