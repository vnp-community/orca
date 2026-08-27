package domain

import "testing"

func TestNewUserProfile_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		companyID string
		wantErr   error
	}{
		{"valid", "u1", "c1", nil},
		{"empty user id", "", "c1", ErrEmptyUserID},
		{"empty company id", "u1", "", ErrEmptyID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUserProfile(tt.userID, tt.companyID, "", nil)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewUserProfile_AllowsEmptyDepartmentID(t *testing.T) {
	profile, err := NewUserProfile("u1", "c1", "", Settings{"agent": Settings{"model": "opus"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.DepartmentID != "" {
		t.Errorf("expected empty DepartmentID (company-only inheritance), got %q", profile.DepartmentID)
	}
}
