package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestSuggestPullRequestReviewers_StopsAtFirstFoundPath(t *testing.T) {
	provider := &fakeProvider{repoFileFound: true, repoFileContent: "*.go @go-team"}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	uc := NewSuggestPullRequestReviewers(creds, registry)

	result, err := uc.Execute(context.Background(), SuggestPullRequestReviewersParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b", BaseRef: "main",
		ChangedFiles: []string{"main.go"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Fatal("expected Found=true")
	}
	if len(result.ReviewerLogins) != 1 || result.ReviewerLogins[0] != "go-team" {
		t.Errorf("expected go-team suggested, got %v", result.ReviewerLogins)
	}
	// The first canonical path ("CODEOWNERS") is found immediately — the
	// provider's GetRepoFileContent must be called exactly once, not for
	// every candidate path.
	if provider.getRepoFileCalls != 1 {
		t.Errorf("expected exactly 1 GetRepoFileContent call (stops at first found), got %d", provider.getRepoFileCalls)
	}
	if provider.lastGetRepoFilePath != "CODEOWNERS" {
		t.Errorf("expected the first canonical path to be tried first, got %q", provider.lastGetRepoFilePath)
	}
}

func TestSuggestPullRequestReviewers_TriesAllPathsInOrderWhenNoneFound(t *testing.T) {
	provider := &fakeProvider{repoFileFound: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	uc := NewSuggestPullRequestReviewers(creds, registry)

	result, err := uc.Execute(context.Background(), SuggestPullRequestReviewersParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b", BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Found {
		t.Error("expected Found=false when no CODEOWNERS file exists at any canonical path")
	}
	if provider.getRepoFileCalls != len(codeownersPaths) {
		t.Errorf("expected all %d canonical paths to be tried, got %d calls", len(codeownersPaths), provider.getRepoFileCalls)
	}
}

func TestSuggestPullRequestReviewers_RequiresTenant(t *testing.T) {
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: &fakeProvider{}}}
	uc := NewSuggestPullRequestReviewers(&fakeCredentialResolver{}, registry)

	_, err := uc.Execute(context.Background(), SuggestPullRequestReviewersParams{Repo: "a/b"})
	if err == nil {
		t.Error("expected error when tenant_id is missing")
	}
}
