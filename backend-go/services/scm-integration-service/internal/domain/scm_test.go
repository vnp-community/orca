package domain

import "testing"

func TestScmProvider_Valid(t *testing.T) {
	valid := []ScmProvider{ScmProviderGitHub, ScmProviderGitLab, ScmProviderBitbucket, ScmProviderAzureDevOps, ScmProviderGitea}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("expected %q to be valid", p)
		}
	}
	if ScmProvider("bogus").Valid() {
		t.Error("expected unknown provider to be invalid")
	}
	if ScmProvider("").Valid() {
		t.Error("expected empty provider to be invalid")
	}
}

func TestNewIssue_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		provider ScmProvider
		repo     string
		title    string
		wantErr  error
	}{
		{"valid", ScmProviderGitHub, "octocat/hello-world", "bug report", nil},
		{"invalid provider", ScmProvider("bogus"), "octocat/hello-world", "bug report", ErrInvalidProvider},
		{"empty repo", ScmProviderGitHub, "", "bug report", ErrEmptyRepo},
		{"empty title", ScmProviderGitHub, "octocat/hello-world", "", ErrEmptyTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIssue("1", tt.provider, tt.repo, tt.title, "open", "https://example.invalid/1")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewIssue_PopulatesFields(t *testing.T) {
	issue, err := NewIssue("42", ScmProviderGitHub, "octocat/hello-world", "bug report", "open", "https://example.invalid/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.ID != "42" || issue.Provider != ScmProviderGitHub || issue.Repo != "octocat/hello-world" ||
		issue.Title != "bug report" || issue.State != "open" || issue.URL != "https://example.invalid/42" {
		t.Errorf("unexpected issue fields: %+v", issue)
	}
}

func TestNewPullRequest_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		provider ScmProvider
		repo     string
		title    string
		wantErr  error
	}{
		{"valid", ScmProviderGitLab, "group/project", "feature branch", nil},
		{"invalid provider", ScmProvider("bogus"), "group/project", "feature branch", ErrInvalidProvider},
		{"empty repo", ScmProviderGitLab, "", "feature branch", ErrEmptyRepo},
		{"empty title", ScmProviderGitLab, "group/project", "", ErrEmptyTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPullRequest("1", tt.provider, tt.repo, tt.title, "open", "https://example.invalid/mr/1", "feature", "main")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewPullRequest_PopulatesFields(t *testing.T) {
	pr, err := NewPullRequest("7", ScmProviderGitLab, "group/project", "feature branch", "opened",
		"https://example.invalid/mr/7", "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.HeadBranch != "feature" || pr.BaseBranch != "main" {
		t.Errorf("unexpected branch fields: %+v", pr)
	}
}
