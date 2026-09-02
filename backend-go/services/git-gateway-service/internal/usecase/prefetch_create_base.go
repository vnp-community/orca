package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// PrefetchCreateBase ensures baseRef is fetched/up to date in repoID's
// local clone and returns its resolved SHA. See detect_worktrees.go's doc
// comment for why this resolves repoID via ProjectClient.GetRepo before
// dispatching, rather than passing repoID straight into dispatchExecutor.
type PrefetchCreateBase struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewPrefetchCreateBase(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *PrefetchCreateBase {
	return &PrefetchCreateBase{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *PrefetchCreateBase) Execute(ctx context.Context, repoID, baseRef string) (string, error) {
	repo, err := uc.projects.GetRepo(ctx, repoID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	sha, err := executor.FetchAndResolveRef(ctx, repoPath, baseRef)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "WORKTREE_PREFETCH_FAILED", "failed to prefetch base ref", err)
	}
	return sha, nil
}
