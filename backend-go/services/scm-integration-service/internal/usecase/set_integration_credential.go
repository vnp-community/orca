package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

type SetIntegrationCredentialInput struct {
	TenantID   string
	Provider   domain.ScmProvider
	Token      string
	ConfigJSON string
}

type SetIntegrationCredential struct {
	writer CredentialWriter
}

func NewSetIntegrationCredential(writer CredentialWriter) *SetIntegrationCredential {
	return &SetIntegrationCredential{writer: writer}
}

func (uc *SetIntegrationCredential) Execute(ctx context.Context, in SetIntegrationCredentialInput) error {
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Token == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TOKEN", "token is required", nil)
	}
	if err := uc.writer.WriteRaw(ctx, in.TenantID, in.Provider, in.Token, in.ConfigJSON); err != nil {
		return apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}
	return nil
}
