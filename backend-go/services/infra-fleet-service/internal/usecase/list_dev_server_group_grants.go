package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListDevServerGroupGrants admin-gated (viewing the grant list is an admin
// console feature) — groupID empty means "every grant in the tenant".
type ListDevServerGroupGrants struct {
	repo DevServerGroupGrantRepository
}

func NewListDevServerGroupGrants(repo DevServerGroupGrantRepository) *ListDevServerGroupGrants {
	return &ListDevServerGroupGrants{repo: repo}
}

func (uc *ListDevServerGroupGrants) Execute(ctx context.Context, groupID string) ([]domain.DevServerGroupGrant, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}

	if groupID == "" {
		grants, err := uc.repo.ListAll(ctx, tenantID)
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_GRANTS_FAILED", "failed to list grants", err)
		}
		return grants, nil
	}
	grants, err := uc.repo.ListByGroup(ctx, tenantID, groupID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_GRANTS_FAILED", "failed to list grants", err)
	}
	return grants, nil
}
