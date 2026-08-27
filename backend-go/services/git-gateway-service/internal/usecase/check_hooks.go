package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

type CheckHooksInput struct {
	WorktreeID string
}

type CheckHooksResult struct {
	InstalledHooks   []string
	OrcaHooksCurrent bool
}

// CheckHooks follows GetStatus's exact resolve -> dispatch -> translate shape.
type CheckHooks struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCheckHooks(resolver ConnectionResolver, local, relay GitExecutor) *CheckHooks {
	return &CheckHooks{resolver: resolver, local: local, relay: relay}
}

func (uc *CheckHooks) Execute(ctx context.Context, in CheckHooksInput) (CheckHooksResult, error) {
	if in.WorktreeID == "" {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	installedHooks, orcaHooksCurrent, err := executor.CheckHooks(ctx, repoPath)
	if err != nil {
		return CheckHooksResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CHECK_HOOKS_FAILED", "failed to check git hooks", err)
	}
	return CheckHooksResult{InstalledHooks: installedHooks, OrcaHooksCurrent: orcaHooksCurrent}, nil
}
