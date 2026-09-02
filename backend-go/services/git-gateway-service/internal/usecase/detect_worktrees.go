package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// DetectWorktrees runs `git worktree list --porcelain` for repoID's local
// clone and returns the raw on-disk worktrees (path + HEAD + branch) — NO
// bookkeeping join here (that diff happens at api-gateway's edge layer,
// TASK-195, per 05-data-architecture.md's cross-service-aggregation rule:
// this service owns no project-service data to reach into). Returning the
// git-level identity alongside each path (not just the path) is what lets
// that edge layer build a real reconciled worktree record for a path with
// no bookkeeping row, instead of only a boolean "orphaned" flag.
//
// Design-gap resolution (flagged in TASK-193's own text): DetectWorktrees-
// Request only carries a repo_id, not a worktree/connection id — resolved
// by calling ProjectClient.GetRepo(repoID) first (turning a nonexistent
// repo into a typed WORKTREE_REPO_NOT_FOUND), then dispatching via
// dispatchExecutorForRepo on the confirmed repo it returns — see that
// function's doc comment for why this does NOT go through dispatchExecutor/
// ConnectionResolver (an earlier revision did; it never worked).
type DetectWorktrees struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewDetectWorktrees(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *DetectWorktrees {
	return &DetectWorktrees{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *DetectWorktrees) Execute(ctx context.Context, repoID string) ([]domain.WorktreeGitInfo, error) {
	repo, err := uc.projects.GetRepo(ctx, repoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	paths, err := executor.ListWorktreePaths(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "WORKTREE_DETECT_FAILED", "git worktree list failed", err)
	}
	return paths, nil
}
