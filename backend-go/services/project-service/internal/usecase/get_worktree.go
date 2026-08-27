package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// GetWorktree is the new single-worktree lookup RPC (SOL-WT-04) —
// git-gateway-service's CompareWorktrees uses it to fetch each compared
// worktree's repo_id/branch/base_ref.
type GetWorktree struct {
	repo WorktreeRepository
}

func NewGetWorktree(repo WorktreeRepository) *GetWorktree {
	return &GetWorktree{repo: repo}
}

func (uc *GetWorktree) Execute(ctx context.Context, worktreeID string) (domain.Worktree, error) {
	wt, err := uc.repo.GetWorktree(ctx, worktreeID)
	if err != nil {
		if err == domain.ErrWorktreeNotFound {
			return domain.Worktree{}, apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
		}
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_GET_WORKTREE_FAILED", "failed to fetch worktree", err)
	}
	return wt, nil
}
