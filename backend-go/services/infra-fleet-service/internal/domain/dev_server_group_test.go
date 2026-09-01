package domain

import "testing"

func TestNewDevServerGroup_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name          string
		tenantID      string
		groupName     string
		parentGroupID string
		wantErr       error
	}{
		{"valid root group", "t1", "Backend Team", "", nil},
		{"valid child group", "t1", "Backend Team - Staging", "parent-1", nil},
		{"empty tenant", "", "Backend Team", "", ErrEmptyDevServerGroupTenant},
		{"empty name", "t1", "", "", ErrEmptyDevServerGroupName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewDevServerGroup("g1", tt.tenantID, tt.groupName, tt.parentGroupID)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if g.ID != "g1" || g.TenantID != tt.tenantID || g.Name != tt.groupName || g.ParentGroupID != tt.parentGroupID {
					t.Errorf("unexpected DevServerGroup: %+v", g)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
