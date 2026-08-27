package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type AbortRebaseInput struct {
	WorktreeID string
}

type AbortRebase struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewAbortRebase(resolver ConnectionResolver, local, relay GitExecutor) *AbortRebase {
	return &AbortRebase{resolver: resolver, local: local, relay: relay}
}

func (uc *AbortRebase) Execute(ctx context.Context, in AbortRebaseInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.AbortRebase(ctx, repoPath)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_ABORT_REBASE_FAILED", "failed to abort rebase", err)
	}
	return result, nil
}
