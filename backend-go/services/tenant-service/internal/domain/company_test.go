package domain

import (
	"errors"
	"testing"
)

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

func TestValidateCompanySettings(t *testing.T) {
	tests := []struct {
		name    string
		s       Settings
		wantErr error
	}{
		{"no agent/security section", Settings{}, nil},
		{"approved model", Settings{"agent": Settings{"approvedModels": []any{"claude-opus-4-5", "codex"}}}, nil},
		{"unapproved model", Settings{"agent": Settings{"approvedModels": []any{"gpt-99"}}}, ErrUnsupportedModel},
		{"timeout in range", Settings{"security": Settings{"sessionTimeoutHours": float64(24)}}, nil},
		{"timeout at lower bound", Settings{"security": Settings{"sessionTimeoutHours": float64(1)}}, nil},
		{"timeout at upper bound", Settings{"security": Settings{"sessionTimeoutHours": float64(168)}}, nil},
		{"timeout below range", Settings{"security": Settings{"sessionTimeoutHours": float64(0)}}, ErrSessionTimeoutRange},
		{"timeout above range", Settings{"security": Settings{"sessionTimeoutHours": float64(169)}}, ErrSessionTimeoutRange},
		{"security absent field is no-op", Settings{"security": Settings{}}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCompanySettings(tt.s)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
