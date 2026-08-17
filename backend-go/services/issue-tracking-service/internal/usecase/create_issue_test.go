package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

func TestCreateIssue_RequiresTenantContext(t *testing.T) {
	uc := NewCreateIssue(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	_, err := uc.Execute(context.Background(), CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ", Title: "t"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestCreateIssue_RequiresTitleAndProjectKey(t *testing.T) {
	uc := NewCreateIssue(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	ctx := withTenant(context.Background(), "tenant-1")

	if _, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ"}); err == nil {
		t.Error("expected an error when title is empty")
	}
	if _, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, Title: "t"}); err == nil {
		t.Error("expected an error when project_key is empty")
	}
}

func TestCreateIssue_CreatesViaProvider(t *testing.T) {
	want := domain.Issue{ID: "PROJ-2", Title: "New issue", State: "", URL: "https://example.atlassian.net/browse/PROJ-2"}
	provider := &fakeIssueTrackerProvider{createReturn: want}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{cred: Credential{BaseURL: "https://example.atlassian.net", Email: "a@b.com", Token: "tok"}}

	uc := NewCreateIssue(registry, credentials)
	ctx := withTenant(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ", Title: "New issue", Description: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
	if provider.createInput.projectKey != "PROJ" || provider.createInput.title != "New issue" || provider.createInput.description != "desc" {
		t.Errorf("unexpected provider call: %+v", provider.createInput)
	}
}

func TestCreateIssue_ProviderErrorPropagates(t *testing.T) {
	provider := &fakeIssueTrackerProvider{createErr: errors.New("jira rejected request")}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{}

	uc := NewCreateIssue(registry, credentials)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ", Title: "t"})
	if err == nil {
		t.Fatal("expected error to propagate from provider failure")
	}
}
