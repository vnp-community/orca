package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type SetWorktreeLineageInput struct {
	WorktreeID string
	// ParentWorktreeID is nil to clear the parent, else the new parent's
	// id — the caller (channels_worktree.go's worktree.set) resolves
	// "clear" (noParent:true) vs "set" (parentWorktree present) before
	// reaching this input, so this usecase itself takes a single
	// unambiguous pointer rather than two separate flags.
	ParentWorktreeID *string
}

// SetWorktreeLineage re-parents (or clears the parent of) an
// already-created worktree — worktrees.ts's setWorktreeLineageForRuntime,
// distinct from RecordWorktreeCreated's creation-time lineage capture.
type SetWorktreeLineage struct {
	repo WorktreeRepository
}

func NewSetWorktreeLineage(repo WorktreeRepository) *SetWorktreeLineage {
	return &SetWorktreeLineage{repo: repo}
}

func (uc *SetWorktreeLineage) Execute(ctx context.Context, in SetWorktreeLineageInput) (domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if in.WorktreeID == "" {
		return domain.Worktree{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_WORKTREE_ID_REQUIRED", "worktree_id is required", nil)
	}

	updated, err := uc.repo.SetWorktreeLineage(ctx, in.WorktreeID, in.ParentWorktreeID)
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		return domain.Worktree{}, apperrors.New(apperrors.KindNotFound, "PROJECT_WORKTREE_NOT_FOUND", "worktree not found", err)
	}
	if err != nil {
		return domain.Worktree{}, apperrors.New(apperrors.KindInternal, "PROJECT_SET_WORKTREE_LINEAGE_FAILED", "failed to set worktree lineage", err)
	}
	return updated, nil
}
