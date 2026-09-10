package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// GetFleetDefinition fetches one FleetDefinition scoped to the caller's
// tenant. Used directly (not through the RPC) by ExportFleetDefinitionYaml
// (TASK-BE-FLEET-013) and DeployFleetDefinition (TASK-BE-FLEET-014).
type GetFleetDefinition struct {
	repo FleetDefinitionRepository
}

func NewGetFleetDefinition(repo FleetDefinitionRepository) *GetFleetDefinition {
	return &GetFleetDefinition{repo: repo}
}

func (uc *GetFleetDefinition) Execute(ctx context.Context, id string) (domain.FleetDefinition, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	def, err := uc.repo.Get(ctx, tenantID, id)
	if err != nil {
		if err == domain.ErrFleetDefinitionNotFound {
			return domain.FleetDefinition{}, apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
		}
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInternal, "INFRA_GET_FLEET_DEFINITION_FAILED", "failed to get fleet definition", err)
	}
	return def, nil
}
