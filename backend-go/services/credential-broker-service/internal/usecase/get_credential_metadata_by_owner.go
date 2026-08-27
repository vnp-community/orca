package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

type GetCredentialMetadataByOwnerInput struct {
	TenantID string
	Category domain.Category
	OwnerID  string
}

// GetCredentialMetadataByOwnerResult wraps the metadata plus a Found flag
// so "no credential exists yet" (Found=false, nil error) is distinguished
// from a real fetch error — GetCredentialMetadataByOwnerResponse.metadata
// is `optional` for exactly this reason (TASK-037).
type GetCredentialMetadataByOwnerResult struct {
	Metadata domain.CredentialMetadata
	Found    bool
}

// GetCredentialMetadataByOwner is a pure metadata read, same
// no-Vault-call/no-audit-row shape as GetCredentialMetadata — see that
// usecase's doc comment. Closes the gap BUG-007 flags: previously the only
// by-owner lookup was ResolveCredentialByOwner, which returns the
// plaintext value — a security mismatch for a status check.
type GetCredentialMetadataByOwner struct {
	metadataRepo CredentialMetadataRepository
}

func NewGetCredentialMetadataByOwner(metadataRepo CredentialMetadataRepository) *GetCredentialMetadataByOwner {
	return &GetCredentialMetadataByOwner{metadataRepo: metadataRepo}
}

func (uc *GetCredentialMetadataByOwner) Execute(ctx context.Context, in GetCredentialMetadataByOwnerInput) (GetCredentialMetadataByOwnerResult, error) {
	if in.TenantID == "" || in.OwnerID == "" {
		return GetCredentialMetadataByOwnerResult{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id and owner_id are required", nil)
	}
	metadata, err := uc.metadataRepo.GetByOwner(ctx, in.TenantID, in.Category, in.OwnerID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			// Not found is the normal "not configured yet" case, not an
			// error — see this type's doc comment.
			return GetCredentialMetadataByOwnerResult{Found: false}, nil
		}
		return GetCredentialMetadataByOwnerResult{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_FETCH_FAILED", "failed to fetch credential metadata", err)
	}
	return GetCredentialMetadataByOwnerResult{Metadata: metadata, Found: true}, nil
}
