package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// BranchDiffInput's FilePath is required (same per-file fix as CommitDiff)
// and BaseRef is the single ref HEAD is compared against (same
// one-ref-not-two fix as BranchCompare) — see GitExecutor.BranchDiff's doc
// comment.
type BranchDiffInput struct {
	WorktreeID string
	BaseRef    string
	FilePath   string
	OldPath    string
}

type BranchDiff struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewBranchDiff(resolver ConnectionResolver, local, relay GitExecutor) *BranchDiff {
	return &BranchDiff{resolver: resolver, local: local, relay: relay}
}

func (uc *BranchDiff) Execute(ctx context.Context, in BranchDiffInput) (domain.FileDiffResult, error) {
	if in.WorktreeID == "" {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.BaseRef == "" {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_BASE_REF", "base_ref is required", nil)
	}
	if in.FilePath == "" {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_FILE_PATH", "file_path is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.BranchDiff(ctx, repoPath, in.BaseRef, in.FilePath, in.OldPath)
	if err != nil {
		return domain.FileDiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_BRANCH_DIFF_FAILED", "failed to diff branch file", err)
	}
	return result, nil
}
