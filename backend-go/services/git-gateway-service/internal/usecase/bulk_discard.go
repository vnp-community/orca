package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type BulkDiscardInput struct {
	WorktreeID string
	Paths      []string
}

type BulkDiscard struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewBulkDiscard(resolver ConnectionResolver, local, relay GitExecutor) *BulkDiscard {
	return &BulkDiscard{resolver: resolver, local: local, relay: relay}
}

func (uc *BulkDiscard) Execute(ctx context.Context, in BulkDiscardInput) (domain.BulkDiscardResult, error) {
	if in.WorktreeID == "" {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if len(in.Paths) == 0 {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_PATHS", "paths is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.BulkDiscard(ctx, repoPath, in.Paths)
	if err != nil {
		return domain.BulkDiscardResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_BULK_DISCARD_FAILED", "failed to discard paths", err)
	}
	return result, nil
}
