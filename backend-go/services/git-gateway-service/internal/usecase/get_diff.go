package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type GetDiffInput struct {
	WorktreeID string
	FilePath   string // required — real git.diff is a per-file operation, see TASK-228
	Staged     bool
}

// GetDiff resolves the worktree's owning host and returns one file's
// unified diff.
type GetDiff struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewGetDiff(resolver ConnectionResolver, local, relay GitExecutor) *GetDiff {
	return &GetDiff{resolver: resolver, local: local, relay: relay}
}

func (uc *GetDiff) Execute(ctx context.Context, in GetDiffInput) (domain.DiffResult, error) {
	if in.WorktreeID == "" {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.FilePath == "" {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_FILE_PATH", "file_path is required", nil)
	}

	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}

	diff, err := executor.GetDiff(ctx, repoPath, in.FilePath, in.Staged)
	if err != nil {
		return domain.DiffResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_DIFF_FAILED", "failed to get git diff", err)
	}
	return diff, nil
}
