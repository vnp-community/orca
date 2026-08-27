package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListReposInput struct {
	ProjectID string
}

type ListRepos struct {
	repo       RepoRepository
	membership MembershipRepository
	opa        OPAClient
}

func NewListRepos(repo RepoRepository, membership MembershipRepository, opa OPAClient) *ListRepos {
	return &ListRepos{repo: repo, membership: membership, opa: opa}
}

// Execute requires any membership (owner/member/viewer) in the project, or
// global admin — project-service.md §9's "any membership" tier.
func (uc *ListRepos) Execute(ctx context.Context, in ListReposInput) ([]domain.Repo, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.membership, uc.opa, in.ProjectID, projectActionAnyMember); err != nil {
		return nil, err
	}

	repos, err := uc.repo.ListRepos(ctx, in.ProjectID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_REPOS_FAILED", "failed to list repos", err)
	}
	return repos, nil
}
