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
	// Description/DefaultBranch/Visibility are all optional — empty
	// DefaultBranch/Visibility fall back to domain.DefaultBranch/
	// domain.DefaultVisibility, matching the DB column defaults.
	Description   string
	DefaultBranch string
	Visibility    string
}

// CreateProject is project-service's core write path. TenantID and CreatedBy
// are NOT part of the input struct — both are pulled from context (see
// common/tenant), never trusted from the request body, per
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
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	project, err := domain.NewProject(uuid.NewString(), tenantID, in.Name, "")
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID", err.Error(), err)
	}

	defaultBranch := in.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = domain.DefaultBranch
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = domain.DefaultVisibility
	}
	if !domain.ValidVisibility(visibility) {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY", domain.ErrInvalidVisibility.Error(), domain.ErrInvalidVisibility)
	}

	project.Description = in.Description
	project.DefaultBranch = defaultBranch
	project.Visibility = visibility
	project.CreatedBy = userID

	created, err := uc.repo.Create(ctx, project)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_FAILED", "failed to persist project", err)
	}
	return created, nil
}
