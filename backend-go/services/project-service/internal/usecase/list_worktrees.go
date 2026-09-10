package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListWorktreesInput struct {
	ProjectID string
	// StatusIn/OlderThan are optional filters — BL-AT-04's
	// cleanup_worktrees step candidate query. Both unset = every existing
	// caller's unfiltered behavior.
	StatusIn  []string
	OlderThan *time.Time
}

type ListWorktrees struct {
	repo       WorktreeRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewListWorktrees(repo WorktreeRepository, membership MembershipRepository, opa OPAClient) *ListWorktrees {
	return &ListWorktrees{repo: repo, membership: membership, opa: opa}
}

// Execute requires any membership (owner/member/viewer) in the project, or
// global admin — project-service.md §9's "any membership" tier.
func (uc *ListWorktrees) Execute(ctx context.Context, in ListWorktreesInput) ([]domain.Worktree, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ProjectID, projectActionAnyMember); err != nil {
		return nil, err
	}

	worktrees, err := uc.repo.ListWorktrees(ctx, in.ProjectID, in.StatusIn, in.OlderThan)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_WORKTREES_FAILED", "failed to list worktrees", err)
	}
	return worktrees, nil
}
