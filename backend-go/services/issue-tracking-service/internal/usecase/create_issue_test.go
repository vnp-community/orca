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

func TestCreateIssue_RequiresTitle(t *testing.T) {
	uc := NewCreateIssue(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	if _, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ"}); err == nil {
		t.Error("expected an error when title is empty")
	}
}

func TestCreateIssue_CreatesViaProvider(t *testing.T) {
	want := domain.Issue{ID: "PROJ-2", Title: "New issue", State: "", URL: "https://example.atlassian.net/browse/PROJ-2"}
	provider := &fakeIssueTrackerProvider{createReturn: want}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{cred: Credential{BaseURL: "https://example.atlassian.net", Email: "a@b.com", Token: "tok"}}

	uc := NewCreateIssue(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	got, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ", Title: "New issue", Description: "desc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID || got.Title != want.Title || got.State != want.State || got.URL != want.URL {
		t.Errorf("expected %+v, got %+v", want, got)
	}
	if provider.createInput.ProjectKey != "PROJ" || provider.createInput.Title != "New issue" || provider.createInput.Description != "desc" {
		t.Errorf("unexpected provider call: %+v", provider.createInput)
	}
}

func TestCreateIssue_ProviderErrorPropagates(t *testing.T) {
	provider := &fakeIssueTrackerProvider{createErr: errors.New("jira rejected request")}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{}

	uc := NewCreateIssue(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, ProjectKey: "PROJ", Title: "t"})
	if err == nil {
		t.Fatal("expected error to propagate from provider failure")
	}
}

func TestCreateIssue_CredentialNotFound_ShortCircuitsBeforeProviderCall(t *testing.T) {
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: &fakeIssueTrackerProvider{}}
	credentials := &fakeCredentialResolver{resolveErr: ErrConnectionNotFound}

	uc := NewCreateIssue(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	_, err := uc.Execute(ctx, CreateIssueInput{Provider: domain.ProviderJira, Title: "t", ProjectKey: "PROJ"})
	if err == nil {
		t.Fatal("expected error")
	}
	if registry.impl.(*fakeIssueTrackerProvider).createIssueCalled {
		t.Fatal("CreateIssue must not reach the provider when credential resolution fails")
	}
}

func TestCreateIssue_CustomFieldsPassThroughVerbatim(t *testing.T) {
	provider := &fakeIssueTrackerProvider{}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{}
	uc := NewCreateIssue(registry, credentials)
	ctx := withUser(withTenant(context.Background(), "tenant-1"), "user-1")

	const custom = `{"customfield_10010":"value"}`
	_, err := uc.Execute(ctx, CreateIssueInput{
		Provider: domain.ProviderJira, Title: "t", ProjectKey: "PROJ", CustomFieldsJSON: custom,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := provider.lastCreateIssueInput.CustomFieldsJSON
	if got != custom {
		t.Errorf("custom fields not passed through verbatim: got %q want %q", got, custom)
	}
}
