package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateHostSetupInput struct {
	ID    string
	Patch domain.HostSetupPatch
}

type UpdateHostSetup struct {
	repo HostSetupRepository
}

func NewUpdateHostSetup(repo HostSetupRepository) *UpdateHostSetup {
	return &UpdateHostSetup{repo: repo}
}

func (uc *UpdateHostSetup) Execute(ctx context.Context, in UpdateHostSetupInput) (domain.HostSetup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	setup, err := uc.repo.Update(ctx, tenantID, in.ID, in.Patch)
	if err != nil {
		if errors.Is(err, domain.ErrHostSetupNotFound) {
			return domain.HostSetup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_HOST_SETUP_NOT_FOUND", "host setup does not exist", err)
		}
		return domain.HostSetup{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_HOST_SETUP_FAILED", "failed to update host setup", err)
	}
	return setup, nil
}
