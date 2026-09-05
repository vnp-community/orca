package wscompat

import "testing"

func TestParseGitHubOwnerRepo(t *testing.T) {
	cases := []struct {
		name      string
		remoteURL string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"SSH form", "git@github.com:acme/widgets.git", "acme", "widgets", false},
		{"SSH form without .git suffix", "git@github.com:acme/widgets", "acme", "widgets", false},
		{"HTTPS form", "https://github.com/acme/widgets.git", "acme", "widgets", false},
		{"HTTPS form without .git suffix", "https://github.com/acme/widgets", "acme", "widgets", false},
		{"ssh.github.com alias normalizes to github.com", "ssh://git@ssh.github.com/acme/widgets.git", "acme", "widgets", false},
		{"HTTPS with userinfo", "https://x-access-token@github.com/acme/widgets.git", "acme", "widgets", false},
		{"non-GitHub host is rejected", "git@gitlab.com:acme/widgets.git", "", "", true},
		{"GitHub Enterprise host is rejected", "https://github.mycompany.com/acme/widgets.git", "", "", true},
		{"a bare filesystem path is rejected", "/opt/aiops-v3", "", "", true},
		{"malformed path segment count is rejected", "https://github.com/acme/widgets/extra", "", "", true},
		{"empty string is rejected", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGitHubOwnerRepo(tc.remoteURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseGitHubOwnerRepo(%q) = %+v, want an error", tc.remoteURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGitHubOwnerRepo(%q) unexpected error: %v", tc.remoteURL, err)
			}
			if got.Owner != tc.wantOwner || got.Repo != tc.wantRepo {
				t.Errorf("parseGitHubOwnerRepo(%q) = %+v, want owner=%q repo=%q", tc.remoteURL, got, tc.wantOwner, tc.wantRepo)
			}
		})
	}
}
