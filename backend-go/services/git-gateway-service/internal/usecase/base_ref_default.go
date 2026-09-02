package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type BaseRefDefaultInput struct {
	RepoID string
}

// BaseRefDefault resolves repoID's owning host and returns its default base
// ref. Repo-scoped, like PrefetchCreateBase (see that file's doc comment and
// dispatchExecutorForRepo's own doc comment in ports.go for why this can't
// route through ConnectionResolver/dispatchExecutor: this usecase is called
// from repo-scoped contexts — Settings, SourceControl's default-branch hint —
// that have no worktree/connection id to resolve.
type BaseRefDefault struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewBaseRefDefault(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *BaseRefDefault {
	return &BaseRefDefault{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *BaseRefDefault) Execute(ctx context.Context, in BaseRefDefaultInput) (string, error) {
	if in.RepoID == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	ref, err := executor.BaseRefDefault(ctx, repoPath)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_BASE_REF_DEFAULT_FAILED", "failed to resolve default base ref", err)
	}
	return ref, nil
}
