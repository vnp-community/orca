package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestCheckHostedReviewEligibility_NotConnected(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: false})
	provider := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Eligible || result.IneligibleReason != "NOT_CONNECTED" {
		t.Fatalf("expected NOT_CONNECTED, got %+v", result)
	}
	if provider.calls != 0 {
		t.Errorf("expected provider.BranchExists NOT called when auth check fails first, got %d calls", provider.calls)
	}
}

func TestCheckHostedReviewEligibility_BranchNotFound(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExists: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Eligible || result.IneligibleReason != "BRANCH_NOT_FOUND" {
		t.Fatalf("expected BRANCH_NOT_FOUND, got %+v", result)
	}
}

func TestCheckHostedReviewEligibility_ReviewAlreadyExists(t *testing.T) {
	existing, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "open", "url", "feature-x", "main")
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExists: true, branchPR: existing, branchFound: true}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Eligible || result.IneligibleReason != "REVIEW_ALREADY_EXISTS" {
		t.Fatalf("expected REVIEW_ALREADY_EXISTS, got %+v", result)
	}
	if result.ExistingPullRequest.ID != "1" {
		t.Errorf("expected ExistingPullRequest to be set, got %+v", result.ExistingPullRequest)
	}
}

func TestCheckHostedReviewEligibility_Eligible(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExists: true, branchFound: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	result, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Eligible || result.IneligibleReason != "" {
		t.Fatalf("expected eligible with no reason, got %+v", result)
	}
}

func TestCheckHostedReviewEligibility_PropagatesBranchExistsFailure(t *testing.T) {
	resolver := &fakeCredentialResolver{token: "tok"}
	getAuthStatus := NewGetAuthStatus(&fakeCredentialResolverConnectedFlag{connected: true})
	provider := &fakeProvider{branchExistsErr: errors.New("provider unavailable")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewCheckHostedReviewEligibility(resolver, registry, getAuthStatus)

	_, err := uc.Execute(context.Background(), CheckHostedReviewEligibilityParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err == nil {
		t.Fatal("expected an error when BranchExists fails")
	}
}

func TestCheckHostedReviewEligibility_RequiresTenantRepoBranch(t *testing.T) {
	uc := NewCheckHostedReviewEligibility(&fakeCredentialResolver{}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}}, NewGetAuthStatus(&fakeCredentialResolver{}))
	cases := []CheckHostedReviewEligibilityParams{
		{Repo: "o/r", HeadBranch: "b"},
		{TenantID: "t1", HeadBranch: "b"},
		{TenantID: "t1", Repo: "o/r"},
	}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
