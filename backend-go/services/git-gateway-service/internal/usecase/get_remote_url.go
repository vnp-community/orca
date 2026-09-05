package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GetRemoteURLInput struct {
	RepoID     string
	RemoteName string // empty = "origin", see GitExecutor.RemoteURL's doc comment
}

// GetRemoteURL resolves repoID's owning host and returns one configured
// remote's raw URL. Repo-scoped, same reasoning as BaseRefDefault (called
// from contexts — github.listWorkItems' owner/repo resolution — with no
// worktree/connection id to resolve from).
type GetRemoteURL struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewGetRemoteURL(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *GetRemoteURL {
	return &GetRemoteURL{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *GetRemoteURL) Execute(ctx context.Context, in GetRemoteURLInput) (string, error) {
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
	url, err := executor.RemoteURL(ctx, repoPath, in.RemoteName)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "GITGATEWAY_REMOTE_NOT_FOUND", "remote does not exist for this repo", err)
	}
	return url, nil
}
