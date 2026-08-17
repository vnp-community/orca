package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// WriteCredentialInput mirrors WriteCredentialRequest 1:1 — see
// architecture/03's usecase-granularity note.
//
// EncryptedEnvelope handling (credential-broker-service.md §9): the browser
// already transport-encrypts this payload before it reaches this service
// (ADR-008), and for SERVICE_SECRET/SSH categories over the internal mTLS
// mesh there may be no browser leg at all. Real client-side crypto
// integration — negotiating a per-request transport key with the browser
// and decrypting that specific envelope format here — is explicitly out of
// scope for this scaffold (see the design doc's note that this is a
// scaffold-simplification point). Rather than fake a decrypt step, this
// usecase treats EncryptedEnvelope as OPAQUE BYTES end to end: it is never
// decrypted, never referred to as "plaintext" anywhere in this codebase
// (variable names, comments, log fields), and is passed directly into Vault
// Transit as the value to encrypt. Whatever the bytes are — an
// already-transport-encrypted envelope, or raw secret material that only
// ever crossed the mTLS-secured internal mesh — Vault's own encryption is
// what ends up protecting them at rest here, which is honest about what
// this scaffold does and does not do. See this service's README "Known
// gaps" for exactly what a real integration would need to add.
type WriteCredentialInput struct {
	TenantID          string
	OwnerID           string
	Category          domain.Category
	EncryptedEnvelope []byte
	RequestingService string
}

// WriteCredential is credential-broker-service's create path: Transit-
// encrypt the incoming envelope, persist the resulting ciphertext under a
// new Vault KV v2 path, and only THEN create the metadata + audit rows —
// see the doc comment above and credential-broker-service.md §10.
type WriteCredential struct {
	metadataRepo CredentialMetadataRepository
	auditRepo    AuditRepository
	store        SecretStore
	now          func() time.Time
}

func NewWriteCredential(metadataRepo CredentialMetadataRepository, auditRepo AuditRepository, store SecretStore) *WriteCredential {
	return &WriteCredential{metadataRepo: metadataRepo, auditRepo: auditRepo, store: store, now: time.Now}
}

func (uc *WriteCredential) Execute(ctx context.Context, in WriteCredentialInput) (domain.CredentialMetadata, error) {
	if in.TenantID == "" || in.OwnerID == "" {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id and owner_id are required", nil)
	}
	if !in.Category.Valid() {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_INVALID_CATEGORY", "unknown credential category", nil)
	}

	id := uuid.NewString()
	vaultPath := vaultPathFor(in.TenantID, id)

	// Re-encrypt under Vault Transit before the bytes are ever written
	// anywhere durable — see the type doc above for why this is honest even
	// though this scaffold doesn't perform a transport decrypt first.
	ciphertext, err := uc.store.TransitEncrypt(ctx, transitKeyName(string(in.Category)), in.EncryptedEnvelope)
	if err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_ENCRYPT_FAILED", "failed to encrypt credential material", err)
	}
	if err := uc.store.KVWrite(ctx, kvMount, vaultPath, map[string]any{"ciphertext": ciphertext}); err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_WRITE_FAILED", "failed to persist credential ciphertext", err)
	}

	now := uc.now()
	// Only created after the Vault write above is confirmed — see this
	// type's doc comment and credential-broker-service.md §10.
	metadata, err := domain.NewCredentialMetadata(id, in.TenantID, in.OwnerID, in.Category, vaultPath, domain.StatusActive, now)
	if err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_INVALID_METADATA", err.Error(), err)
	}
	if err := uc.metadataRepo.Create(ctx, metadata); err != nil {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_SAVE_FAILED", "failed to persist credential metadata", err)
	}

	if err := appendAudit(ctx, uc.auditRepo, metadata.ID, in.RequestingService, domain.ActionWrite, now); err != nil {
		return domain.CredentialMetadata{}, err
	}

	return metadata, nil
}
