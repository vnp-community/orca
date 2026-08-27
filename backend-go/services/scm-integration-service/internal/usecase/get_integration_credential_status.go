package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type GetIntegrationCredentialStatusInput struct {
	TenantID string
	Provider domain.ScmProvider
}

type GetIntegrationCredentialStatusResult struct {
	Configured bool
	ConfigJSON string
}

type GetIntegrationCredentialStatus struct {
	reader CredentialStatusReader
}

func NewGetIntegrationCredentialStatus(reader CredentialStatusReader) *GetIntegrationCredentialStatus {
	return &GetIntegrationCredentialStatus{reader: reader}
}

func (uc *GetIntegrationCredentialStatus) Execute(ctx context.Context, in GetIntegrationCredentialStatusInput) (GetIntegrationCredentialStatusResult, error) {
	if in.TenantID == "" {
		return GetIntegrationCredentialStatusResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	configured, configJSON, err := uc.reader.GetStatus(ctx, in.TenantID, in.Provider)
	if err != nil {
		return GetIntegrationCredentialStatusResult{}, apperrors.New(apperrors.KindInternal, "SCM_STATUS_FETCH_FAILED", "failed to fetch credential status", err)
	}
	return GetIntegrationCredentialStatusResult{Configured: configured, ConfigJSON: configJSON}, nil
}
