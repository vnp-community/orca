package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestGetCredentialMetadata_ReturnsMetadataNeverTouchesVault(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	store := newFakeSecretStore(rec)

	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryAiProviderKey, domain.StatusActive, "sk-fake-key")
	rec.calls = nil // reset so this test only inspects GetCredentialMetadata's own call log

	uc := NewGetCredentialMetadata(metadataRepo)
	got, err := uc.Execute(context.Background(), GetCredentialMetadataInput{CredentialID: m.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != m.ID || got.Status != domain.StatusActive {
		t.Errorf("got %+v, want id=%s status=active", got, m.ID)
	}

	// The whole point of this RPC: it must never call into Vault (no
	// TransitDecrypt/KVRead) and never write an audit row — a metadata
	// read exposes no secret material, so there is nothing to audit.
	for _, call := range rec.snapshot() {
		if call == "store.TransitDecrypt" || call == "store.KVRead" || call == "audit.Append" {
			t.Errorf("GetCredentialMetadata must never call %q, but it did (calls=%v)", call, rec.snapshot())
		}
	}
}

func TestGetCredentialMetadata_NotFound(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)

	uc := NewGetCredentialMetadata(metadataRepo)
	_, err := uc.Execute(context.Background(), GetCredentialMetadataInput{CredentialID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for a missing credential")
	}
}

func TestGetCredentialMetadata_RequiresID(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)

	uc := NewGetCredentialMetadata(metadataRepo)
	_, err := uc.Execute(context.Background(), GetCredentialMetadataInput{})
	if err == nil {
		t.Fatal("expected an error for an empty credential_id")
	}
}
