package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// MergeWorktreeIntoBase is the merge saga: dispatch to the repo's own
// base-branch checkout (not the winning worktree's own path) via
// ProjectClient.GetRepo(repoID) then dispatchExecutor(..., repo.ID) — the
// identical pattern CreateWorktree.Execute already establishes, per
// dispatchExecutor's own doc comment that this dispatch key is reused,
// unmodified, for repo-scoped operations.
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
		ffResult, err := mainExecutor.FastForward(ctx, mainRepoPath, &domain.PushTargetInput{RemoteName: "origin", BranchName: wt.Branch})
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
		// AbortMerge RPCs using ConflictDispatchKey — see dispatchKeyForRepo's
		// doc comment below.
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
// CreateWorktree already resolves a bare repo.ID — see grpcclient's
// ConnectionResolver.ResolveConnection, which special-cases this prefix.
func dispatchKeyForRepo(repoID string) string {
	return "repo:" + repoID
}
