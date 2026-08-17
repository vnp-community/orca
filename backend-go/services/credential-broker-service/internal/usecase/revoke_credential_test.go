package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestRevokeCredential_CallsVaultRevokeAndMarksRevoked(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "token")

	uc := NewRevokeCredential(metadataRepo, auditRepo, store)
	got, err := uc.Execute(context.Background(), RevokeCredentialInput{CredentialID: m.ID, RequestingService: "scm-integration-service"})
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

	// A subsequent resolve must now fail closed.
	resolveUC := NewResolveCredential(metadataRepo, newFakeAuditRepo(rec), store)
	if _, err := resolveUC.Execute(context.Background(), ResolveCredentialInput{CredentialID: m.ID, RequestingService: "scm-integration-service"}); err == nil {
		t.Error("expected resolving a revoked credential to fail")
	}
}

func TestRevokeCredential_IsIdempotent(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusRevoked, "token")

	uc := NewRevokeCredential(metadataRepo, auditRepo, store)
	got, err := uc.Execute(context.Background(), RevokeCredentialInput{CredentialID: m.ID})
	if err != nil {
		t.Fatalf("unexpected error revoking an already-revoked credential: %v", err)
	}
	if !got.IsRevoked() {
		t.Error("expected already-revoked credential to remain revoked")
	}
	if len(store.revokeCalls) != 0 {
		t.Errorf("expected no additional vault call for an already-revoked credential, got %d", len(store.revokeCalls))
	}
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no additional audit entry for a no-op revoke, got %d", len(auditRepo.entries))
	}
}

func TestRevokeCredential_NotFound(t *testing.T) {
	rec := &callRecorder{}
	uc := NewRevokeCredential(newFakeMetadataRepo(rec), newFakeAuditRepo(rec), newFakeSecretStore(rec))
	_, err := uc.Execute(context.Background(), RevokeCredentialInput{CredentialID: "missing"})
	if err == nil {
		t.Fatal("expected an error for a nonexistent credential")
	}
}
