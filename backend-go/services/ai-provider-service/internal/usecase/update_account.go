package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

// UpdateAccount mutates Label/ModelHint/BaseURL only — guards against
// becoming a second path to mutate lifecycle state, which must only ever
// go through UpdateStatus (see ports.go's UpdateStatusInput doc comment).
type UpdateAccount struct {
	repo ProviderAccountRepository
}

func NewUpdateAccount(repo ProviderAccountRepository) *UpdateAccount {
	return &UpdateAccount{repo: repo}
}

func (uc *UpdateAccount) Execute(ctx context.Context, in UpdateFields) (domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	if in.AccountID == "" {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInvalidArgument, "AIPROVIDER_NO_ACCOUNT_ID", "account_id is required", nil)
	}
	in.TenantID = tenantID
	account, err := uc.repo.Update(ctx, in)
	if err != nil {
		return domain.ProviderAccount{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_UPDATE_FAILED", "failed to update provider account", err)
	}
	return account, nil
}
