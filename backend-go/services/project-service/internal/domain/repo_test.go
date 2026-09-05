package domain

import "testing"

func TestNewRepo_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		url       string
		wantErr   error
	}{
		{"valid", "p1", "https://github.com/org/repo", nil},
		{"empty project id", "", "https://github.com/org/repo", ErrEmptyProjectID},
		{"empty url", "p1", "", ErrEmptyRepoURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRepo("r1", tt.projectID, tt.url, "display", "")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewRepo_StartsAtZeroPosition(t *testing.T) {
	r, err := NewRepo("r1", "p1", "https://github.com/org/repo", "display", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Position != 0 {
		t.Errorf("expected a freshly constructed repo to have position 0, got %d", r.Position)
	}
}

func TestNewRepo_CarriesDevServerID(t *testing.T) {
	r, err := NewRepo("r1", "p1", "https://github.com/org/repo", "display", "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.DevServerID != "ds-1" {
		t.Errorf("expected DevServerID %q, got %q", "ds-1", r.DevServerID)
	}
}
