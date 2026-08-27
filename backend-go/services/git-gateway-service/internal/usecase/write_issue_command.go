package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type WriteIssueCommandInput struct {
	WorktreeID string
	Content    string
}

// WriteIssueCommand follows GetStatus's exact resolve -> dispatch -> translate shape.
type WriteIssueCommand struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewWriteIssueCommand(resolver ConnectionResolver, local, relay GitExecutor) *WriteIssueCommand {
	return &WriteIssueCommand{resolver: resolver, local: local, relay: relay}
}

func (uc *WriteIssueCommand) Execute(ctx context.Context, in WriteIssueCommandInput) error {
	if in.WorktreeID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if err := executor.WriteIssueCommand(ctx, repoPath, in.Content); err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_WRITE_ISSUE_COMMAND_FAILED", "failed to write issue command file", err)
	}
	return nil
}
