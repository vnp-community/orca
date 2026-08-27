package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// RevokeCredentialByOwnerInput mirrors RevokeCredentialByOwnerRequest 1:1.
// See credentialbroker.proto's doc comment on that RPC for why this lookup
// key (tenant_id, category, owner_id) exists alongside RevokeCredential's
// by-id lookup.
type RevokeCredentialByOwnerInput struct {
	TenantID          string
	Category          domain.Category
	OwnerID           string
	RequestingService string
}

// RevokeCredentialByOwner is RevokeCredential's sibling — same Vault-revoke-
// then-audited-status-transition logic (see RevokeCredential's doc comment),
// keyed by owner instead of by id. Shares CredentialMetadataRepository.
// GetByOwner with ResolveCredentialByOwner.Execute (resolve_credential_by_owner.go)
// so the two never drift on lookup semantics.
//
// Idempotency decision: unlike RevokeCredential (by id), which can still
// fetch an already-revoked row via Get and short-circuits to an idempotent
// success, this usecase can never observe "already revoked" as a distinct
// state — GetByOwner filters revoked rows out at the SQL level (documented
// on that method and exercised by
// TestResolveCredentialByOwner_RevokedIsNotFound), so revoking an
// already-revoked or never-existent owner-keyed credential both surface as
// plain CREDENTIAL_NOT_FOUND, the same as ResolveCredentialByOwner's
// not-found case. This is intentionally NOT the "return the already-revoked
// metadata, no error" idempotency RevokeCredential offers by id — there is
// no row left to return once revoked-filtering removes it from GetByOwner's
// result set, so a caller that revokes-by-owner twice gets NotFound the
// second time, not a silent success. Callers that need true revoke-is-a-
// no-op idempotency should use RevokeCredential by id instead.
type RevokeCredentialByOwner struct {
	metadataRepo CredentialMetadataRepository
	store        SecretStore
	txRunner     TxRunner
	now          func() time.Time
}

func NewRevokeCredentialByOwner(metadataRepo CredentialMetadataRepository, store SecretStore, txRunner TxRunner) *RevokeCredentialByOwner {
	return &RevokeCredentialByOwner{metadataRepo: metadataRepo, store: store, txRunner: txRunner, now: time.Now}
}

func (uc *RevokeCredentialByOwner) Execute(ctx context.Context, in RevokeCredentialByOwnerInput) (domain.CredentialMetadata, error) {
	if in.TenantID == "" || in.OwnerID == "" {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id and owner_id are required", nil)
	}
	if !in.Category.Valid() {
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_INVALID_CATEGORY", "unknown credential category", nil)
	}

	metadata, err := uc.metadataRepo.GetByOwner(ctx, in.TenantID, in.Category, in.OwnerID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			return domain.CredentialMetadata{}, apperrors.New(apperrors.KindNotFound, "CREDENTIAL_NOT_FOUND", "no credential found for this tenant/category/owner", err)
		}
		return domain.CredentialMetadata{}, apperrors.New(apperrors.KindInternal, "CREDENTIAL_FETCH_FAILED", "failed to fetch credential metadata", err)
	}

	// Invalidate the Vault-side material FIRST — same ordering rationale as
	// RevokeCredential.Execute: if this fails, the metadata row must stay
	// non-revoked so a retry is still meaningful.
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
