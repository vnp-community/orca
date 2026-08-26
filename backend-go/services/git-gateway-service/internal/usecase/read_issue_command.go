package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type ReadIssueCommandInput struct {
	WorktreeID string
}

type ReadIssueCommandResult struct {
	Content string
	Exists  bool
}

// ReadIssueCommand follows GetStatus's exact resolve -> dispatch -> translate shape.
type ReadIssueCommand struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewReadIssueCommand(resolver ConnectionResolver, local, relay GitExecutor) *ReadIssueCommand {
	return &ReadIssueCommand{resolver: resolver, local: local, relay: relay}
}

func (uc *ReadIssueCommand) Execute(ctx context.Context, in ReadIssueCommandInput) (ReadIssueCommandResult, error) {
	if in.WorktreeID == "" {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	content, exists, err := executor.ReadIssueCommand(ctx, repoPath)
	if err != nil {
		return ReadIssueCommandResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_READ_ISSUE_COMMAND_FAILED", "failed to read issue command file", err)
	}
	return ReadIssueCommandResult{Content: content, Exists: exists}, nil
}
