package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// ListWorktreeLineage backs worktree.lineageList (wscompat) — tenant-wide,
// unlike ListWorktrees, so there's no single project_id to run the usual
// requireProjectAccess membership check against. The Postgres RLS policy on
// project.worktrees already scopes ListLineage's query to the caller's
// tenant, same as every other WorktreeRepository query — a plain tenant
// auth check (no per-project membership tier) is the right gate here.
type ListWorktreeLineage struct {
	repo WorktreeRepository
}

func NewListWorktreeLineage(repo WorktreeRepository) *ListWorktreeLineage {
	return &ListWorktreeLineage{repo: repo}
}

func (uc *ListWorktreeLineage) Execute(ctx context.Context) ([]domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	worktrees, err := uc.repo.ListLineage(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_WORKTREE_LINEAGE_FAILED", "failed to list worktree lineage", err)
	}
	return worktrees, nil
}
