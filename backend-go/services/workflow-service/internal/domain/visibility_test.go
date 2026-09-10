package domain

import "testing"

func TestVisibility_CanEscalateTo(t *testing.T) {
	tests := []struct {
		name string
		from Visibility
		to   Visibility
		want bool
	}{
		{"private to team allowed (one tier forward)", VisibilityPrivate, VisibilityTeam, true},
		{"team to company allowed (one tier forward)", VisibilityTeam, VisibilityCompany, true},
		{"company to public allowed (one tier forward)", VisibilityCompany, VisibilityPublic, true},
		{"private to company rejected (skips team)", VisibilityPrivate, VisibilityCompany, false},
		{"private to public rejected (skips two tiers)", VisibilityPrivate, VisibilityPublic, false},
		{"team to private allowed (unpublish, any distance)", VisibilityTeam, VisibilityPrivate, true},
		{"public to private allowed (unpublish, any distance)", VisibilityPublic, VisibilityPrivate, true},
		{"company to private allowed (unpublish)", VisibilityCompany, VisibilityPrivate, true},
		{"team to public rejected (skips company, not an unpublish)", VisibilityTeam, VisibilityPublic, false},
		{"same tier rejected (not a forward step)", VisibilityTeam, VisibilityTeam, false},
		{"backward non-private rejected (company to team)", VisibilityCompany, VisibilityTeam, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.from.CanEscalateTo(tt.to); got != tt.want {
				t.Errorf("%s.CanEscalateTo(%s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestVisibility_Valid(t *testing.T) {
	for _, v := range []Visibility{VisibilityPrivate, VisibilityTeam, VisibilityCompany, VisibilityPublic} {
		if !v.Valid() {
			t.Errorf("expected %q to be valid", v)
		}
	}
	if Visibility("bogus").Valid() {
		t.Error("expected an unknown visibility to be invalid")
	}
}

func TestWorkflowTemplate_AverageRating(t *testing.T) {
	tests := []struct {
		name  string
		sum   int32
		count int32
		want  float64
	}{
		{"no ratings yet", 0, 0, 0},
		{"all five stars", 15, 3, 5},
		{"mixed ratings", 10, 4, 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := WorkflowTemplate{RatingSum: tt.sum, RatingCount: tt.count}
			if got := tmpl.AverageRating(); got != tt.want {
				t.Errorf("AverageRating() = %v, want %v", got, tt.want)
			}
		})
	}
}
