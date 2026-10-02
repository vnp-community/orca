package grpc

import "testing"

func TestValidateLinkedIssue(t *testing.T) {
	cases := []struct {
		name, provider, ref string
		wantErr             bool
	}{
		{"none", "", "", false},
		{"jira", "jira", "ENG-1", false},
		{"github", "github", "o/r#1", false},
		{"provider only", "jira", "", true},
		{"ref only", "", "ENG-1", true},
		{"unknown provider", "bitbucket", "X-1", true},
		{"wrong case", "Jira", "ENG-1", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateLinkedIssue(c.provider, c.ref); (err != nil) != c.wantErr {
				t.Fatalf("validateLinkedIssue(%q,%q) err=%v, wantErr=%v", c.provider, c.ref, err, c.wantErr)
			}
		})
	}
}
