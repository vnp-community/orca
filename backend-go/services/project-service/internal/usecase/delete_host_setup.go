package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type DeleteHostSetupInput struct {
	ID string
}

type DeleteHostSetup struct {
	repo HostSetupRepository
}

func NewDeleteHostSetup(repo HostSetupRepository) *DeleteHostSetup {
	return &DeleteHostSetup{repo: repo}
}

func (uc *DeleteHostSetup) Execute(ctx context.Context, in DeleteHostSetupInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.repo.Delete(ctx, tenantID, in.ID); err != nil {
		if errors.Is(err, domain.ErrHostSetupNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_HOST_SETUP_NOT_FOUND", "host setup does not exist", err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_DELETE_HOST_SETUP_FAILED", "failed to delete host setup", err)
	}
	return nil
}
