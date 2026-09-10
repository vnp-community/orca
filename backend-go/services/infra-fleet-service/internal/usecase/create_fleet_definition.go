package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type CreateFleetDefinitionInput struct {
	Name      string
	Servers   []domain.FleetSpecServer
	Provision *domain.ProvisionConfig
}

// CreateFleetDefinition persists a new FleetDefinition — CR-FLEET-003's
// source of truth for a fleet, distinct from the one-shot
// BulkProvisionFleet (CR-FLEET-001) that actually provisions it.
type CreateFleetDefinition struct {
	repo FleetDefinitionRepository
}

func NewCreateFleetDefinition(repo FleetDefinitionRepository) *CreateFleetDefinition {
	return &CreateFleetDefinition{repo: repo}
}

func (uc *CreateFleetDefinition) Execute(ctx context.Context, in CreateFleetDefinitionInput) (domain.FleetDefinition, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	// tenant.UserID, not a RequireUserID — common/tenant.go has no
	// require-variant for user id, only tenant id (verified before writing
	// this usecase).
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_USER", "no user in request context", nil)
	}

	def, err := domain.NewFleetDefinition(uuid.NewString(), tenantID, in.Name, in.Servers, in.Provision, userID)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInvalidArgument, "INFRA_INVALID_FLEET_DEFINITION", err.Error(), err)
	}

	saved, err := uc.repo.Create(ctx, def)
	if err != nil {
		return domain.FleetDefinition{}, apperrors.New(apperrors.KindInternal, "INFRA_CREATE_FLEET_DEFINITION_FAILED", "failed to create fleet definition", err)
	}
	return saved, nil
}
