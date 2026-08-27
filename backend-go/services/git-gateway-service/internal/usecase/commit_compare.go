package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// CommitCompareInput's CommitID must be a full 40/64-hex git object id — see
// GitExecutor.CommitCompare's doc comment. Real op is one commit vs. its own
// parent, not two arbitrary commits (TASK-209's Contract correction).
type CommitCompareInput struct {
	WorktreeID string
	CommitID   string
}

type CommitCompare struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCommitCompare(resolver ConnectionResolver, local, relay GitExecutor) *CommitCompare {
	return &CommitCompare{resolver: resolver, local: local, relay: relay}
}

func (uc *CommitCompare) Execute(ctx context.Context, in CommitCompareInput) (domain.CommitCompareResult, error) {
	if in.WorktreeID == "" {
		return domain.CommitCompareResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.CommitID == "" {
		return domain.CommitCompareResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_COMMIT_ID", "commit_id is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.CommitCompareResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.CommitCompare(ctx, repoPath, in.CommitID)
	if err != nil {
		return domain.CommitCompareResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_COMMIT_COMPARE_FAILED", "failed to compare commit", err)
	}
	return result, nil
}
