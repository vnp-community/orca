package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type DeleteAccount struct {
	repo ProviderAccountRepository
}

func NewDeleteAccount(repo ProviderAccountRepository) *DeleteAccount {
	return &DeleteAccount{repo: repo}
}

func (uc *DeleteAccount) Execute(ctx context.Context, accountID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	if accountID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_ACCOUNT_ID", "account_id is required", nil)
	}
	if err := uc.repo.Delete(ctx, tenantID, accountID); err != nil {
		return apperrors.New(apperrors.KindInternal, "AIPROVIDER_DELETE_FAILED", "failed to delete provider account", err)
	}
	return nil
}
