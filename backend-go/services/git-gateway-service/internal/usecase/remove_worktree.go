package usecase

import (
	"context"
	"log"

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
	resolver   ConnectionResolver
	projects   ProjectClient
	scrollback ScrollbackCleaner
	local      GitExecutor
	relay      GitExecutor
}

func NewRemoveWorktree(resolver ConnectionResolver, projects ProjectClient, scrollback ScrollbackCleaner, local, relay GitExecutor) *RemoveWorktree {
	return &RemoveWorktree{resolver: resolver, projects: projects, scrollback: scrollback, local: local, relay: relay}
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
	// Best-effort: an orphaned scrollback row is caught by BR-TM-12's 30-day
	// sweep either way, so a cleanup failure here must not fail the worktree
	// removal itself.
	if err := uc.scrollback.DeleteTerminalScrollbackSnapshots(ctx, worktreeID); err != nil {
		// TODO: thread a structured logger into RemoveWorktree if one isn't
		// already available at this call site; log.Printf is a placeholder.
		log.Printf("remove_worktree: best-effort scrollback cleanup failed for worktree %s: %v", worktreeID, err)
	}
	return nil
}
