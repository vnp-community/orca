package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

func TestResolveCredentialByOwner_ResolvesAndAudits(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	// seedCredential always uses owner_id "user-1" — mirrors what
	// scm-integration-service/issue-tracking-service would resolve against
	// (owner_id = provider name in their real usage, but the lookup
	// mechanics are identical).
	m := seedCredential(t, rec, metadataRepo, store, domain.CategoryScmOAuth, domain.StatusActive, "github-token-xyz")

	uc := NewResolveCredentialByOwner(metadataRepo, auditRepo, store)
	value, err := uc.Execute(context.Background(), ResolveCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "user-1",
		RequestingService: "scm-integration-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(value) != "github-token-xyz" {
		t.Errorf("got %q, want %q", value, "github-token-xyz")
	}
	if len(auditRepo.entries) != 1 || auditRepo.entries[0].CredentialID != m.ID {
		t.Errorf("expected one audit entry for %s, got %v", m.ID, auditRepo.entries)
	}
}

func TestResolveCredentialByOwner_NotFound_NoAuditRow(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	uc := NewResolveCredentialByOwner(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), ResolveCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryScmOAuth, OwnerID: "github",
		RequestingService: "scm-integration-service",
	})
	if err == nil {
		t.Fatal("expected an error when no credential matches (tenant, category, owner)")
	}
	// Same schema-driven limitation as ResolveCredential's not-found path
	// (access_audit_log.credential_id has a NOT NULL FK) — no row to
	// reference, so no audit entry.
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit entries on not-found, got %v", auditRepo.entries)
	}
}

func TestResolveCredentialByOwner_RequiresScope(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	uc := NewResolveCredentialByOwner(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), ResolveCredentialByOwnerInput{Category: domain.CategoryScmOAuth})
	if err == nil {
		t.Fatal("expected an error for missing tenant_id/owner_id")
	}
}

// TestResolveCredentialByOwner_RevokedIsNotFound documents a real, intended
// behavior difference from ResolveCredential (by-id): GetByOwner's query
// filters out revoked rows at the SQL level ("most recent NON-REVOKED
// credential for this owner"), so a revoked credential is indistinguishable
// from "no credential ever existed" for an owner-based lookup — unlike
// ResolveCredential, which is handed a specific id and CAN find a revoked
// row to report as "found but revoked" (see
// TestResolveCredential_RevokedCredential_StillAuditsThenFailsClosed).
// resolveMetadata's IsRevoked branch is therefore unreachable via this
// caller — not dead code so much as a guarantee that only ResolveCredential
// exercises it, by construction of GetByOwner's query.
func TestResolveCredentialByOwner_RevokedIsNotFound(t *testing.T) {
	rec := &callRecorder{}
	metadataRepo := newFakeMetadataRepo(rec)
	auditRepo := newFakeAuditRepo(rec)
	store := newFakeSecretStore(rec)

	seedCredential(t, rec, metadataRepo, store, domain.CategoryIssueTrackerOAuth, domain.StatusRevoked, "jira-token")

	uc := NewResolveCredentialByOwner(metadataRepo, auditRepo, store)
	_, err := uc.Execute(context.Background(), ResolveCredentialByOwnerInput{
		TenantID: "tenant-1", Category: domain.CategoryIssueTrackerOAuth, OwnerID: "user-1",
		RequestingService: "issue-tracking-service",
	})
	if err == nil {
		t.Fatal("expected an error resolving a revoked credential by owner")
	}
	if len(auditRepo.entries) != 0 {
		t.Errorf("expected no audit entries (GetByOwner never surfaces the revoked row), got %v", auditRepo.entries)
	}
}
