package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// GetCredentialMetadataInput mirrors GetCredentialMetadataRequest 1:1.
type GetCredentialMetadataInput struct {
	CredentialID string
}

// GetCredentialMetadata is a pure metadata read — no Vault call, no
// plaintext, no audit row (unlike ResolveCredential/ResolveCredentialByOwner,
// which touch Vault and are audited by design; a metadata-only read never
// exposes secret material, so there is nothing security-sensitive to
// audit here). Added for Epic B so ai-provider-service's
// CredentialBrokerClient.ResolveCredential port — which by contract must
// NEVER see plaintext (see that service's grpcclient package doc comment)
// — has a real RPC to call instead of a stub. See
// credentialbroker.proto's doc comment on this RPC.
type GetCredentialMetadata struct {
	metadataRepo CredentialMetadataRepository
}

func NewGetCredentialMetadata(metadataRepo CredentialMetadataRepository) *GetCredentialMetadata {
	return &GetCredentialMetadata{metadataRepo: metadataRepo}
}

func (uc *GetCredentialMetadata) Execute(ctx context.Context, in GetCredentialMetadataInput) (domain.CredentialMetadata, error) {
	if in.CredentialID == "" {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_ID", "credential_id is required", nil)
	}
	metadata, err := uc.metadataRepo.Get(ctx, in.CredentialID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			return domain.CredentialMetadata{}, apperrors.New(apperrors.KindNotFound, "CREDENTIAL_NOT_FOUND", "credential not found", err)
		}
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_FETCH_FAILED", "failed to fetch credential metadata", err)
	}
	return metadata, nil
}
