package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type ListIntegrationCredentials struct {
	lister CredentialLister
}

func NewListIntegrationCredentials(lister CredentialLister) *ListIntegrationCredentials {
	return &ListIntegrationCredentials{lister: lister}
}

func (uc *ListIntegrationCredentials) Execute(ctx context.Context, tenantID string) ([]domain.ScmProvider, error) {
	if tenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	providers, err := uc.lister.ListConfiguredProviders(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "SCM_LIST_FAILED", "failed to list configured providers", err)
	}
	return providers, nil
}
