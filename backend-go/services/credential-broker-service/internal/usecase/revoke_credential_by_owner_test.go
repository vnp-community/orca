package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestRevokeCredentialByOwner_CallsVaultRevokeAndMarksRevoked(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "github-token-xyz")

	uc := NewRevokeCredentialByOwner(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	got, err := uc.Execute(context.Background(), RevokeCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "user-1",
		RequestingService: "scm-integration-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsRevoked() {
		t.Fatalf("expected returned metadata to be revoked, got status %s", got.Status)
	}

	// The core assertion: revocation actually calls the store's
	// revoke/delete path, not just the Postgres status flip.
	if len(store.revokeCalls) != 1 {
		t.Fatalf("expected RevokeSecret to be called exactly once, got %d calls: %v", len(store.revokeCalls), store.revokeCalls)
	}
	if store.revokeCalls[0] != kvKey(kvMount, m.VaultPath) {
		t.Errorf("expected RevokeSecret to target %s, got %s", kvKey(kvMount, m.VaultPath), store.revokeCalls[0])
	}

	persisted, err := metadataRepo.Get(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("fetching persisted metadata: %v", err)
	}
	if persisted.Status != domain.StatusRevoked {
		t.Errorf("expected persisted status revoked, got %s", persisted.Status)
	}

	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Action != domain.ActionRevoke {
		t.Fatalf("expected exactly one revoke audit entry, got %v", auditRepo.entries)
	}

	// A subsequent owner-keyed resolve must now fail closed.
	resolveUC := NewResolveCredentialByOwner(metadataRepo, newFakeAuditRepo(rec), store)
	if _, err := resolveUC.Execute(context.Background(), ResolveCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "user-1", RequestingService: "scm-integration-service",
	}); err == nil {
		t.Error("expected resolving a revoked credential by owner to fail")
	}
}

// TestRevokeCredentialByOwner_AlreadyRevokedIsNotFound documents the
// idempotency decision recorded on RevokeCredentialByOwner's doc comment:
// unlike RevokeCredential (by id), which special-cases an already-revoked
// row as an idempotent success, GetByOwner filters revoked rows out at the
// SQL level, so revoking an already-revoked owner-keyed credential surfaces
// as plain CREDENTIAL_NOT_FOUND — the second call in a retry sequence is an
// error, not a silent no-op.
func TestRevokeCredentialByOwner_AlreadyRevokedIsNotFound(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusRevoked, "github-token-xyz")

	uc := NewRevokeCredentialByOwner(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	_, err := uc.Execute(context.Background(), RevokeCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "user-1",
		RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected an error revoking an already-revoked credential by owner")
	}
	if len(store.revokeCalls) != 0 {
		t.Errorf("expected no vault call for an already-revoked owner-keyed credential, got %d", len(store.revokeCalls))
	}
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit entries, got %v", auditRepo.entries)
	}
}

func TestRevokeCredentialByOwner_NotFound(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	uc := NewRevokeCredentialByOwner(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	_, err := uc.Execute(context.Background(), RevokeCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "github",
		RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected an error when no credential matches (tenant, category, owner)")
	}
}

func TestRevokeCredentialByOwner_RequiresScope(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	uc := NewRevokeCredentialByOwner(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	_, err := uc.Execute(context.Background(), RevokeCredentialByOwnerInput{Category: domain.CategoryScmOAuth})
	if err == nil {
		t.Fatal("expected an error for missing tenant_id/owner_id")
	}
}

func TestRevokeCredentialByOwner_VaultFailureLeavesMetadataUnrevoked(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)
	store.revokeErr = errors.New("vault unreachable")

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "github-token-xyz")

	uc := NewRevokeCredentialByOwner(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	_, err := uc.Execute(context.Background(), RevokeCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "user-1",
		RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected an error when the vault revoke fails")
	}

	persisted, getErr := metadataRepo.Get(context.Background(), m.ID)
	if getErr != nil {
		t.Fatalf("fetching persisted metadata: %v", getErr)
	}
	if persisted.Status == domain.StatusRevoked {
		t.Error("expected metadata to stay non-revoked when the vault revoke fails, so a retry is meaningful")
	}
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit entry when the vault revoke fails before the tx even starts, got %v", auditRepo.entries)
	}
}

// TestRevokeCredentialByOwner_AuditFailureRollsBackStatus proves the
// metadata status transition and the audit append commit atomically inside
// one TxRunner.RunInTx call — the same proof pattern established for
// RevokeCredential/RotateCredential/WriteCredential's TxRunner tests.
func TestRevokeCredentialByOwner_AuditFailureRollsBackStatus(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	auditRepo.appendErr = errors.New("audit db down")
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "github-token-xyz")

	uc := NewRevokeCredentialByOwner(metadataRepo, store, newFakeTxRunner(rec, metadataRepo, auditRepo))
	_, err := uc.Execute(context.Background(), RevokeCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "user-1",
		RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected an error when the audit append fails")
	}

	persisted, getErr := metadataRepo.Get(context.Background(), m.ID)
	if getErr != nil {
		t.Fatalf("fetching persisted metadata: %v", getErr)
	}
	if persisted.Status == domain.StatusRevoked {
		t.Error("expected the status update to roll back when the audit append fails in the same tx")
	}
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit entries to persist after rollback, got %v", auditRepo.entries)
	}

	// Note: the Vault-side secret was already destroyed before the tx ran
	// (same ordering as RevokeCredential) — this test only proves the
	// metadata+audit pair's atomicity, not a Vault rollback, matching
	// RevokeCredential's documented ordering tradeoff.
}
