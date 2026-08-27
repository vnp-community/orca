package domain

import "testing"

func TestNewTeam_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		companyID string
		teamName  string
		wantErr   error
	}{
		{"valid", "t1", "c1", "Platform", nil},
		{"empty id", "", "c1", "Platform", ErrEmptyID},
		{"empty company id", "t1", "", "Platform", ErrEmptyID},
		{"empty name", "t1", "c1", "", ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTeam(tt.id, tt.companyID, tt.teamName, nil)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewTeamMember_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name    string
		teamID  string
		userID  string
		wantErr error
	}{
		{"valid", "t1", "u1", nil},
		{"empty team id", "", "u1", ErrEmptyID},
		{"empty user id", "t1", "", ErrEmptyUserID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTeamMember(tt.teamID, tt.userID, 5)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if got.Priority != 5 {
					t.Errorf("expected Priority=5, got %d", got.Priority)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
