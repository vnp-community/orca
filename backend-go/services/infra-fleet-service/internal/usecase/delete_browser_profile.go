package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
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
	if err := uc.repo.Delete(ctx, tenantID, id); err != nil {
		if errors.Is(err, domain.ErrBrowserProfileNotFound) {
			return apperrors.New(apperrors.KindNotFound, "INFRA_BROWSER_PROFILE_NOT_FOUND", "browser profile not found", err)
		}
		return apperrors.New(apperrors.KindInternal, "INFRA_DELETE_BROWSER_PROFILE_FAILED", "failed to delete browser profile", err)
	}
	return nil
}
