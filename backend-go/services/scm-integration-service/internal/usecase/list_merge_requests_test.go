package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestListMergeRequests_MapsFilterAndReturnsResults(t *testing.T) {
	mrs := []domain.MergeRequest{{ID: "1", Repo: "group/project", IID: 42, Title: "Fix bug", State: "opened"}}
	provider := &fakeGitLabMergeRequestProvider{mrs: mrs}
	uc := NewListMergeRequests(&fakeCredentialResolver{token: "tok"}, provider)

	got, err := uc.Execute(context.Background(), ListMergeRequestsParams{
		TenantID: "tenant-1", Repo: "group/project", State: "opened", SourceBranch: "feature-x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].IID != 42 {
		t.Fatalf("expected one MR with iid 42, got %+v", got)
	}
	if provider.lastFilter.State != "opened" || provider.lastFilter.SourceBranch != "feature-x" {
		t.Errorf("expected filter to be passed through, got %+v", provider.lastFilter)
	}
}

func TestListMergeRequests_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeGitLabMergeRequestProvider{mrsErr: errors.New("gitlab unavailable")}
	uc := NewListMergeRequests(&fakeCredentialResolver{token: "tok"}, provider)

	_, err := uc.Execute(context.Background(), ListMergeRequestsParams{TenantID: "tenant-1", Repo: "group/project"})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestListMergeRequests_RequiresTenantAndRepo(t *testing.T) {
	uc := NewListMergeRequests(&fakeCredentialResolver{}, &fakeGitLabMergeRequestProvider{})
	cases := []ListMergeRequestsParams{{Repo: "group/project"}, {TenantID: "tenant-1"}}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
