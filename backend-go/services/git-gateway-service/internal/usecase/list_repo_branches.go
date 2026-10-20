package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type ListRepoBranchesInput struct {
	RepoID string
}

type ListRepoBranchesResult struct {
	Branches  []domain.RepoBranch
	Truncated bool
}

// ListRepoBranches lists the branches of repoID on its owning host. Repo-scoped like SearchRefs:
// the caller has no worktree/connection id, so it dispatches via dispatchExecutorForRepo.
type ListRepoBranches struct {
	reachability RepoHostRouting
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewListRepoBranches(reachability RepoHostRouting, projects ProjectClient, local, relay GitExecutor) *ListRepoBranches {
	return &ListRepoBranches{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *ListRepoBranches) Execute(ctx context.Context, in ListRepoBranchesInput) (ListRepoBranchesResult, error) {
	if in.RepoID == "" {
		return ListRepoBranchesResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return ListRepoBranchesResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return ListRepoBranchesResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	branches, truncated, err := executor.ListRepoBranches(ctx, repoPath)
	if err != nil {
		return ListRepoBranchesResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_LIST_BRANCHES_FAILED", "failed to list repo branches", err)
	}
	return ListRepoBranchesResult{Branches: branches, Truncated: truncated}, nil
}
