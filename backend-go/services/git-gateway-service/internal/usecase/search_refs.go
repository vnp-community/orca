package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type SearchRefsInput struct {
	RepoID string
	Query  string
}

// SearchRefs resolves repoID's owning host and searches its refs.
// Repo-scoped, same reasoning as BaseRefDefault (see that file's doc
// comment) — this usecase is called from repo-scoped contexts with no
// worktree/connection id, so it must dispatch via dispatchExecutorForRepo,
// not dispatchExecutor/ConnectionResolver.
type SearchRefs struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewSearchRefs(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *SearchRefs {
	return &SearchRefs{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *SearchRefs) Execute(ctx context.Context, in SearchRefsInput) ([]string, error) {
	if in.RepoID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	refs, err := executor.SearchRefs(ctx, repoPath, in.Query)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "GITGATEWAY_SEARCH_REFS_FAILED", "failed to search refs", err)
	}
	return refs, nil
}
