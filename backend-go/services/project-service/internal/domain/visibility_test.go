package domain

import "testing"

func TestValidVisibility(t *testing.T) {
	tests := []struct {
		visibility string
		want       bool
	}{
		{VisibilityPrivate, true},
		{VisibilityTeam, true},
		{VisibilityDepartment, true},
		{VisibilityCompany, true},
		{"", false},
		{"bogus", false},
	}

	for _, tt := range tests {
		t.Run(tt.visibility, func(t *testing.T) {
			if got := ValidVisibility(tt.visibility); got != tt.want {
				t.Errorf("ValidVisibility(%q) = %v, want %v", tt.visibility, got, tt.want)
			}
		})
	}
}
