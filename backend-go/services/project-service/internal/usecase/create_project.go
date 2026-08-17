package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type CreateProjectInput struct {
	Name string
}

// CreateProject is project-service's core write path. TenantID is NOT part
// of the input struct — it's pulled from context (see common/tenant), never
// trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule. The creator
// becomes an implicit owner via a follow-up AddMember call by the caller
// (api-gateway) — see project-service.md §9; this usecase only creates the
// project row.
type CreateProject struct {
	repo ProjectRepository
}

func NewCreateProject(repo ProjectRepository) *CreateProject {
	return &CreateProject{repo: repo}
}

func (uc *CreateProject) Execute(ctx context.Context, in CreateProjectInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	project, err := domain.NewProject(uuid.NewString(), tenantID, in.Name, "")
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID", err.Error(), err)
	}

	created, err := uc.repo.Create(ctx, project)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_FAILED", "failed to persist project", err)
	}
	return created, nil
}
