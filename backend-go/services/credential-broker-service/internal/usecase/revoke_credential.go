package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// RevokeCredentialInput mirrors RevokeCredentialRequest 1:1.
type RevokeCredentialInput struct {
	CredentialID      string
	RequestingService string
}

// RevokeCredential revokes a credential in BOTH Vault and this service's
// own metadata — per credential-broker-service.md §9 ("Immediate revocation
// without a deploy"), this is what makes any subsequent ResolveCredential
// call for this credential fail from this point forward, with no code or
// data deploy needed. The status update and its audit row are written
// together inside one TxRunner.RunInTx call, per credential-broker-service.md
// §8 — see TxRunner's doc comment.
type RevokeCredential struct {
	metadataRepo CredentialMetadataRepository
	store        SecretStore
	txRunner     TxRunner
	now          func() time.Time
}

func NewRevokeCredential(metadataRepo CredentialMetadataRepository, store SecretStore, txRunner TxRunner) *RevokeCredential {
	return &RevokeCredential{metadataRepo: metadataRepo, store: store, txRunner: txRunner, now: time.Now}
}

func (uc *RevokeCredential) Execute(ctx context.Context, in RevokeCredentialInput) (domain.CredentialMetadata, error) {
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
		// Idempotent: revoking an already-revoked credential is a no-op
		// success, not an error — matches the general idempotency posture
		// of this service's write paths (see WriteCredential's
		// RequestID-free equivalent reasoning in usage-service.md, applied
		// here to a terminal-state transition instead of an insert).
		return metadata, nil
	}

	// Invalidate the Vault-side material FIRST — if this fails, the
	// metadata row must stay non-revoked so a retry is still meaningful
	// (an operator or caller can safely retry RevokeCredential).
	if err := uc.store.RevokeSecret(ctx, kvMount, metadata.VaultPath); err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_REVOKE_FAILED", "failed to revoke credential material in vault", err)
	}

	now := uc.now()
	if err := uc.txRunner.RunInTx(ctx, func(ctx context.Context, metadataRepo CredentialMetadataRepository, auditRepo AuditRepository) error {
		if err := metadataRepo.UpdateStatus(ctx, metadata.ID, domain.StatusRevoked, now); err != nil {
			return apperrors.New(apperrors.KindInternal, "CREDENTIAL_UPDATE_FAILED", "failed to mark credential revoked", err)
		}
		return appendAudit(ctx, auditRepo, metadata.ID, in.RequestingService, domain.ActionRevoke, now)
	}); err != nil {
		return domain.CredentialMetadata{}, err
	}
	metadata = metadata.Revoke(now)

	return metadata, nil
}
