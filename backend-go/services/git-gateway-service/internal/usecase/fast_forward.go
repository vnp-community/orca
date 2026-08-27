package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// FastForwardInput's PushTarget is optional (nil = let the executor resolve
// the worktree's configured push target) — see domain.PushTargetInput's doc
// comment.
type FastForwardInput struct {
	WorktreeID string
	PushTarget *domain.PushTargetInput
}

type FastForward struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewFastForward(resolver ConnectionResolver, local, relay GitExecutor) *FastForward {
	return &FastForward{resolver: resolver, local: local, relay: relay}
}

func (uc *FastForward) Execute(ctx context.Context, in FastForwardInput) (domain.FastForwardResult, error) {
	if in.WorktreeID == "" {
		return domain.FastForwardResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.FastForwardResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.FastForward(ctx, repoPath, in.PushTarget)
	if err != nil {
		return domain.FastForwardResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_FAST_FORWARD_FAILED", "failed to fast-forward", err)
	}
	return result, nil
}
