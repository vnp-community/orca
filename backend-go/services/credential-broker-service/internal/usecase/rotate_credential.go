package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// RotateCredentialInput mirrors RotateCredentialRequest 1:1.
// NewEncryptedEnvelope gets the same opaque-bytes handling as
// WriteCredentialInput.EncryptedEnvelope — see write_credential.go's doc
// comment.
type RotateCredentialInput struct {
	CredentialID         string
	NewEncryptedEnvelope []byte
	RequestingService    string
}

// RotateCredential re-encrypts new material under the SAME Vault path an
// existing credential already points at, then flips its status back to
// active.
type RotateCredential struct {
	metadataRepo CredentialMetadataRepository
	auditRepo    AuditRepository
	store        SecretStore
	now          func() time.Time
}

func NewRotateCredential(metadataRepo CredentialMetadataRepository, auditRepo AuditRepository, store SecretStore) *RotateCredential {
	return &RotateCredential{metadataRepo: metadataRepo, auditRepo: auditRepo, store: store, now: time.Now}
}

func (uc *RotateCredential) Execute(ctx context.Context, in RotateCredentialInput) (domain.CredentialMetadata, error) {
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
	if metadata.IsRevoked() {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindFailedPrecondition, "CREDENTIAL_REVOKED", "cannot rotate a revoked credential", domain.ErrCredentialRevoked)
	}

	ciphertext, err := uc.store.TransitEncrypt(ctx, transitKeyName(string(metadata.Category)), in.NewEncryptedEnvelope)
	if err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_ENCRYPT_FAILED", "failed to encrypt rotated credential material", err)
	}
	if err := uc.store.KVWrite(ctx, kvMount, metadata.VaultPath, map[string]any{"ciphertext": ciphertext}); err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_WRITE_FAILED", "failed to persist rotated credential ciphertext", err)
	}

	now := uc.now()
	if err := uc.metadataRepo.UpdateStatus(ctx, metadata.ID, domain.StatusActive, now); err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_UPDATE_FAILED", "failed to update credential status after rotation", err)
	}
	metadata = metadata.WithStatus(domain.StatusActive, now)

	if err := appendAudit(ctx, uc.auditRepo, metadata.ID, in.RequestingService, domain.ActionRotate, now); err != nil {
		return domain.CredentialMetadata{}, err
	}

	return metadata, nil
}
