package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type CheckHooksInput struct {
	RepoID string
}

type CheckHooksResult struct {
	InstalledHooks   []string
	OrcaHooksCurrent bool
}

// CheckHooks resolves repoID's owning host and checks its git hooks.
// Repo-scoped, like BaseRefDefault/PrefetchCreateBase (see those files' doc
// comments and dispatchExecutorForRepo's own doc comment in ports.go for why
// this can't route through ConnectionResolver/dispatchExecutor): this
// usecase is called from repo-scoped contexts — Settings' Worktree Hooks
// section — that have no worktree/connection id to resolve. Found live:
// GITGATEWAY_MISSING_WORKTREE_ID on every call from Settings, since that
// context never has one to send.
type CheckHooks struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewCheckHooks(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *CheckHooks {
	return &CheckHooks{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *CheckHooks) Execute(ctx context.Context, in CheckHooksInput) (CheckHooksResult, error) {
	if in.RepoID == "" {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	installedHooks, orcaHooksCurrent, err := executor.CheckHooks(ctx, repoPath)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CHECK_HOOKS_FAILED", "failed to check git hooks", err)
	}
	return CheckHooksResult{InstalledHooks: installedHooks, OrcaHooksCurrent: orcaHooksCurrent}, nil
}
