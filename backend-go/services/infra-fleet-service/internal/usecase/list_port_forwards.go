package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type ListPortForwardsInput struct {
	ConnectionID string
}

// ListPortForwards lists connectionID's currently-active port forwards.
type ListPortForwards struct {
	repo PortForwardRepository
}

func NewListPortForwards(repo PortForwardRepository) *ListPortForwards {
	return &ListPortForwards{repo: repo}
}

func (uc *ListPortForwards) Execute(ctx context.Context, in ListPortForwardsInput) ([]domain.PortForward, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	forwards, err := uc.repo.ListActiveByConnection(ctx, tenantID, in.ConnectionID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_PORT_FORWARDS_FAILED", "failed to list port forwards", err)
	}
	return forwards, nil
}
