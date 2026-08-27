package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListDevServers exposes DevServerRepository.List over gRPC — the repository
// method already existed (used nowhere else) but no usecase or RPC ever
// called it, so the frontend's `devServer.list` channel had nothing to
// wire to. See docs/execution-plan.md §2 Epic A.
type ListDevServers struct {
	repo DevServerRepository
}

func NewListDevServers(repo DevServerRepository) *ListDevServers {
	return &ListDevServers{repo: repo}
}

func (uc *ListDevServers) Execute(ctx context.Context) ([]domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	devServers, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVERS_FAILED", "failed to list dev servers", err)
	}
	return devServers, nil
}
