package domain

import "testing"

func TestNewCompany_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		company string
		wantErr error
	}{
		{"valid", "c1", "Acme", nil},
		{"empty id", "", "Acme", ErrEmptyID},
		{"empty name", "c1", "", ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCompany(tt.id, tt.company, nil)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if got.Settings == nil {
					t.Error("expected NewCompany to normalize a nil Settings to a non-nil empty map")
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
