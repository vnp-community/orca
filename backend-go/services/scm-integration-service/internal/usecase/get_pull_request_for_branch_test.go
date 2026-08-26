package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestGetPullRequestForBranch_FoundSuccess(t *testing.T) {
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "open", "url", "feature-x", "main")
	provider := &fakeProvider{branchPR: pr, branchFound: true}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewGetPullRequestForBranch(&fakeCredentialResolver{token: "tok"}, registry)

	got, err := uc.Execute(context.Background(), GetPullRequestForBranchParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Found || got.PullRequest.ID != "1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestGetPullRequestForBranch_NotFound(t *testing.T) {
	provider := &fakeProvider{branchFound: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewGetPullRequestForBranch(&fakeCredentialResolver{token: "tok"}, registry)

	got, err := uc.Execute(context.Background(), GetPullRequestForBranchParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Found {
		t.Fatalf("expected not found, got %+v", got)
	}
}

func TestGetPullRequestForBranch_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeProvider{branchErr: errors.New("provider unavailable")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewGetPullRequestForBranch(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), GetPullRequestForBranchParams{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", HeadBranch: "feature-x"})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestGetPullRequestForBranch_RequiresTenantRepoBranch(t *testing.T) {
	uc := NewGetPullRequestForBranch(&fakeCredentialResolver{}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}})
	cases := []GetPullRequestForBranchParams{
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
