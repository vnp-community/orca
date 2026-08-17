package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func seedCredential(t *testing.T, rec *callRecorder, metadataRepo *fakeMetadataRepo, store *fakeSecretStore, category domain.Category, status domain.Status, rawValue string) domain.CredentialMetadata {
	t.Helper()
	writeUC := NewWriteCredential(store, newFakeTxRunner(rec, metadataRepo, newFakeAuditRepo(rec)))
	m, err := writeUC.Execute(context.Background(), WriteCredentialInput{
		TenantID: "tenant-1", OwnerID: "user-1", Category: category,
		EncryptedEnvelope: []byte(rawValue), RequestingService: "seed",
	})
	if err != nil {
		t.Fatalf("seeding credential: %v", err)
	}
	if status != domain.StatusActive {
		if err := metadataRepo.UpdateStatus(context.Background(), m.ID, status, m.UpdatedAt); err != nil {
			t.Fatalf("seeding status: %v", err)
		}
		m.Status = status
	}
	return m
}

func TestResolveCredential_AuditWrittenBeforeReturn_OnSuccess(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "github-token-abc")
	rec.calls = nil // reset so this test only inspects ResolveCredential's own call order

	uc := NewResolveCredential(metadataRepo, auditRepo, store)
	value, err := uc.Execute(context.Background(), ResolveCredentialInput{
		CredentialID: m.ID, RequestingService: "scm-integration-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(value) != "github-token-abc" {
		t.Errorf("expected resolved value %q, got %q", "github-token-abc", value)
	}

	// The ordering assertion: TransitDecrypt (the last Vault call) must
	// appear in the call log strictly before audit.Append — proving the
	// audit row is written before the resolved value is handed back, per
	// credential-broker-service.md §8's ordering requirement.
	calls := rec.snapshot()
	decryptIdx, auditIdx := indexOf(calls, "store.TransitDecrypt"), indexOf(calls, "audit.Append")
	if decryptIdx == -1 || auditIdx == -1 {
		t.Fatalf("expected both a decrypt and an audit call, got %v", calls)
	}
	if auditIdx < decryptIdx {
		t.Fatalf("expected audit.Append AFTER store.TransitDecrypt, got order %v", calls)
	}
	if len(auditRepo.entries) != 1 || auditRepo.entries[0].Action != domain.ActionResolve {
		t.Fatalf("expected exactly one resolve audit entry, got %v", auditRepo.entries)
	}
}

func TestResolveCredential_RevokedCredential_StillAuditsThenFailsClosed(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategorySsh, domain.StatusRevoked, "ssh-key-material")

	uc := NewResolveCredential(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), ResolveCredentialInput{
		CredentialID: m.ID, RequestingService: "infra-fleet-service",
	})
	if err == nil {
		t.Fatal("expected resolving a revoked credential to fail")
	}
	if len(auditRepo.entries) != 1 {
		t.Fatalf("expected the denied resolve attempt to still be audited, got %d entries", len(auditRepo.entries))
	}
}

func TestResolveCredential_NotFound_WritesNoAudit(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	uc := NewResolveCredential(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), ResolveCredentialInput{
		CredentialID: "does-not-exist", RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected an error for a nonexistent credential")
	}
	// access_audit_log.credential_id has a NOT NULL FK to
	// credential_metadata(id) — there is no row to reference, so no audit
	// entry can legally exist for this attempt. See resolve_credential.go's
	// doc comment.
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit entries for a not-found credential (FK constraint), got %d", len(auditRepo.entries))
	}
}

func TestResolveCredential_VaultFailure_StillAudits(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryAiProviderKey, domain.StatusActive, "anthropic-key")
	store.decryptErr = errors.New("vault unreachable")

	uc := NewResolveCredential(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), ResolveCredentialInput{
		CredentialID: m.ID, RequestingService: "ai-provider-service",
	})
	if err == nil {
		t.Fatal("expected an error when vault decrypt fails")
	}
	if len(auditRepo.entries) != 1 {
		t.Fatalf("expected the failed vault attempt to still be audited, got %d entries", len(auditRepo.entries))
	}
}

func TestResolveCredential_AuditWriteFailure_PropagatesAsError(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "token")
	auditRepo.appendErr = errors.New("db unavailable")

	uc := NewResolveCredential(metadataRepo, auditRepo, store)
	value, err := uc.Execute(context.Background(), ResolveCredentialInput{
		CredentialID: m.ID, RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected ResolveCredential to fail when the audit write fails, per §8's never-best-effort rule")
	}
	if value != nil {
		t.Error("expected no value to be returned when the audit write failed")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T", err)
	}
}

func indexOf(haystack []string, needle string) int {
	for i, v := range haystack {
		if v == needle {
			return i
		}
	}
	return -1
}
