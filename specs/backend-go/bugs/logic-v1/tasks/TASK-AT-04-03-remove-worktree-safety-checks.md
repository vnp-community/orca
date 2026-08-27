# TASK-AT-04-03: `RemoveWorktree` — mandatory uncommitted-changes and open-PR safety checks (BR-AT-11/BR-AT-12)

**From Solution:** SOL-AT-04
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go`, `backend-go/services/git-gateway-service/internal/usecase/ports.go`
**Depends on:** TASK-AT-04-01
**Status:** `[ ]` TODO

---

## Context

`RemoveWorktree.Execute` today takes `force` straight from the caller with
no uncommitted-changes or open-PR check at all — unenforced even for the
existing single-delete path. Fixing it here fixes it for every caller
(manual `worktree.rm` AND SOL-AT-04's new bulk `cleanup_worktrees` path).

## Changes to make

Extend the `SCMClient` port in `ports.go` (if not already present) with:

```go
GetPullRequestForBranch(ctx context.Context, tenantID, branch string) (pr PullRequestInfo, found bool, err error)
```

(This RPC already exists on `scm-integration-service` per SOL-AT-04's design
— reuse it, add only the client-side port method/wiring if missing.)

In `remove_worktree.go`, update `Execute` to accept `allowOpenPR bool` and
add both checks before the actual removal call:

```go
func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force, allowOpenPR bool) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-AT-11 — fails closed on its own check error.
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_CHECK_FAILED", "failed to check worktree status before removal", err)
	}
	if len(status.Files) > 0 && !force {
		return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES", "worktree has uncommitted changes", nil)
	}

	// BR-AT-12 — independent of force; requires the SEPARATE allow_open_pr
	// override.
	branch, err := uc.currentBranch(ctx, executor, repoPath)
	if err == nil && branch != "" {
		pr, found, err := uc.scm.GetPullRequestForBranch(ctx, uc.tenantIDFromCtx(ctx), branch)
		if err == nil && found && pr.State == "open" && !allowOpenPR {
			return apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_OPEN_PR", "worktree's branch has an open pull request", nil)
		}
		// A GetPullRequestForBranch error fails OPEN on this check only — a
		// repo with no SCM integration has no way to answer this question,
		// and BR-AT-12 shouldn't make its worktrees permanently undeletable.
	}

	if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return nil
}
```

Update the gRPC handler (`internal/adapter/grpc/server.go`) and the WS
`worktree.rm` channel handler to thread the new `allow_open_pr` request
field through to `Execute`, defaulting to `false` when absent (existing
manual-delete callers get the new BR-AT-12 protection by default — this is
an intentional behavior change, document it in cutover notes per this
solution's own test-plan note).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
go test ./services/git-gateway-service/internal/usecase/... -run TestRemoveWorktree
```

Expected: fake `GitExecutor.GetStatus` returns dirty files, `force=false` →
`WORKTREE_HAS_UNCOMMITTED_CHANGES`, `executor.RemoveWorktree` never called;
`force=true` → proceeds. Fake `SCMClient` reports an open PR,
`allow_open_pr=false` → `WORKTREE_HAS_OPEN_PR`; `allow_open_pr=true` →
proceeds. `SCMClient` errors (no integration configured) → deletion proceeds
(fail-open).
