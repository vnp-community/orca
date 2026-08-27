# TASK-WT-05-04: `MergeWorktreeIntoBase` usecase (BR-WT-16/17) + gRPC handler

**From Solution:** SOL-WT-05
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base.go` (new)
**Depends on:** TASK-WT-05-02, TASK-WT-05-03
**Status:** `[x]` DONE — Created merge_worktree_into_base.go (BR-WT-16/17) + dispatchKeyForRepo repo: prefix; ConnectionResolver.ResolveConnection updated to strip the repo: prefix. gRPC handler wired via positional New(...) in main.go. go build+test clean.

---

## Context

The merge saga: dispatch to the repo's own base-branch checkout (not the winning worktree's own path) via `ProjectClient.GetRepo(repoID)` then `dispatchExecutor(..., repo.ID)` — the identical pattern `CreateWorktree.Execute` already establishes (`create_worktree.go:42-50`), per `dispatchExecutor`'s own doc comment that this dispatch key is reused, unmodified, for repo-scoped operations (`ports.go:373-382`). Depends on `ProjectClient.GetWorktree` from [TASK-WT-04-05](./TASK-WT-04-05-proto-compare-worktrees.md)/[TASK-WT-04-03](./TASK-WT-04-03-usecase-grpc-get-worktree.md) — this task assumes that RPC and port already exist; if the SOL-WT-04 task set hasn't landed yet, this task blocks on it too.

## Changes to make

Create `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base.go`:

```go
package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type MergeWorktreeIntoBase struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewMergeWorktreeIntoBase(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *MergeWorktreeIntoBase {
	return &MergeWorktreeIntoBase{resolver: resolver, projects: projects, local: local, relay: relay}
}

type MergeWorktreeInput struct {
	WorktreeID, BaseBranch, Strategy, CommitMessage string
}

func (uc *MergeWorktreeIntoBase) Execute(ctx context.Context, in MergeWorktreeInput) (domain.MergeResult, error) {
	if in.Strategy != "merge" && in.Strategy != "squash" && in.Strategy != "rebase" {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInvalidArgument, "MERGE_STRATEGY_INVALID", "strategy must be merge, squash, or rebase", nil)
	}

	wt, err := uc.projects.GetWorktree(ctx, in.WorktreeID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_NOT_FOUND", "worktree not found", err)
	}

	// BR-WT-16 — must commit all changes before merging.
	wtExecutor, wtPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	if status, err := wtExecutor.GetStatus(ctx, wtPath); err == nil && len(status.Files) > 0 {
		return domain.MergeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "MERGE_UNCOMMITTED_CHANGES",
			fmt.Sprintf("%d uncommitted change(s) in the winning worktree", len(status.Files)), nil)
	}

	repo, err := uc.projects.GetRepo(ctx, wt.RepoID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	// Dispatch against the REPO's own checkout, not the winning worktree's
	// path — a merge target must be base_branch checked out, typically the
	// primary clone. dispatchExecutor's worktreeID param is reused,
	// unmodified, as the dispatch key here (ports.go's doc comment) — the
	// same pattern CreateWorktree.Execute already uses for repo.ID.
	mainExecutor, mainRepoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "REPO_RESOLVE_FAILED", "failed to resolve host", err)
	}

	if in.Strategy == "rebase" {
		// Rebase the winner's branch onto base_branch, then fast-forward
		// base_branch to the rebased tip — both already-real RPCs, no new
		// git logic for this path.
		if _, err := wtExecutor.RebaseFromBase(ctx, wtPath, in.BaseBranch); err != nil {
			return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "MERGE_REBASE_FAILED", "rebase onto base failed", err)
		}
		ffResult, err := mainExecutor.FastForward(ctx, mainRepoPath, &domain.PushTargetInput{RemoteName: "origin", Branch: wt.Branch})
		if err != nil {
			return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "MERGE_FAST_FORWARD_FAILED", "fast-forward of base branch failed", err)
		}
		return domain.MergeResult{ResultSHA: ffResult.ResultSHA}, nil
	}

	// "merge" / "squash".
	result, err := mainExecutor.MergeBranch(ctx, mainRepoPath, wt.Branch, in.Strategy, in.CommitMessage)
	if err != nil {
		return domain.MergeResult{}, apperrors.New(apperrors.KindInternal, "MERGE_FAILED", "merge failed", err)
	}
	if result.HasConflicts {
		// BR-WT-17 — deliberately NOT auto-resolved or auto-aborted. The
		// main repo checkout is left in the conflicted state; the client
		// resolves via the existing ConflictOperation/ResolveConflict/
		// AbortMerge RPCs using ConflictDispatchKey — see this file's
		// "genuine extension" note below.
		return domain.MergeResult{HasConflicts: true, ConflictedPaths: result.ConflictedPaths, ConflictDispatchKey: dispatchKeyForRepo(repo.ID)}, nil
	}
	return domain.MergeResult{ResultSHA: result.ResultSHA}, nil
}

// dispatchKeyForRepo is a genuine, explicitly flagged extension beyond what
// git-gateway-service.md documents (SOL-WT-05): AbortMerge/ConflictOperation/
// ResolveConflict's request messages all name their dispatch field
// worktree_id and every existing caller passes a real project.worktrees row
// id — but a conflicted MergeBranch happens against the repo's own
// checkout, which has no worktree_id in project-service's bookkeeping.
// Rather than triplicate those 3 RPCs into repo-scoped duplicates, this
// broadens what dispatchExecutor already treats as an opaque dispatch key:
// a "repo:"-prefixed id, resolved by ConnectionResolver the same way
// CreateWorktree already resolves a bare repo.ID. ConnectionResolver's
// ResolveConnection implementation must special-case this prefix — see
// this task's own Context section; if that resolver change isn't in place
// yet, this key is inert until it is.
func dispatchKeyForRepo(repoID string) string {
	return "repo:" + repoID
}
```

**Flagged extension, not silently resolved**: `dispatchKeyForRepo`'s `"repo:"` prefix scheme requires `ConnectionResolver.ResolveConnection` (in `internal/adapter/grpcclient/resolver.go`) to recognize and strip that prefix before doing its normal worktree-id resolution — check that file's current implementation and add the special case as part of this task; without it, a conflicted merge's `ConflictDispatchKey` is unusable by the 3 existing conflict RPCs.

Add the gRPC handler to `backend-go/services/git-gateway-service/internal/adapter/grpc/server.go` (near `AbortMerge`), and wire `mergeWorktreeIntoBase *usecase.MergeWorktreeIntoBase` through the `Server` struct + `New(...)` constructor + `cmd/server/main.go`, same pattern as every other usecase:

```go
func (s *Server) MergeBranch(ctx context.Context, req *gitgatewayv1.MergeBranchRequest) (*gitgatewayv1.MergeBranchResponse, error) {
	result, err := s.mergeWorktreeIntoBase.Execute(ctx, usecase.MergeWorktreeInput{
		WorktreeID: req.GetWorktreeId(), BaseBranch: req.GetBaseBranch(),
		Strategy: req.GetStrategy(), CommitMessage: req.GetCommitMessage(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &gitgatewayv1.MergeBranchResponse{
		ResultSha: result.ResultSHA, HasConflicts: result.HasConflicts,
		ConflictedPaths: result.ConflictedPaths, ConflictDispatchKey: result.ConflictDispatchKey,
	}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build. Behavior tests land in [TASK-WT-05-06](./TASK-WT-05-06-tests-usecase-and-executor.md).
