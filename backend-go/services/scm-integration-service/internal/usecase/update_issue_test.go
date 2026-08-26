package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestUpdateIssue_Success(t *testing.T) {
	issue, _ := domain.NewIssue("1", domain.ScmProviderGitHub, "o/r", "t", "open", "url")
	provider := &fakeProvider{updatedIssue: issue}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewUpdateIssue(&fakeCredentialResolver{token: "tok"}, registry)

	title := "new title"
	got, err := uc.Execute(context.Background(), UpdateIssueParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1, Patch: IssuePatch{Title: &title},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "1" || provider.calls != 1 {
		t.Fatalf("unexpected result: %+v calls=%d", got, provider.calls)
	}
}

func TestUpdateIssue_PropagatesProviderFailure(t *testing.T) {
	provider := &fakeProvider{updateIssueErr: errors.New("not found")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	uc := NewUpdateIssue(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), UpdateIssueParams{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1})
	if err == nil {
		t.Fatal("expected an error when the provider call fails")
	}
}

func TestUpdateIssue_RequiresTenantAndRepo(t *testing.T) {
	uc := NewUpdateIssue(&fakeCredentialResolver{}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}})
	cases := []UpdateIssueParams{{Repo: "o/r"}, {TenantID: "t1"}}
	for _, in := range cases {
		if _, err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("expected a validation error for %+v", in)
		}
	}
}
