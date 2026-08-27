package domain

import "testing"

func TestProvider_Valid(t *testing.T) {
	tests := []struct {
		name string
		p    Provider
		want bool
	}{
		{"jira", ProviderJira, true},
		{"linear", ProviderLinear, true},
		{"empty", Provider(""), false},
		{"bogus", Provider("bogus"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Valid(); got != tt.want {
				t.Errorf("Provider(%q).Valid() = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestNewIssue_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		title   string
		state   string
		url     string
		wantErr error
	}{
		{"valid", "PROJ-1", "Fix the bug", "In Progress", "https://example.atlassian.net/browse/PROJ-1", nil},
		{"empty id", "", "Fix the bug", "In Progress", "https://example.atlassian.net/browse/PROJ-1", ErrEmptyID},
		{"empty title", "PROJ-1", "", "In Progress", "https://example.atlassian.net/browse/PROJ-1", ErrEmptyTitle},
		{"empty state and url allowed", "PROJ-1", "Fix the bug", "", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue, err := NewIssue(tt.id, tt.title, tt.state, tt.url)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if issue.ID != tt.id || issue.Title != tt.title || issue.State != tt.state || issue.URL != tt.url {
					t.Errorf("unexpected issue: %+v", issue)
				}
				return
			}
			if err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
