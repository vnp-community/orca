package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestRequestPullRequestReviewers_Success(t *testing.T) {
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "open", "url", "head", "base")
	provider := &fakeProvider{reviewersPR: pr}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewRequestPullRequestReviewers(&fakeCredentialResolver{token: "tok"}, registry)

	got, err := uc.Execute(context.Background(), RequestPullRequestReviewersParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1, ReviewerLogins: []string{"alice"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "1" || provider.calls != 1 {
		t.Fatalf("unexpected result: %+v calls=%d", got, provider.calls)
	}
}

func TestRequestPullRequestReviewers_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeProvider{reviewersErr: errors.New("not found")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewRequestPullRequestReviewers(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), RequestPullRequestReviewersParams{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestRequestPullRequestReviewers_RequiresTenantAndRepo(t *testing.T) {
	uc := NewRequestPullRequestReviewers(&fakeCredentialResolver{}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}})
	cases := []RequestPullRequestReviewersParams{{Repo: "o/r"}, {TenantID: "t1"}}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
