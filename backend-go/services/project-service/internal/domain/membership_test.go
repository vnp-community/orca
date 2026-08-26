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

func TestAssertNotLastOwnerRemoval(t *testing.T) {
	tests := []struct {
		name                   string
		currentOwnerCount      int
		targetIsCurrentlyOwner bool
		targetRoleAfter        ProjectRole
		wantErr                error
	}{
		{
			name:                   "last owner removed",
			currentOwnerCount:      1,
			targetIsCurrentlyOwner: true,
			targetRoleAfter:        "",
			wantErr:                ErrProjectWouldBeOwnerless,
		},
		{
			name:                   "last owner demoted",
			currentOwnerCount:      1,
			targetIsCurrentlyOwner: true,
			targetRoleAfter:        ProjectRoleMember,
			wantErr:                ErrProjectWouldBeOwnerless,
		},
		{
			name:                   "non-last owner removed",
			currentOwnerCount:      2,
			targetIsCurrentlyOwner: true,
			targetRoleAfter:        "",
			wantErr:                nil,
		},
		{
			name:                   "non-last owner demoted",
			currentOwnerCount:      2,
			targetIsCurrentlyOwner: true,
			targetRoleAfter:        ProjectRoleMember,
			wantErr:                nil,
		},
		{
			name:                   "removing a non-owner never errors, even at count 1",
			currentOwnerCount:      1,
			targetIsCurrentlyOwner: false,
			targetRoleAfter:        "",
			wantErr:                nil,
		},
		{
			name:                   "removing a non-owner never errors, even at count 0",
			currentOwnerCount:      0,
			targetIsCurrentlyOwner: false,
			targetRoleAfter:        "",
			wantErr:                nil,
		},
		{
			name:                   "promotion never blocked regardless of owner count",
			currentOwnerCount:      1,
			targetIsCurrentlyOwner: false,
			targetRoleAfter:        ProjectRoleOwner,
			wantErr:                nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AssertNotLastOwnerRemoval(tt.currentOwnerCount, tt.targetIsCurrentlyOwner, tt.targetRoleAfter)
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
