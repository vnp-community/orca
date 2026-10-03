package usecase

import "testing"

func TestParseLinkedJiraIssue(t *testing.T) {
	cases := []struct {
		name                string
		branch, title, body string
		want                string
	}{
		{"branch with slash", "feature/ENG-123-fix-login", "", "", "ENG-123"},
		{"branch wins over title", "ENG-1-a", "ENG-2 thing", "Fixes ENG-3", "ENG-1"},
		{"title when branch has none", "main-fix", "[ABC2-45] add thing", "", "ABC2-45"},
		{"title wins over body", "x", "ENG-2: y", "Fixes ENG-3", "ENG-2"},
		{"body closing keyword beats earlier bare mention", "x", "y", "See OPS-9 for context.\nFixes ENG-3", "ENG-3"},
		{"body closing keyword with colon", "x", "y", "Resolves: ENG-8", "ENG-8"},
		{"body bare mention", "x", "y", "relates to ENG-4", "ENG-4"},
		{"underscore boundary", "ENG-5_fix", "", "", "ENG-5"},
		{"lowercase ignored", "eng-123-fix", "eng-1", "eng-2", ""},
		{"UTF-8", "x", "support UTF-8 input", "", ""},
		{"SHA-256", "x", "use SHA-256", "", ""},
		{"ISO-8601", "x", "", "dates are ISO-8601", ""},
		{"CVE", "x", "fix CVE-2024-1234", "", ""},
		{"deny-listed then real key", "x", "UTF-8 handling for ENG-7", "", "ENG-7"},
		{"preceded by letter", "xENG-1", "", "", ""},
		{"followed by letter", "ENG-1a", "", "", ""},
		{"preceded by digit", "1ENG-1", "", "", ""},
		{"second numeric segment", "ENG-12-3", "", "", ""},
		{"single-letter project", "A-1", "", "", ""},
		{"empty", "", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, ref := ParseLinkedJiraIssue(tc.branch, tc.title, tc.body)
			if ref != tc.want {
				t.Fatalf("ref = %q, want %q", ref, tc.want)
			}
			if tc.want != "" && provider != "jira" || tc.want == "" && provider != "" {
				t.Errorf("provider = %q for ref %q", provider, ref)
			}
		})
	}
}
