package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// ResolveCredentialByOwnerInput mirrors ResolveCredentialByOwnerRequest
// 1:1. See credentialbroker.proto's doc comment on that RPC for why this
// lookup key (tenant_id, category, owner_id) exists alongside
// ResolveCredential's by-id lookup.
type ResolveCredentialByOwnerInput struct {
	TenantID          string
	Category          domain.Category
	OwnerID           string
	RequestingService string
}

// ResolveCredentialByOwner is ResolveCredential's sibling — same
// fail-closed/audited resolve, keyed by owner instead of by id. Shares
// resolveMetadata with ResolveCredential.Execute (resolve_credential.go) so
// the two never drift on the audit-ordering guarantee.
type ResolveCredentialByOwner struct {
	metadataRepo CredentialMetadataRepository
	auditRepo    AuditRepository
	store        SecretStore
	now          func() time.Time
}

func NewResolveCredentialByOwner(metadataRepo CredentialMetadataRepository, auditRepo AuditRepository, store SecretStore) *ResolveCredentialByOwner {
	return &ResolveCredentialByOwner{metadataRepo: metadataRepo, auditRepo: auditRepo, store: store, now: time.Now}
}

func (uc *ResolveCredentialByOwner) Execute(ctx context.Context, in ResolveCredentialByOwnerInput) ([]byte, error) {
	if in.TenantID == "" || in.OwnerID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_SCOPE", "tenant_id and owner_id are required", nil)
	}
	if !in.Category.Valid() {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_INVALID_CATEGORY", "unknown credential category", nil)
	}

	metadata, err := uc.metadataRepo.GetByOwner(ctx, in.TenantID, in.Category, in.OwnerID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			// No audit row: same schema-driven limitation
			// ResolveCredential.Execute's doc comment documents for the
			// by-id not-found case — access_audit_log.credential_id has a
			// NOT NULL FK to credential_metadata(id), and there is no row
			// to reference here either.
			return nil, apperrors.New(apperrors.KindNotFound, "CREDENTIAL_NOT_FOUND", "no credential found for this tenant/category/owner", err)
		}
		return nil, apperrors.New(apperrors.KindInternal, "CREDENTIAL_FETCH_FAILED", "failed to fetch credential metadata", err)
	}

	return resolveMetadata(ctx, resolveDeps{auditRepo: uc.auditRepo, store: uc.store, now: uc.now}, metadata, in.RequestingService)
}
