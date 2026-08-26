package domain

import "testing"

func TestHostSetupStatus_Valid(t *testing.T) {
	tests := []struct {
		status HostSetupStatus
		want   bool
	}{
		{HostSetupPending, true},
		{HostSetupValidated, true},
		{HostSetupCompleted, true},
		{HostSetupFailed, true},
		{HostSetupStatus("bogus"), false},
		{HostSetupStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.want {
				t.Errorf("Valid() for %q = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestNewHostSetup_StartsAtPending(t *testing.T) {
	setup, err := NewHostSetup("hs1", "t1", "dev1", "/home/dev/repo", "My Setup", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if setup.Status != HostSetupPending {
		t.Errorf("expected Status=%q, got %q", HostSetupPending, setup.Status)
	}
}

func TestNewHostSetup_RejectsEmptyRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		tenantID    string
		devServerID string
		folderPath  string
		createdBy   string
		wantErr     bool
	}{
		{"valid", "t1", "dev1", "/home/dev/repo", "u1", false},
		{"empty tenant", "", "dev1", "/home/dev/repo", "u1", true},
		{"empty dev server id", "t1", "", "/home/dev/repo", "u1", true},
		{"empty folder path", "t1", "dev1", "", "u1", true},
		{"empty created by", "t1", "dev1", "/home/dev/repo", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHostSetup("hs1", tt.tenantID, tt.devServerID, tt.folderPath, "display", tt.createdBy)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
