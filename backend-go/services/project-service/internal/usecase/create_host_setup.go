package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type CreateHostSetupInput struct {
	DevServerID string
	FolderPath  string
	DisplayName string
}

type CreateHostSetup struct {
	repo       HostSetupRepository
	devServers DevServerLister
}

func NewCreateHostSetup(repo HostSetupRepository, devServers DevServerLister) *CreateHostSetup {
	return &CreateHostSetup{repo: repo, devServers: devServers}
}

func (uc *CreateHostSetup) Execute(ctx context.Context, in CreateHostSetupInput) (domain.HostSetup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.HostSetup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	exists, err := uc.devServers.Exists(ctx, tenantID, in.DevServerID)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
	}
	if !exists {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
	}

	setup, err := domain.NewHostSetup(uuid.NewString(), tenantID, in.DevServerID, in.FolderPath, in.DisplayName, userID)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_HOST_SETUP", err.Error(), err)
	}

	created, err := uc.repo.Create(ctx, setup)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_HOST_SETUP_FAILED", "failed to persist host setup", err)
	}
	return created, nil
}
