package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListHostSetups struct {
	repo HostSetupRepository
}

func NewListHostSetups(repo HostSetupRepository) *ListHostSetups {
	return &ListHostSetups{repo: repo}
}

func (uc *ListHostSetups) Execute(ctx context.Context) ([]domain.HostSetup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	setups, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_HOST_SETUPS_FAILED", "failed to list host setups", err)
	}
	return setups, nil
}
