package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListProjectGroups struct {
	repo ProjectGroupRepository
}

func NewListProjectGroups(repo ProjectGroupRepository) *ListProjectGroups {
	return &ListProjectGroups{repo: repo}
}

func (uc *ListProjectGroups) Execute(ctx context.Context) ([]domain.ProjectGroup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	groups, err := uc.repo.ListProjectGroups(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_GROUPS_FAILED", "failed to list project groups", err)
	}
	return groups, nil
}
