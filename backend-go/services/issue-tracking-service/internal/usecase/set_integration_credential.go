package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// SetIntegrationCredentialInput mirrors SetIntegrationCredentialRequest 1:1.
type SetIntegrationCredentialInput struct {
	TenantID   string
	Provider   domain.Provider
	Token      string
	ConfigJSON string
}

// SetIntegrationCredential is credentials.set's entry point — a manually
// pasted Jira/Linear token, written via CredentialWriter.WriteRaw under the
// provider-name-only owner_id convention (distinct from CredentialResolver's
// per-user "<userID>:<provider>" owner_id used by the Connect flow — see
// internal/adapter/credential/client.go's package doc comment).
type SetIntegrationCredential struct {
	writer CredentialWriter
}

func NewSetIntegrationCredential(writer CredentialWriter) *SetIntegrationCredential {
	return &SetIntegrationCredential{writer: writer}
}

func (uc *SetIntegrationCredential) Execute(ctx context.Context, in SetIntegrationCredentialInput) error {
	if in.TenantID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Token == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "ISSUETRACKING_NO_TOKEN", "token is required", nil)
	}
	if err := uc.writer.WriteRaw(ctx, in.TenantID, in.Provider, in.Token, in.ConfigJSON); err != nil {
		return apperrors.New(apperrors.KindInternal, "ISSUETRACKING_CREDENTIAL_WRITE_FAILED", "failed to write credential via credential-broker-service", err)
	}
	return nil
}
