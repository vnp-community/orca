package domain

import "testing"

func TestNewDepartment_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		companyID string
		deptName  string
		wantErr   error
	}{
		{"valid", "d1", "c1", "Engineering", nil},
		{"empty id", "", "c1", "Engineering", ErrEmptyID},
		{"empty company id", "d1", "", "Engineering", ErrEmptyID},
		{"empty name", "d1", "c1", "", ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDepartment(tt.id, tt.companyID, tt.deptName, nil)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
