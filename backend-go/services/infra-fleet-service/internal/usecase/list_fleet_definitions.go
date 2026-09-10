package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListFleetDefinitions returns every FleetDefinition registered for the
// caller's tenant.
type ListFleetDefinitions struct {
	repo FleetDefinitionRepository
}

func NewListFleetDefinitions(repo FleetDefinitionRepository) *ListFleetDefinitions {
	return &ListFleetDefinitions{repo: repo}
}

func (uc *ListFleetDefinitions) Execute(ctx context.Context) ([]domain.FleetDefinition, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	defs, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_FLEET_DEFINITIONS_FAILED", "failed to list fleet definitions", err)
	}
	return defs, nil
}
