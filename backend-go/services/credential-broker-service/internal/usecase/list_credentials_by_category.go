package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

type ListCredentialsByCategoryInput struct {
	TenantID string
	Category domain.Category
}

type ListCredentialsByCategory struct {
	metadataRepo CredentialMetadataRepository
}

func NewListCredentialsByCategory(metadataRepo CredentialMetadataRepository) *ListCredentialsByCategory {
	return &ListCredentialsByCategory{metadataRepo: metadataRepo}
}

func (uc *ListCredentialsByCategory) Execute(ctx context.Context, in ListCredentialsByCategoryInput) ([]domain.CredentialMetadata, error) {
	if in.TenantID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id is required", nil)
	}
	rows, err := uc.metadataRepo.ListByCategory(ctx, in.TenantID, in.Category)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "CREDENTIAL_LIST_FAILED", "failed to list credentials", err)
	}
	return rows, nil
}
