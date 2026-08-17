package domain

import "testing"

func TestNewProjectGroup_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		gname    string
		wantErr  error
	}{
		{"valid", "t1", "group-a", nil},
		{"empty tenant", "", "group-a", ErrEmptyTenantID},
		{"empty name", "t1", "", ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProjectGroup("g1", tt.tenantID, tt.gname, "")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewProjectGroup_RejectsSelfParent(t *testing.T) {
	if _, err := NewProjectGroup("g1", "t1", "group-a", "g1"); err != ErrGroupSelfParent {
		t.Fatalf("expected ErrGroupSelfParent, got %v", err)
	}
}

func TestNewProjectGroup_AllowsDistinctParent(t *testing.T) {
	g, err := NewProjectGroup("g2", "t1", "group-b", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ParentGroupID != "g1" {
		t.Errorf("expected ParentGroupID=g1, got %q", g.ParentGroupID)
	}
}
