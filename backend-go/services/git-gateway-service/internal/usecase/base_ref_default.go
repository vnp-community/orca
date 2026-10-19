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
	reachability RepoHostRouting
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewBaseRefDefault(reachability RepoHostRouting, projects ProjectClient, local, relay GitExecutor) *BaseRefDefault {
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
	ref, err := executor.BaseRefDefault(ctx, repoPath, uc.configuredDefaultBranch(ctx, repo.ProjectID))
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_BASE_REF_DEFAULT_FAILED", "failed to resolve default base ref", err)
	}
	return ref, nil
}

// configuredDefaultBranch is only a hint for local-only repos, so a failed or
// empty lookup must not fail the call: the executor then falls back to
// conventional names.
func (uc *BaseRefDefault) configuredDefaultBranch(ctx context.Context, projectID string) string {
	if projectID == "" {
		return ""
	}
	branch, err := uc.projects.GetProjectDefaultBranch(ctx, projectID)
	if err != nil {
		return ""
	}
	return branch
}
