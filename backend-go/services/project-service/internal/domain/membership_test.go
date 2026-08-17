package domain

import "testing"

func TestNewProjectMember_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		userID    string
		role      ProjectRole
		wantErr   error
	}{
		{"valid owner", "p1", "u1", ProjectRoleOwner, nil},
		{"valid member", "p1", "u1", ProjectRoleMember, nil},
		{"empty project", "", "u1", ProjectRoleMember, ErrEmptyProjectID},
		{"empty user", "p1", "", ProjectRoleMember, ErrEmptyUserID},
		{"invalid role", "p1", "u1", ProjectRole("bogus"), ErrInvalidRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProjectMember(tt.projectID, tt.userID, tt.role)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
