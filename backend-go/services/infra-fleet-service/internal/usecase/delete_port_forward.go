package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type DeletePortForwardInput struct {
	ID string
}

// DeletePortForward marks a port forward closed (soft-delete via status —
// same pattern TeardownConnection uses for domain.Connection) — idempotent
// on an already-closed forward.
type DeletePortForward struct {
	repo PortForwardRepository
}

func NewDeletePortForward(repo PortForwardRepository) *DeletePortForward {
	return &DeletePortForward{repo: repo}
}

func (uc *DeletePortForward) Execute(ctx context.Context, in DeletePortForwardInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.repo.UpdateStatus(ctx, tenantID, in.ID, domain.PortForwardStatusClosed); err != nil {
		return apperrors.New(apperrors.KindInternal, "INFRA_DELETE_PORT_FORWARD_FAILED", "failed to delete port forward", err)
	}
	return nil
}
