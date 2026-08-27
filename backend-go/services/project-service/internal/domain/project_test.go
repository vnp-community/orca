package domain

import "testing"

func TestNewProject_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		pname    string
		wantErr  error
	}{
		{"valid", "t1", "my-project", nil},
		{"empty tenant", "", "my-project", ErrEmptyTenantID},
		{"empty name", "t1", "", ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProject("p1", tt.tenantID, tt.pname, "")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewProject_StartsUnbound(t *testing.T) {
	p, err := NewProject("p1", "t1", "my-project", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.DevServerID != "" {
		t.Errorf("expected a freshly created project to start unbound, got dev_server_id=%q", p.DevServerID)
	}
}

func TestProject_Rebind(t *testing.T) {
	p, err := NewProject("p1", "t1", "my-project", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rebound, err := p.Rebind("dev-server-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rebound.DevServerID != "dev-server-2" {
		t.Errorf("expected DevServerID=dev-server-2, got %q", rebound.DevServerID)
	}
	if p.DevServerID != "" {
		t.Errorf("Rebind must not mutate the receiver, got %q", p.DevServerID)
	}
}

func TestProject_Rebind_RejectsEmptyDevServerID(t *testing.T) {
	p, _ := NewProject("p1", "t1", "my-project", "dev-server-1")
	if _, err := p.Rebind(""); err != ErrEmptyDevServerID {
		t.Fatalf("expected ErrEmptyDevServerID, got %v", err)
	}
}
