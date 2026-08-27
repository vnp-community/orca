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
// active. The status update and its audit row are written together inside
// one TxRunner.RunInTx call, per credential-broker-service.md §8 — see
// TxRunner's doc comment. The initial metadataRepo.Get lookup runs outside
// that transaction: it's a read used only to validate the request, not part
// of the mutation that needs atomicity with the audit row.
type RotateCredential struct {
	metadataRepo CredentialMetadataRepository
	store        SecretStore
	txRunner     TxRunner
	now          func() time.Time
}

func NewRotateCredential(metadataRepo CredentialMetadataRepository, store SecretStore, txRunner TxRunner) *RotateCredential {
	return &RotateCredential{metadataRepo: metadataRepo, store: store, txRunner: txRunner, now: time.Now}
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
	if err := uc.txRunner.RunInTx(ctx, func(ctx context.Context, metadataRepo CredentialMetadataRepository, auditRepo AuditRepository) error {
		if err := metadataRepo.UpdateStatus(ctx, metadata.ID, domain.StatusActive, now); err != nil {
			return apperrors.New(apperrors.KindInternal, "CREDENTIAL_UPDATE_FAILED", "failed to update credential status after rotation", err)
		}
		return appendAudit(ctx, auditRepo, metadata.ID, in.RequestingService, domain.ActionRotate, now)
	}); err != nil {
		return domain.CredentialMetadata{}, err
	}
	metadata = metadata.WithStatus(domain.StatusActive, now)

	return metadata, nil
}
