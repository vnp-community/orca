package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// DetectWorktrees runs `git worktree list --porcelain` for repoID's local
// clone and returns the raw on-disk worktree paths — NO bookkeeping join
// here (that diff happens at api-gateway's edge layer, TASK-195, per
// 05-data-architecture.md's cross-service-aggregation rule: this service
// owns no project-service data to reach into).
//
// Design-gap resolution (flagged in TASK-193's own text, resolved here):
// DetectWorktreesRequest only carries a repo_id, but dispatchExecutor's
// ConnectionResolver resolves by worktree/connection id, not repo id —
// passing repoID straight through would silently conflate the two. Fixed
// the same way CreateWorktree resolves its own repo/host: call
// ProjectClient.GetRepo(repoID) first (turning a nonexistent repo into a
// typed WORKTREE_REPO_NOT_FOUND instead of a confusing resolver failure),
// then dispatch on the confirmed repo.ID it echoes back.
type DetectWorktrees struct {
	resolver ConnectionResolver
	projects ProjectClient
	local    GitExecutor
	relay    GitExecutor
}

func NewDetectWorktrees(resolver ConnectionResolver, projects ProjectClient, local, relay GitExecutor) *DetectWorktrees {
	return &DetectWorktrees{resolver: resolver, projects: projects, local: local, relay: relay}
}

func (uc *DetectWorktrees) Execute(ctx context.Context, repoID string) ([]string, error) {
	repo, err := uc.projects.GetRepo(ctx, repoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, repo.ID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	paths, err := executor.ListWorktreePaths(ctx, repoPath)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "WORKTREE_DETECT_FAILED", "git worktree list failed", err)
	}
	return paths, nil
}
