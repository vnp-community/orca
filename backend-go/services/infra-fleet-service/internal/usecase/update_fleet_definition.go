package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type UpdateFleetDefinitionInput struct {
	ID        string
	Servers   []domain.FleetSpecServer
	Provision *domain.ProvisionConfig
}

// UpdateFleetDefinition replaces a FleetDefinition's Servers/Provision and
// bumps its version — optimistic locking (TASK-BE-FLEET-012's chosen
// concurrency-control strategy: CR-FLEET-003 didn't specify one, and 2
// clients silently clobbering each other's edit is worse than a caller
// re-fetching after a conflict). The caller must read-then-write: this
// usecase reads the current row first, so the version it locks on is
// always the one it (or a concurrent writer) most recently saw.
type UpdateFleetDefinition struct {
	repo FleetDefinitionRepository
}

func NewUpdateFleetDefinition(repo FleetDefinitionRepository) *UpdateFleetDefinition {
	return &UpdateFleetDefinition{repo: repo}
}

func (uc *UpdateFleetDefinition) Execute(ctx context.Context, in UpdateFleetDefinitionInput) (domain.FleetDefinition, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	current, err := uc.repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		if err == domain.ErrFleetDefinitionNotFound {
			return domain.FleetDefinition{}, apperrors.New(apperrors.KindNotFound, "INFRA_FLEET_DEFINITION_NOT_FOUND", "fleet definition not found", err)
		}
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInternal, "INFRA_GET_FLEET_DEFINITION_FAILED", "failed to get fleet definition", err)
	}

	if len(in.Servers) == 0 {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_FLEET_DEFINITION", domain.ErrEmptyFleetDefinitionServers.Error(), domain.ErrEmptyFleetDefinitionServers)
	}

	next := current
	next.Servers = in.Servers
	next.Provision = in.Provision
	next.Version = current.Version + 1

	saved, err := uc.repo.Update(ctx, next)
	if err != nil {
		if err == domain.ErrFleetDefinitionVersionConflict {
			return domain.FleetDefinition{}, apperrors.New(apperrors.KindFailedPrecondition, "INFRA_FLEET_DEFINITION_VERSION_CONFLICT", "fleet definition was updated concurrently, refetch and retry", err)
		}
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInternal, "INFRA_UPDATE_FLEET_DEFINITION_FAILED", "failed to update fleet definition", err)
	}
	return saved, nil
}
