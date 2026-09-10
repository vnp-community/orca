package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ReadIssueCommandInput struct {
	RepoID string
}

type ReadIssueCommandResult struct {
	Content string
	Exists  bool
}

// ReadIssueCommand resolves repoID's owning host and reads its issue
// command file. Repo-scoped — see CheckHooks' doc comment for why.
type ReadIssueCommand struct {
	reachability DevServerReachability
	projects     ProjectClient
	local        GitExecutor
	relay        GitExecutor
}

func NewReadIssueCommand(reachability DevServerReachability, projects ProjectClient, local, relay GitExecutor) *ReadIssueCommand {
	return &ReadIssueCommand{reachability: reachability, projects: projects, local: local, relay: relay}
}

func (uc *ReadIssueCommand) Execute(ctx context.Context, in ReadIssueCommandInput) (ReadIssueCommandResult, error) {
	if in.RepoID == "" {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_REPO_ID", "repo_id is required", nil)
	}
	repo, err := uc.projects.GetRepo(ctx, in.RepoID)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindNotFound, "WORKTREE_REPO_NOT_FOUND", "repo does not exist", err)
	}
	ctx, executor, repoPath, err := dispatchExecutorForRepo(ctx, uc.reachability, uc.local, uc.relay, repo)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve repo's owning host", err)
	}
	content, exists, err := executor.ReadIssueCommand(ctx, repoPath)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_READ_ISSUE_COMMAND_FAILED", "failed to read issue command file", err)
	}
	return ReadIssueCommandResult{Content: content, Exists: exists}, nil
}
