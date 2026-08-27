package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// DeleteBrowserProfile removes browser profile metadata, tenant-scoped —
// see SOL-006 Group C.
type DeleteBrowserProfile struct {
	repo BrowserProfileRepository
}

func NewDeleteBrowserProfile(repo BrowserProfileRepository) *DeleteBrowserProfile {
	return &DeleteBrowserProfile{repo: repo}
}

func (uc *DeleteBrowserProfile) Execute(ctx context.Context, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if id == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "INFRA_NO_BROWSER_PROFILE_ID", "id is required", nil)
	}
	return uc.repo.Delete(ctx, tenantID, id)
}
