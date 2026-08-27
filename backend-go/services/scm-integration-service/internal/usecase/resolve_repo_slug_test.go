package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestResolveRepoSlug_Success(t *testing.T) {
	provider := &fakeProvider{slugOwner: "octocat", slugName: "hello-world"}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewResolveRepoSlug(&fakeCredentialResolver{token: "tok"}, registry)

	got, err := uc.Execute(context.Background(), ResolveRepoSlugParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Candidate: "git@github.com:octocat/hello-world.git",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != "octocat/hello-world" {
		t.Fatalf("unexpected slug: %+v", got)
	}
}

func TestResolveRepoSlug_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeProvider{slugErr: errors.New("not found")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewResolveRepoSlug(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), ResolveRepoSlugParams{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Candidate: "x/y"})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestResolveRepoSlug_RequiresTenantAndCandidate(t *testing.T) {
	uc := NewResolveRepoSlug(&fakeCredentialResolver{}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}})
	cases := []ResolveRepoSlugParams{{Candidate: "x/y"}, {TenantID: "t1"}}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
