package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListDevServerGroups returns every dev-server group in the caller's
// tenant — same tenant-only gate as ListDevServers (no per-group membership
// check exists yet; CR-DS-007 adds the department/team grant model that a
// future ListDevServerGroupsForUser would filter against).
type ListDevServerGroups struct {
	repo DevServerGroupRepository
}

func NewListDevServerGroups(repo DevServerGroupRepository) *ListDevServerGroups {
	return &ListDevServerGroups{repo: repo}
}

func (uc *ListDevServerGroups) Execute(ctx context.Context) ([]domain.DevServerGroup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	groups, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVER_GROUPS_FAILED", "failed to list dev server groups", err)
	}
	return groups, nil
}
