package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RenameWorktreeInput struct {
	WorktreeID string
	Branch     string
}

type RenameWorktree struct {
	repo WorktreeRepository
}

func NewRenameWorktree(repo WorktreeRepository) *RenameWorktree {
	return &RenameWorktree{repo: repo}
}

func (uc *RenameWorktree) Execute(ctx context.Context, in RenameWorktreeInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED", "worktree_id is required", nil)
	}
	if in.Branch == "" {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_BRANCH_REQUIRED", domain.ErrEmptyWorktreeBranch.Error(), domain.ErrEmptyWorktreeBranch)
	}

	updated, err := uc.repo.RenameWorktree(ctx, in.WorktreeID, in.Branch)
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		return domain.Worktree{}, apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
	}
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_RENAME_WORKTREE_FAILED", "failed to rename worktree", err)
	}
	return updated, nil
}
