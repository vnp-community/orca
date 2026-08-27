package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type RecordWorktreeRemovedInput struct {
	WorktreeID string
}

// RecordWorktreeRemoved hard-deletes the worktree row — see
// WorktreeRepository.RecordWorktreeRemoved's doc comment for the
// soft-vs-hard-delete decision.
type RecordWorktreeRemoved struct {
	repo WorktreeRepository
}

func NewRecordWorktreeRemoved(repo WorktreeRepository) *RecordWorktreeRemoved {
	return &RecordWorktreeRemoved{repo: repo}
}

func (uc *RecordWorktreeRemoved) Execute(ctx context.Context, in RecordWorktreeRemovedInput) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED", "worktree_id is required", nil)
	}

	err := uc.repo.RecordWorktreeRemoved(ctx, in.WorktreeID)
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		return apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
	}
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_REMOVE_WORKTREE_FAILED", "failed to remove worktree", err)
	}
	return nil
}
