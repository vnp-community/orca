package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// RemoveWorktree has no compensating action on bookkeeping failure — see
// this package's ports.go doc comment for why "on-disk gone, bookkeeping
// stale" is a safe terminal state, unlike CreateWorktree's failure
// direction: "worktree doesn't exist" is a safe terminal state to leave a
// stale bookkeeping record pointing at (unlike a create failure, which
// leaves live, unaccounted-for disk usage and a dangling branch).
//
// worktreeID is passed straight into dispatchExecutor here (not through
// ProjectClient.GetRepo first) — RemoveWorktreeRequest carries a
// worktree_id directly, which IS the dispatch key ConnectionResolver
// expects (see resolver.go's ResolveConnection doc comment), unlike the
// repo-scoped usecases (CreateWorktree, DetectWorktrees, ...) that only
// have a repo_id and must resolve it via GetRepo first.
type RemoveWorktree struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *RemoveWorktree) Execute(ctx context.Context, worktreeID string, force bool) error {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	if err := executor.RemoveWorktree(ctx, repoPath, force); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, worktreeID); err != nil {
		return apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return nil
}
