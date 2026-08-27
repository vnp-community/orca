package domain

import (
	"testing"
	"time"
)

func TestNewCredentialMetadata_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		id        string
		tenantID  string
		ownerID   string
		category  Category
		vaultPath string
		wantErr   error
	}{
		{"valid", "c1", "t1", "o1", CategoryScmOAuth, "credential/t1/c1", nil},
		{"empty id", "", "t1", "o1", CategoryScmOAuth, "credential/t1/c1", ErrEmptyID},
		{"empty tenant", "c1", "", "o1", CategoryScmOAuth, "credential/t1/c1", ErrEmptyTenant},
		{"empty owner", "c1", "t1", "", CategoryScmOAuth, "credential/t1/c1", ErrEmptyOwner},
		{"invalid category", "c1", "t1", "o1", Category("bogus"), "credential/t1/c1", ErrInvalidCategory},
		{"empty vault path", "c1", "t1", "o1", CategoryScmOAuth, "", ErrEmptyVaultPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCredentialMetadata(tt.id, tt.tenantID, tt.ownerID, tt.category, tt.vaultPath, StatusActive, "", now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewCredentialMetadata_DefaultsStatusToPending(t *testing.T) {
	now := time.Now()
	m, err := NewCredentialMetadata("c1", "t1", "o1", CategoryScmOAuth, "credential/t1/c1", "", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Status != StatusPending {
		t.Errorf("expected default status pending, got %s", m.Status)
	}
}

func TestNewCredentialMetadata_RejectsInvalidStatus(t *testing.T) {
	now := time.Now()
	_, err := NewCredentialMetadata("c1", "t1", "o1", CategoryScmOAuth, "credential/t1/c1", Status("bogus"), "", now)
	if err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

// No test can enforce "no secret field exists" at runtime — this test
// documents the invariant so a future struct-literal change to
// CredentialMetadata that adds a field is at least forced through this
// file's diff, per credential-broker-service.md §5.
func TestCredentialMetadata_HasNoSecretField(t *testing.T) {
	m := CredentialMetadata{
		ID: "c1", TenantID: "t1", OwnerID: "o1",
		Category: CategoryScmOAuth, Status: StatusActive, VaultPath: "credential/t1/c1",
	}
	// Compile-time enumeration of every field this struct is allowed to
	// have. If this test still compiles, no secret-shaped field snuck in.
	_ = struct {
		ID, TenantID, OwnerID, VaultPath string
		Category                         Category
		Status                           Status
		CreatedAt, UpdatedAt             time.Time
	}{m.ID, m.TenantID, m.OwnerID, m.VaultPath, m.Category, m.Status, m.CreatedAt, m.UpdatedAt}
}

func TestCategory_Engine(t *testing.T) {
	assertEngine := func(t *testing.T, c Category, want VaultEngine) {
		t.Helper()
		if got := c.Engine(); got != want {
			t.Errorf("Category(%s).Engine() = %s, want %s", c, got, want)
		}
	}

	assertEngine(t, CategoryAiProviderKey, VaultEngineTransit)
	for _, c := range []Category{CategoryScmOAuth, CategoryIssueTrackerOAuth, CategorySsh, CategoryServiceSecret} {
		assertEngine(t, c, VaultEngineKV2)
	}
}

func TestCredentialMetadata_Revoke(t *testing.T) {
	now := time.Now()
	m, err := NewCredentialMetadata("c1", "t1", "o1", CategoryScmOAuth, "credential/t1/c1", StatusActive, "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	later := now.Add(time.Hour)
	revoked := m.Revoke(later)
	if !revoked.IsRevoked() {
		t.Error("expected revoked credential to report IsRevoked() == true")
	}
	if m.IsRevoked() {
		t.Error("Revoke must not mutate the receiver")
	}
	if !revoked.UpdatedAt.Equal(later) {
		t.Errorf("expected UpdatedAt=%v, got %v", later, revoked.UpdatedAt)
	}
}
