package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

type ListAccountsInput struct {
	DevServerID string
}

// ListAccounts is a thin translation layer over
// ProviderAccountRepository.List — the repo method and filter already
// existed (ports.go), this usecase just adds tenant enforcement.
type ListAccounts struct {
	repo ProviderAccountRepository
}

func NewListAccounts(repo ProviderAccountRepository) *ListAccounts {
	return &ListAccounts{repo: repo}
}

func (uc *ListAccounts) Execute(ctx context.Context, in ListAccountsInput) ([]domain.ProviderAccount, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	return uc.repo.List(ctx, ListAccountsFilter{TenantID: tenantID, DevServerID: in.DevServerID})
}
