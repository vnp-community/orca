package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type CreateProjectGroupInput struct {
	Name string
	// ParentGroupID is optional — empty means this group is a root of its
	// own tree.
	ParentGroupID string
}

type CreateProjectGroup struct {
	repo ProjectGroupRepository
}

func NewCreateProjectGroup(repo ProjectGroupRepository) *CreateProjectGroup {
	return &CreateProjectGroup{repo: repo}
}

func (uc *CreateProjectGroup) Execute(ctx context.Context, in CreateProjectGroupInput) (domain.ProjectGroup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	group, err := domain.NewProjectGroup(uuid.NewString(), tenantID, in.Name, in.ParentGroupID)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_GROUP_INVALID", err.Error(), err)
	}

	// Explicit existence check (rather than relying on the FK constraint
	// alone) so an unknown parent surfaces as a clean KindFailedPrecondition
	// — mirrors workflow-service's CreateTemplate/ParentTemplateID
	// convention (see that usecase for the identical pattern).
	if group.ParentGroupID != "" {
		if _, err := uc.repo.GetProjectGroup(ctx, tenantID, group.ParentGroupID); err != nil {
			if errors.Is(err, domain.ErrProjectGroupNotFound) {
				return domain.ProjectGroup{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_GROUP_PARENT_NOT_FOUND", "parent group does not exist", err)
			}
			return domain.ProjectGroup{}, apperrors.New(apperrors.KindInternal, "PROJECT_GROUP_FETCH_FAILED", "failed to fetch parent group", err)
		}
	}

	created, err := uc.repo.CreateProjectGroup(ctx, group)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInternal, "PROJECT_GROUP_CREATE_FAILED", "failed to persist project group", err)
	}
	return created, nil
}
