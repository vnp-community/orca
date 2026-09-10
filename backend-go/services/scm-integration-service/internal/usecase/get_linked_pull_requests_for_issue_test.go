package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestGetLinkedPullRequestsForIssue_SupportedProviderReturnsResults(t *testing.T) {
	pr, err := domain.NewPullRequest("9", domain.ScmProviderGitHub, "o/r", "fix", "open", "https://example.invalid/pull/9", "fix", "main")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	github := &fakeProvider{linkedPRs: []domain.PullRequest{pr}, linkedPRsSupported: true}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	uc := NewGetLinkedPullRequestsForIssue(&fakeCredentialResolver{token: "tok"}, registry)

	out, err := uc.Execute(context.Background(), GetLinkedPullRequestsForIssueInput{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", IssueNumber: 42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.CapabilityUnsupported {
		t.Error("expected CapabilityUnsupported=false for a provider that supports the query")
	}
	if len(out.PullRequests) != 1 || out.PullRequests[0].ID != "9" {
		t.Fatalf("expected the fake provider's linked PRs back, got %+v", out.PullRequests)
	}
}

func TestGetLinkedPullRequestsForIssue_UnsupportedProviderDegradesWithoutError(t *testing.T) {
	provider := &fakeProvider{linkedPRsSupported: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderBitbucket: provider}}
	uc := NewGetLinkedPullRequestsForIssue(&fakeCredentialResolver{token: "tok"}, registry)

	out, err := uc.Execute(context.Background(), GetLinkedPullRequestsForIssueInput{
		TenantID: "t1", Provider: domain.ScmProviderBitbucket, Repo: "o/r", IssueNumber: 42,
	})
	if err != nil {
		t.Fatalf("expected a capability-unsupported provider to degrade, not error: %v", err)
	}
	if !out.CapabilityUnsupported {
		t.Error("expected CapabilityUnsupported=true")
	}
	if len(out.PullRequests) != 0 {
		t.Fatalf("expected an empty list, got %+v", out.PullRequests)
	}
}
