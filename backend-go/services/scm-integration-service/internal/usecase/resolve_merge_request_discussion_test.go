package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestResolveMergeRequestDiscussion_Success(t *testing.T) {
	provider := &fakeGitLabMergeRequestProvider{disc: domain.MergeRequestDiscussion{ID: "disc-1", Resolved: true, ResolvedBy: "alice"}}
	uc := NewResolveMergeRequestDiscussion(&fakeCredentialResolver{token: "tok"}, provider)

	got, err := uc.Execute(context.Background(), ResolveMergeRequestDiscussionParams{
		TenantID: "tenant-1", Repo: "group/project", MergeRequestIID: 42, DiscussionID: "disc-1", Resolved: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Resolved || got.ResolvedBy != "alice" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if provider.calls != 1 {
		t.Errorf("expected provider called exactly once, got %d", provider.calls)
	}
}

func TestResolveMergeRequestDiscussion_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeGitLabMergeRequestProvider{discErr: errors.New("not found")}
	uc := NewResolveMergeRequestDiscussion(&fakeCredentialResolver{token: "tok"}, provider)

	_, err := uc.Execute(context.Background(), ResolveMergeRequestDiscussionParams{
		TenantID: "tenant-1", Repo: "group/project", MergeRequestIID: 42, DiscussionID: "disc-1",
	})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestResolveMergeRequestDiscussion_RequiresTenantRepoDiscussion(t *testing.T) {
	uc := NewResolveMergeRequestDiscussion(&fakeCredentialResolver{}, &fakeGitLabMergeRequestProvider{})
	cases := []ResolveMergeRequestDiscussionParams{
		{Repo: "group/project", DiscussionID: "d1"},
		{TenantID: "t1", DiscussionID: "d1"},
		{TenantID: "t1", Repo: "group/project"},
	}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
