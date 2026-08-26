package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestListCredentialsByCategory_FiltersByRequestingTenant(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	store := newFakeSecretStore(rec)
	writeUC := NewWriteCredential(store, newFakeTxRunner(rec, metadataRepo, newFakeAuditRepo(rec)))

	// 3 credentials across 2 tenants, same category.
	seed := []struct {
		tenantID, ownerID string
	}{
		{"tenant-1", "bitbucket"},
		{"tenant-1", "gitea"},
		{"tenant-2", "bitbucket"},
	}
	for _, s := range seed {
		if _, err := writeUC.Execute(context.Background(), WriteCredentialInput{
			TenantID: s.tenantID, OwnerID: s.ownerID, Category: domain.CategoryScmOAuth,
			EncryptedEnvelope: []byte("tok"), RequestingService: "seed",
		}); err != nil {
			t.Fatalf("seeding credential for %+v: %v", s, err)
		}
	}

	uc := NewListCredentialsByCategory(metadataRepo)
	got, err := uc.Execute(context.Background(), ListCredentialsByCategoryInput{TenantID: "tenant-1", Category: domain.CategoryScmOAuth})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 credentials for tenant-1, got %d: %+v", len(got), got)
	}
	owners := map[string]bool{}
	for _, m := range got {
		if m.TenantID != "tenant-1" {
			t.Errorf("expected only tenant-1 rows, got tenant %q", m.TenantID)
		}
		owners[m.OwnerID] = true
	}
	if !owners["bitbucket"] || !owners["gitea"] {
		t.Errorf("expected both bitbucket and gitea owners, got %+v", owners)
	}
}

func TestListCredentialsByCategory_MissingTenant_ErrorsWithoutCallingRepo(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)

	uc := NewListCredentialsByCategory(metadataRepo)
	if _, err := uc.Execute(context.Background(), ListCredentialsByCategoryInput{}); err == nil {
		t.Fatal("expected CREDENTIAL_MISSING_SCOPE error")
	}
	for _, call := range rec.snapshot() {
		if call == "metadata.ListByCategory" {
			t.Error("expected ListByCategory to never be called when tenant_id is missing")
		}
	}
}
