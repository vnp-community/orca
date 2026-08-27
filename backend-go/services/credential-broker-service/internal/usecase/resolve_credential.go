package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// ResolveCredentialInput mirrors ResolveCredentialRequest 1:1.
// RequestingService is resolved from mTLS/JWT identity by the gRPC layer
// (internal/adapter/grpc), never trusted from the request body — see
// credential-broker-service.md §4's AccessAuditEntry note.
type ResolveCredentialInput struct {
	CredentialID      string
	RequestingService string
}

// ResolveCredential fetches a credential's decrypted value from Vault. See
// Execute's doc comment for the audit-ordering guarantee this usecase must
// preserve.
type ResolveCredential struct {
	metadataRepo CredentialMetadataRepository
	auditRepo    AuditRepository
	store        SecretStore
	now          func() time.Time
}

func NewResolveCredential(metadataRepo CredentialMetadataRepository, auditRepo AuditRepository, store SecretStore) *ResolveCredential {
	return &ResolveCredential{metadataRepo: metadataRepo, auditRepo: auditRepo, store: store, now: time.Now}
}

// Execute resolves a credential's plaintext value. The access-audit row is
// written BEFORE the value is returned to the caller on every path that
// reaches Vault — this ordering is the load-bearing guarantee from
// credential-broker-service.md §8 ("audit log write must never be
// best-effort... every resolve usecase writes its audit row... synchronously
// before returning success") and §9 (fail-closed on revoked credentials or
// an unreachable Vault, with the attempt still audited). See this file's
// tests for the ordering assertion.
//
// One documented exception: a resolve for a credential_id that doesn't
// exist at all writes NO audit row, because access_audit_log.credential_id
// has a NOT NULL foreign key to credential_metadata(id) (see
// migrations/0001_init.up.sql) — there is no valid row to reference. This
// is a schema-driven limitation, not an oversight; see this service's
// README "Known gaps".
func (uc *ResolveCredential) Execute(ctx context.Context, in ResolveCredentialInput) ([]byte, error) {
	if in.CredentialID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CREDENTIAL_MISSING_ID", "credential_id is required", nil)
	}

	metadata, err := uc.metadataRepo.Get(ctx, in.CredentialID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) {
			return nil, apperrors.New(apperrors.KindNotFound, "CREDENTIAL_NOT_FOUND", "credential not found", err)
		}
		return nil, apperrors.New(apperrors.KindInternal, "CREDENTIAL_FETCH_FAILED", "failed to fetch credential metadata", err)
	}

	return resolveMetadata(ctx, resolveDeps{auditRepo: uc.auditRepo, store: uc.store, now: uc.now}, metadata, in.RequestingService)
}

// resolveDeps bundles what resolveMetadata needs from either
// ResolveCredential or ResolveCredentialByOwner — a plain struct rather
// than an interface since both callers already hold the same
// AuditRepository/SecretStore/now types.
type resolveDeps struct {
	auditRepo AuditRepository
	store     SecretStore
	now       func() time.Time
}

// resolveMetadata is ResolveCredential.Execute's fail-closed/audit-then-
// return logic, factored out so ResolveCredentialByOwner.Execute (which
// looks the row up by (tenant_id, category, owner_id) instead of by id) can
// share it exactly rather than re-implement the ordering guarantee this
// file's doc comment describes. See that doc comment for the ordering
// requirement this function preserves.
func resolveMetadata(ctx context.Context, deps resolveDeps, metadata domain.CredentialMetadata, requestingService string) ([]byte, error) {
	now := deps.now()

	if metadata.IsRevoked() {
		if auditErr := appendAudit(ctx, deps.auditRepo, metadata.ID, requestingService, domain.ActionResolve, now); auditErr != nil {
			return nil, auditErr
		}
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "CREDENTIAL_REVOKED", "credential has been revoked", domain.ErrCredentialRevoked)
	}

	value, resolveErr := resolveFromVault(ctx, deps.store, metadata)
	if resolveErr != nil {
		if auditErr := appendAudit(ctx, deps.auditRepo, metadata.ID, requestingService, domain.ActionResolve, now); auditErr != nil {
			return nil, auditErr
		}
		return nil, apperrors.New(apperrors.KindInternal, "CREDENTIAL_VAULT_RESOLVE_FAILED", "failed to resolve credential from vault", resolveErr)
	}

	// Audit BEFORE returning the value — see this method's doc comment.
	if err := appendAudit(ctx, deps.auditRepo, metadata.ID, requestingService, domain.ActionResolve, now); err != nil {
		return nil, err
	}

	return value, nil
}

func resolveFromVault(ctx context.Context, store SecretStore, metadata domain.CredentialMetadata) ([]byte, error) {
	data, err := store.KVRead(ctx, kvMount, metadata.VaultPath)
	if err != nil {
		return nil, err
	}
	ciphertext, _ := data["ciphertext"].(string)
	if ciphertext == "" {
		return nil, errors.New("usecase: no ciphertext stored at vault_path")
	}
	return store.TransitDecrypt(ctx, transitKeyName(string(metadata.Category)), ciphertext)
}
