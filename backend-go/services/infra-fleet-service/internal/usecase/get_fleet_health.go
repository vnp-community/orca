package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// GetFleetHealth reads the latest fleet-health samples for the caller's
// tenant. The 30s-cadence poller that writes these samples (see
// specs/backend-go/services/infra-fleet-service.md §8) is not implemented in
// this scaffold — see this service's README "Known gaps".
type GetFleetHealth struct {
	health FleetHealthPort
}

func NewGetFleetHealth(health FleetHealthPort) *GetFleetHealth {
	return &GetFleetHealth{health: health}
}

func (uc *GetFleetHealth) Execute(ctx context.Context) ([]domain.DevServerHealth, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	statuses, err := uc.health.GetFleetHealth(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_FLEET_HEALTH_FAILED", "failed to fetch fleet health", err)
	}
	return statuses, nil
}
