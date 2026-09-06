package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type WriteIssueCommandInput struct {
	RepoID  string
	Content string
}

// WriteIssueCommand resolves repoID's owning host and writes its issue
// command file. Repo-scoped — see CheckHooks' doc comment for why.
type WriteIssueCommand struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewWriteIssueCommand(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *WriteIssueCommand {
	return &WriteIssueCommand{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *WriteIssueCommand) Execute(ctx context.Context, in WriteIssueCommandInput) error {
	if in.RepoID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	if err := executor.WriteIssueCommand(ctx, repoPath, in.Content); err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_WRITE_ISSUE_COMMAND_FAILED", "failed to write issue command file", err)
	}
	return nil
}
