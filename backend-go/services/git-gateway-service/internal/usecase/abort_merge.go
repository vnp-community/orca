package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type AbortMergeInput struct {
	WorktreeID string
}

type AbortMerge struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewAbortMerge(resolver ConnectionResolver, local, relay GitExecutor) *AbortMerge {
	return &AbortMerge{resolver: resolver, local: local, relay: relay}
}

func (uc *AbortMerge) Execute(ctx context.Context, in AbortMergeInput) (domain.SimpleResult, error) {
	if in.WorktreeID == "" {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.AbortMerge(ctx, repoPath)
	if err != nil {
		return domain.SimpleResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_ABORT_MERGE_FAILED", "failed to abort merge", err)
	}
	return result, nil
}
