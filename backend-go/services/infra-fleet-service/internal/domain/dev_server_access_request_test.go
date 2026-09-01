package domain

import "testing"

func TestNewDevServerAccessRequest_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		tenantID  string
		userID    string
		groupID   string
		kind      GranteeKind
		granteeID string
		wantErr   error
	}{
		{"valid", "t1", "u1", "g1", GranteeKindDepartment, "dept1", nil},
		{"empty tenant", "", "u1", "g1", GranteeKindDepartment, "dept1", ErrEmptyAccessRequestTenant},
		{"empty user", "t1", "", "g1", GranteeKindDepartment, "dept1", ErrEmptyAccessRequestUser},
		{"empty group", "t1", "u1", "", GranteeKindDepartment, "dept1", ErrEmptyAccessRequestGroupID},
		{"invalid kind", "t1", "u1", "g1", GranteeKind("bogus"), "dept1", ErrInvalidGranteeKind},
		{"empty grantee", "t1", "u1", "g1", GranteeKindDepartment, "", ErrEmptyGranteeID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewDevServerAccessRequest("req1", tt.tenantID, tt.userID, tt.groupID, "please", tt.kind, tt.granteeID, 1000)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if r.Status != AccessRequestStatusPending {
					t.Errorf("want new request status=pending, got %q", r.Status)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAccessRequestStatus_Valid(t *testing.T) {
	valid := []AccessRequestStatus{AccessRequestStatusPending, AccessRequestStatusApproved, AccessRequestStatusRejected}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("expected %q to be valid", s)
		}
	}
	invalid := []AccessRequestStatus{"", "bogus", "PENDING"}
	for _, s := range invalid {
		if s.Valid() {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}
