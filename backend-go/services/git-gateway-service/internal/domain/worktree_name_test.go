package domain_test

import (
	"testing"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestValidateWorktreeName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
	}{
		{"lowercase-dash-underscore-digits", "feature-123_abc", true},
		{"rejects-uppercase", "Feature", false},
		{"rejects-spaces", "my feature", false},
		{"rejects-unicode", "fïx", false},
		{"rejects-empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateWorktreeName(tc.input)
			if tc.valid && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("expected invalid, got nil")
			}
		})
	}
}

func TestSuggestAlternateName_WalksCollisions(t *testing.T) {
	taken := map[string]bool{"foo": true, "foo-2": true}
	if got := domain.SuggestAlternateName("foo", taken); got != "foo-3" {
		t.Fatalf("got %q, want foo-3", got)
	}
	if got := domain.SuggestAlternateName("bar", taken); got != "bar" {
		t.Fatalf("got %q, want bar (untaken)", got)
	}
}
