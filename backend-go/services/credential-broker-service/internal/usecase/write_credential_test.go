package usecase

import (
	"errors"
	"testing"

	"context"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestWriteCredential_EncryptsPersistsAndAudits(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	uc := NewWriteCredential(metadataRepo, auditRepo, store)
	got, err := uc.Execute(context.Background(), WriteCredentialInput{
		TenantID:          "tenant-1",
		OwnerID:           "user-1",
		Category:          domain.CategoryScmOAuth,
		EncryptedEnvelope: []byte("opaque-envelope-bytes"),
		RequestingService: "scm-integration-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusActive {
		t.Errorf("expected status active, got %s", got.Status)
	}
	if got.VaultPath == "" {
		t.Error("expected a non-empty vault_path")
	}
	if len(metadataRepo.rows) != 1 {
		t.Fatalf("expected 1 metadata row, got %d", len(metadataRepo.rows))
	}
	if len(auditRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditRepo.entries))
	}
	if auditRepo.entries[0].Action != domain.ActionWrite {
		t.Errorf("expected write action, got %s", auditRepo.entries[0].Action)
	}
	if auditRepo.entries[0].CredentialID != got.ID {
		t.Errorf("expected audit entry to reference the created credential id")
	}

	// The stored ciphertext must never equal the raw input bytes — a
	// regression here would mean this usecase is persisting bytes without
	// ever calling Vault Transit, defeating the entire point of this
	// service.
	stored, err := store.KVRead(context.Background(), kvMount, got.VaultPath)
	if err != nil {
		t.Fatalf("expected ciphertext to be readable back from the fake store: %v", err)
	}
	if stored["ciphertext"] == "opaque-envelope-bytes" {
		t.Error("expected the persisted value to be Transit ciphertext, not the raw envelope bytes")
	}
}

func TestWriteCredential_RejectsMissingScope(t *testing.T) {
	rec := &callRecorder{}
	uc := NewWriteCredential(newFakeMetadataRepo(rec), newFakeAuditRepo(rec), newFakeSecretStore(rec))

	_, err := uc.Execute(context.Background(), WriteCredentialInput{
		Category: domain.CategoryScmOAuth,
	})
	if err == nil {
		t.Fatal("expected an error when tenant_id/owner_id are missing")
	}
}

func TestWriteCredential_VaultEncryptFailure_CreatesNoRowsAndNoAudit(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)
	store.encryptErr = errors.New("vault sealed")

	uc := NewWriteCredential(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), WriteCredentialInput{
		TenantID: "tenant-1", OwnerID: "user-1", Category: domain.CategoryScmOAuth,
	})
	if err == nil {
		t.Fatal("expected an error when vault encrypt fails")
	}
	if len(metadataRepo.rows) != 0 {
		t.Errorf("expected no metadata row when the vault write never succeeded, got %d", len(metadataRepo.rows))
	}
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit row for a credential that was never created, got %d", len(auditRepo.entries))
	}
}

func TestWriteCredential_AuditWriteFailure_FailsTheWholeOperation(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	auditRepo.appendErr = errors.New("db unavailable")
	store := newFakeSecretStore(rec)

	uc := NewWriteCredential(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), WriteCredentialInput{
		TenantID: "tenant-1", OwnerID: "user-1", Category: domain.CategoryScmOAuth,
	})
	// Per credential-broker-service.md §8: "if the audit write fails, the
	// operation fails" — even though the metadata row above WAS created.
	if err == nil {
		t.Fatal("expected WriteCredential to fail when the audit write fails")
	}
}
