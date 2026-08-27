package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type SetWorktreeActivationInput struct {
	WorktreeID string
	Active     bool
}

type SetWorktreeActivation struct {
	repo WorktreeRepository
}

func NewSetWorktreeActivation(repo WorktreeRepository) *SetWorktreeActivation {
	return &SetWorktreeActivation{repo: repo}
}

func (uc *SetWorktreeActivation) Execute(ctx context.Context, in SetWorktreeActivationInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED", "worktree_id is required", nil)
	}

	updated, err := uc.repo.SetWorktreeActivation(ctx, in.WorktreeID, in.Active)
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		return domain.Worktree{}, apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
	}
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_SET_WORKTREE_ACTIVATION_FAILED", "failed to update worktree activation", err)
	}
	return updated, nil
}
