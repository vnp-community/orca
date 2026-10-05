package grpc

import (
	"strings"
	"testing"
)

func TestValidateLinkedIssue(t *testing.T) {
	cases := []struct {
		name, provider, ref, site string
		wantSite                  string
		wantErr                   bool
	}{
		{"none", "", "", "", "", false},
		{"jira", "jira", "ENG-1", "", "", false},
		{"jira with site trimmed", "jira", "ENG-1", "  https://a.atlassian.net ", "https://a.atlassian.net", false},
		{"github", "github", "o/r#1", "", "", false},
		{"provider only", "jira", "", "", "", true},
		{"ref only", "", "ENG-1", "", "", true},
		{"unknown provider", "bitbucket", "X-1", "", "", true},
		{"wrong case", "Jira", "ENG-1", "", "", true},
		{"site without link", "", "", "https://a.atlassian.net", "", true},
		{"site max length", "jira", "ENG-1", strings.Repeat("a", 512), strings.Repeat("a", 512), false},
		{"site too long", "jira", "ENG-1", strings.Repeat("a", 513), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site, err := validateLinkedIssue(c.provider, c.ref, c.site)
			if (err != nil) != c.wantErr {
				t.Fatalf("validateLinkedIssue(%q,%q,%q) err=%v, wantErr=%v", c.provider, c.ref, c.site, err, c.wantErr)
			}
			if site != c.wantSite {
				t.Fatalf("site=%q, want %q", site, c.wantSite)
			}
		})
	}
}
