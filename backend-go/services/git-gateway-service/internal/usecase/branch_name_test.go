package usecase

import "testing"

// TestGenerateBranchName is BR-PI-04's table-driven regression guard: exact
// "type/description-issueId" shape, label->type inference, kebab-casing,
// and 40-char truncation.
func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		labels      []string
		externalRef string
		want        string
	}{
		{
			name: "bug label infers fix type", title: "Login button is broken",
			labels: []string{"bug"}, externalRef: "owner/repo#42",
			want: "fix/login-button-is-broken-owner-repo-42",
		},
		{
			name: "enhancement label infers feat type", title: "Add dark mode",
			labels: []string{"enhancement"}, externalRef: "owner/repo#7",
			want: "feat/add-dark-mode-owner-repo-7",
		},
		{
			name: "feature label infers feat type", title: "Support SSO login",
			labels: []string{"feature"}, externalRef: "owner/repo#8",
			want: "feat/support-sso-login-owner-repo-8",
		},
		{
			name: "no matching label falls back to chore", title: "Update dependencies",
			labels: []string{"dependencies"}, externalRef: "owner/repo#9",
			want: "chore/update-dependencies-owner-repo-9",
		},
		{
			name: "no labels at all falls back to chore", title: "Refactor internals",
			labels: nil, externalRef: "ENG-123",
			want: "chore/refactor-internals-eng-123",
		},
		{
			name: "label match is case-insensitive", title: "Fix crash on startup",
			labels: []string{"Bug"}, externalRef: "owner/repo#10",
			want: "fix/fix-crash-on-startup-owner-repo-10",
		},
		{
			name:   "title longer than 40 chars is truncated before kebab-casing",
			title:  "This is a very long issue title that definitely exceeds forty characters",
			labels: []string{"bug"}, externalRef: "owner/repo#11",
			// truncate(title, 40) = first 40 bytes of the title, kebab-cased.
			want: "fix/this-is-a-very-long-issue-title-that-def-owner-repo-11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateBranchName(tt.title, tt.labels, tt.externalRef)
			if got != tt.want {
				t.Errorf("generateBranchName(%q, %v, %q) = %q, want %q", tt.title, tt.labels, tt.externalRef, got, tt.want)
			}
		})
	}
}

func TestGenerateBranchName_ShapeIsTypeSlashDescriptionDashIssueID(t *testing.T) {
	got := generateBranchName("Fix login bug", []string{"bug"}, "acme/widgets#99")
	want := "fix/fix-login-bug-acme-widgets-99"
	if got != want {
		t.Fatalf("expected the exact type/description-issueId shape, got %q want %q", got, want)
	}
}

func TestKebabCase(t *testing.T) {
	tests := map[string]string{
		"Simple Title":         "simple-title",
		"  leading/trailing  ": "leading-trailing",
		"multi   space":        "multi-space",
		"Already-Kebab":        "already-kebab",
		"punct!@#$%^&*()here":  "punct-here",
	}
	for in, want := range tests {
		if got := kebabCase(in); got != want {
			t.Errorf("kebabCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 40); got != "short" {
		t.Errorf("expected short strings to pass through unchanged, got %q", got)
	}
	long := "0123456789012345678901234567890123456789extra"
	got := truncate(long, 40)
	if len(got) != 40 {
		t.Errorf("expected truncation to exactly 40 chars, got %d chars: %q", len(got), got)
	}
}
