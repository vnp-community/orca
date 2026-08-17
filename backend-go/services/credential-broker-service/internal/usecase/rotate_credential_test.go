package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestRotateCredential_ReEncryptsAndAudits(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryAiProviderKey, domain.StatusActive, "old-key")

	uc := NewRotateCredential(metadataRepo, auditRepo, store)
	got, err := uc.Execute(context.Background(), RotateCredentialInput{
		CredentialID: m.ID, NewEncryptedEnvelope: []byte("new-key"), RequestingService: "ai-provider-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != domain.StatusActive {
		t.Errorf("expected status active after rotation, got %s", got.Status)
	}
	if got.VaultPath != m.VaultPath {
		t.Errorf("expected rotation to reuse the same vault_path, got %s vs %s", got.VaultPath, m.VaultPath)
	}

	// The resolved value after rotation must be the NEW material.
	resolveUC := NewResolveCredential(metadataRepo, newFakeAuditRepo(rec), store)
	value, err := resolveUC.Execute(context.Background(), ResolveCredentialInput{CredentialID: m.ID, RequestingService: "ai-provider-service"})
	if err != nil {
		t.Fatalf("resolving rotated credential: %v", err)
	}
	if string(value) != "new-key" {
		t.Errorf("expected resolved value %q after rotation, got %q", "new-key", value)
	}

	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Action != domain.ActionRotate {
		t.Fatalf("expected exactly one rotate audit entry, got %v", auditRepo.entries)
	}
}

func TestRotateCredential_RejectsRevoked(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusRevoked, "token")

	uc := NewRotateCredential(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), RotateCredentialInput{CredentialID: m.ID, NewEncryptedEnvelope: []byte("x")})
	if err == nil {
		t.Fatal("expected rotating a revoked credential to fail")
	}
}

func TestRotateCredential_NotFound(t *testing.T) {
	rec := &callRecorder{}
	uc := NewRotateCredential(newFakeMetadataRepo(rec), newFakeAuditRepo(rec), newFakeSecretStore(rec))
	_, err := uc.Execute(context.Background(), RotateCredentialInput{CredentialID: "missing", NewEncryptedEnvelope: []byte("x")})
	if err == nil {
		t.Fatal("expected an error for a nonexistent credential")
	}
}
