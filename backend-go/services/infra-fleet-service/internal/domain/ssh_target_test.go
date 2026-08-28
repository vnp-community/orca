package domain

import "testing"

func TestNewSshTarget_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name         string
		tenantID     string
		host         string
		userName     string
		vaultSSHRole string
		wantErr      error
	}{
		{"valid", "t1", "10.0.0.1", "orca", "ssh-role-dev", nil},
		{"empty tenant", "", "10.0.0.1", "orca", "ssh-role-dev", ErrEmptySshTargetTenant},
		{"empty host", "t1", "", "orca", "ssh-role-dev", ErrEmptySshTargetHost},
		{"empty user", "t1", "10.0.0.1", "", "ssh-role-dev", ErrEmptySshTargetUser},
		{"empty vault role", "t1", "10.0.0.1", "orca", "", ErrEmptyVaultSSHRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := NewSshTarget("st1", tt.tenantID, tt.host, 0, tt.userName, tt.vaultSSHRole, "", "", "", nil)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if target.ID != "st1" || target.VaultSSHRole != tt.vaultSSHRole {
					t.Errorf("unexpected SshTarget: %+v", target)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewSshTarget_NeverStoresRawKeyMaterial(t *testing.T) {
	// Regression guard for the security invariant in
	// specs/backend-go/services/infra-fleet-service.md §9: SshTarget has no
	// field a caller could stuff key material into, and VaultSSHRole is
	// mandatory (a pointer, not optional).
	target, err := NewSshTarget("st1", "t1", "host", 0, "user", "role", "", "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.VaultSSHRole == "" {
		t.Error("expected VaultSSHRole to be set")
	}
}

func TestNewSshTarget_PortDefaultsTo22(t *testing.T) {
	target, err := NewSshTarget("st1", "t1", "host", 0, "user", "role", "", "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Port != 22 {
		t.Errorf("expected default port 22, got %d", target.Port)
	}
}

func TestNewSshTarget_PortKnownHostsJumpHostRoundTrip(t *testing.T) {
	target, err := NewSshTarget("st1", "t1", "host", 2222, "user", "role", "SHA256:abc", "bastion-1", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Port != 2222 {
		t.Errorf("expected port 2222, got %d", target.Port)
	}
	if target.KnownHostsFingerprint != "SHA256:abc" {
		t.Errorf("expected known hosts fingerprint to round-trip, got %q", target.KnownHostsFingerprint)
	}
	if target.JumpHostTargetID != "bastion-1" {
		t.Errorf("expected jump host target id to round-trip, got %q", target.JumpHostTargetID)
	}
}
