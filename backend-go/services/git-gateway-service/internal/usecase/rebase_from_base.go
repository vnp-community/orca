package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type RebaseFromBaseInput struct {
	WorktreeID string
	BaseRef    string
}

type RebaseFromBase struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewRebaseFromBase(resolver ConnectionResolver, local, relay GitExecutor) *RebaseFromBase {
	return &RebaseFromBase{resolver: resolver, local: local, relay: relay}
}

func (uc *RebaseFromBase) Execute(ctx context.Context, in RebaseFromBaseInput) (domain.RebaseResult, error) {
	if in.WorktreeID == "" {
		return domain.RebaseResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.BaseRef == "" {
		return domain.RebaseResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_BASE_REF", "base_ref is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.RebaseResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.RebaseFromBase(ctx, repoPath, in.BaseRef)
	if err != nil {
		return domain.RebaseResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_REBASE_FAILED", "failed to rebase from base", err)
	}
	return result, nil
}
