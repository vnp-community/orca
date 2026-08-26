package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListBrowserProfiles lists the browser profiles registered for one dev
// server, scoped to the caller's tenant — see SOL-006 Group C.
type ListBrowserProfiles struct {
	repo BrowserProfileRepository
}

func NewListBrowserProfiles(repo BrowserProfileRepository) *ListBrowserProfiles {
	return &ListBrowserProfiles{repo: repo}
}

func (uc *ListBrowserProfiles) Execute(ctx context.Context, devServerID string) ([]domain.BrowserProfile, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if devServerID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_DEV_SERVER", "dev_server_id is required", nil)
	}
	return uc.repo.List(ctx, tenantID, devServerID)
}
