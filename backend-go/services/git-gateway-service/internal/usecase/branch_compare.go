package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// BranchCompareInput is current HEAD vs. ONE BaseRef — not two arbitrary
// branches, see GitExecutor.BranchCompare's doc comment (TASK-209's
// Contract correction).
type BranchCompareInput struct {
	WorktreeID string
	BaseRef    string
}

type BranchCompare struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewBranchCompare(resolver ConnectionResolver, local, relay GitExecutor) *BranchCompare {
	return &BranchCompare{resolver: resolver, local: local, relay: relay}
}

func (uc *BranchCompare) Execute(ctx context.Context, in BranchCompareInput) (domain.BranchCompareResult, error) {
	if in.WorktreeID == "" {
		return domain.BranchCompareResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.BaseRef == "" {
		return domain.BranchCompareResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_BASE_REF", "base_ref is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.BranchCompareResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.BranchCompare(ctx, repoPath, in.BaseRef)
	if err != nil {
		return domain.BranchCompareResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_BRANCH_COMPARE_FAILED", "failed to compare branch", err)
	}
	return result, nil
}
