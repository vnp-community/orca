package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestListIssueCommentsBySlug_EmptySlugFailsBeforeProviderCall(t *testing.T) {
	github := &fakeGitHubProjects{}
	uc := NewListIssueCommentsBySlug(&fakeCredentialResolver{token: "tok"}, github)

	_, err := uc.Execute(context.Background(), ListIssueCommentsBySlugParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "",
	})
	if err == nil {
		t.Fatal("expected an error for an empty item_slug")
	}
	if github.calls != 0 {
		t.Fatalf("expected no provider call for an empty item_slug, got %d calls", github.calls)
	}
}

func TestListIssueCommentsBySlug_HappyPathPassesThroughUnchanged(t *testing.T) {
	want := []ProjectComment{{ID: "c-1", Body: "looks good", Author: "octocat"}}
	github := &fakeGitHubProjects{comments: want}
	uc := NewListIssueCommentsBySlug(&fakeCredentialResolver{token: "tok"}, github)

	got, err := uc.Execute(context.Background(), ListIssueCommentsBySlugParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "o/r#42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("expected the fake provider's comments back unchanged, got %+v", got)
	}
	if github.lastItemSlug != "o/r#42" {
		t.Fatalf("expected item_slug to reach the provider, got %q", github.lastItemSlug)
	}
}
