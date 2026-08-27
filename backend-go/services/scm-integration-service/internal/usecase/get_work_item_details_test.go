package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestGetWorkItemDetails_Success(t *testing.T) {
	details := domain.WorkItemDetailsGitLab{ID: "1", IID: 42, ItemType: "merge_request", Title: "Fix bug"}
	provider := &fakeGitLabMergeRequestProvider{details: details}
	uc := NewGetWorkItemDetails(&fakeCredentialResolver{token: "tok"}, provider)

	got, err := uc.Execute(context.Background(), GetWorkItemDetailsParams{TenantID: "tenant-1", Repo: "group/project", IID: 42, ItemType: "merge_request"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IID != 42 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestGetWorkItemDetails_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeGitLabMergeRequestProvider{detailsErr: errors.New("not found")}
	uc := NewGetWorkItemDetails(&fakeCredentialResolver{token: "tok"}, provider)

	_, err := uc.Execute(context.Background(), GetWorkItemDetailsParams{TenantID: "tenant-1", Repo: "group/project", IID: 42})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestGetWorkItemDetails_RequiresTenantAndRepo(t *testing.T) {
	uc := NewGetWorkItemDetails(&fakeCredentialResolver{}, &fakeGitLabMergeRequestProvider{})
	cases := []GetWorkItemDetailsParams{{Repo: "group/project"}, {TenantID: "t1"}}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
