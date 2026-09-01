package domain

import "testing"

func TestNewDevServerGroupGrant_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		groupID   string
		kind      GranteeKind
		granteeID string
		wantErr   error
	}{
		{"valid department grant", "t1", "g1", GranteeKindDepartment, "dept1", nil},
		{"valid team grant", "t1", "g1", GranteeKindTeam, "team1", nil},
		{"empty tenant", "", "g1", GranteeKindDepartment, "dept1", ErrEmptyGrantTenant},
		{"empty group", "t1", "", GranteeKindDepartment, "dept1", ErrEmptyGrantGroupID},
		{"invalid kind", "t1", "g1", GranteeKind("bogus"), "dept1", ErrInvalidGranteeKind},
		{"empty grantee", "t1", "g1", GranteeKindDepartment, "", ErrEmptyGranteeID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewDevServerGroupGrant("grant1", tt.tenantID, tt.groupID, tt.kind, tt.granteeID)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if g.ID != "grant1" || g.TenantID != tt.tenantID || g.DevServerGroupID != tt.groupID || g.GranteeKind != tt.kind || g.GranteeID != tt.granteeID {
					t.Errorf("unexpected grant: %+v", g)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGranteeKind_Valid(t *testing.T) {
	valid := []GranteeKind{GranteeKindDepartment, GranteeKindTeam}
	for _, k := range valid {
		if !k.Valid() {
			t.Errorf("expected %q to be valid", k)
		}
	}
	invalid := []GranteeKind{"", "bogus", "DEPARTMENT"}
	for _, k := range invalid {
		if k.Valid() {
			t.Errorf("expected %q to be invalid", k)
		}
	}
}
