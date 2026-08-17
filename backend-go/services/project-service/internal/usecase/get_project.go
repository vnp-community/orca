package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type GetProjectInput struct {
	ID string
}

type GetProject struct {
	repo ProjectRepository
	opa  OPAClient
}

func NewGetProject(repo ProjectRepository, opa OPAClient) *GetProject {
	return &GetProject{repo: repo, opa: opa}
}

// Execute requires any membership (owner/member/viewer) in the project, or
// global admin — project-service.md §9's "any membership" tier.
func (uc *GetProject) Execute(ctx context.Context, in GetProjectInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ID, projectActionAnyMember); err != nil {
		return domain.Project{}, err
	}

	project, err := uc.repo.Get(ctx, tenantID, in.ID)
	if errors.Is(err, domain.ErrProjectNotFound) {
		return domain.Project{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project not found", err)
	}
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_FETCH_FAILED", "failed to fetch project", err)
	}
	return project, nil
}
