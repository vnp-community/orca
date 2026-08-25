package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestMergePullRequest_Success(t *testing.T) {
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "closed", "url", "head", "base")
	provider := &fakeProvider{mergedPR: pr, merged: true, mergeSHA: "abc123"}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewMergePullRequest(&fakeCredentialResolver{token: "tok"}, registry)

	result, err := uc.Execute(context.Background(), MergePullRequestParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1, MergeMethod: "squash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Merged || result.SHA != "abc123" {
		t.Errorf("expected merged=true sha=abc123, got merged=%v sha=%s", result.Merged, result.SHA)
	}
	if provider.calls != 1 {
		t.Errorf("expected provider called exactly once, got %d", provider.calls)
	}
}

func TestMergePullRequest_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeProvider{mergeErr: errors.New("merge conflict")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewMergePullRequest(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), MergePullRequestParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1,
	})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestMergePullRequest_RequiresTenantRepoNumber(t *testing.T) {
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}}
	uc := NewMergePullRequest(&fakeCredentialResolver{}, registry)

	cases := []MergePullRequestParams{
		{Repo: "o/r", Number: 1},
		{TenantID: "tenant-1", Number: 1},
		{TenantID: "tenant-1", Repo: "o/r"},
	}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
