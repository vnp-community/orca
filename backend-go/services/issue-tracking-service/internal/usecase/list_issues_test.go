package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// fakeIssueTrackerProvider is an in-memory IssueTrackerProvider.
type fakeIssueTrackerProvider struct {
	issues      []domain.Issue
	listErr     error
	createErr   error
	createInput struct {
		projectKey, title, description string
	}
	createReturn domain.Issue
}

func (f *fakeIssueTrackerProvider) ListIssues(ctx context.Context, cred Credential, projectKey string) ([]domain.Issue, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.issues, nil
}

func (f *fakeIssueTrackerProvider) CreateIssue(ctx context.Context, cred Credential, projectKey, title, description string) (domain.Issue, error) {
	if f.createErr != nil {
		return domain.Issue{}, f.createErr
	}
	f.createInput.projectKey = projectKey
	f.createInput.title = title
	f.createInput.description = description
	return f.createReturn, nil
}

// fakeProviderRegistry resolves a single fixed provider, or errors if asked
// to resolve anything else — enough for these unit tests.
type fakeProviderRegistry struct {
	provider    domain.Provider
	impl        IssueTrackerProvider
	resolveErr  error
	resolveArgs []domain.Provider
}

func (f *fakeProviderRegistry) Resolve(provider domain.Provider) (IssueTrackerProvider, error) {
	f.resolveArgs = append(f.resolveArgs, provider)
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	if provider != f.provider {
		return nil, errors.New("fakeProviderRegistry: unexpected provider")
	}
	return f.impl, nil
}

// fakeCredentialResolver returns a fixed credential, or errors.
type fakeCredentialResolver struct {
	cred       Credential
	resolveErr error
}

func (f *fakeCredentialResolver) Resolve(ctx context.Context, tenantID string, provider domain.Provider) (Credential, error) {
	if f.resolveErr != nil {
		return Credential{}, f.resolveErr
	}
	return f.cred, nil
}

func TestListIssues_RequiresTenantContext(t *testing.T) {
	uc := NewListIssues(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	_, err := uc.Execute(context.Background(), ListIssuesInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestListIssues_RejectsInvalidProvider(t *testing.T) {
	uc := NewListIssues(&fakeProviderRegistry{}, &fakeCredentialResolver{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.Provider("bogus")})
	if err == nil {
		t.Fatal("expected an error for an invalid provider")
	}
}

func TestListIssues_ReturnsProviderIssues(t *testing.T) {
	want := []domain.Issue{{ID: "PROJ-1", Title: "Fix bug", State: "Todo", URL: "https://example.atlassian.net/browse/PROJ-1"}}
	provider := &fakeIssueTrackerProvider{issues: want}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{cred: Credential{BaseURL: "https://example.atlassian.net", Email: "a@b.com", Token: "tok"}}

	uc := NewListIssues(registry, credentials)
	ctx := withTenant(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.ProviderJira, ProjectKey: "PROJ"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "PROJ-1" {
		t.Errorf("unexpected issues: %+v", got)
	}
}

func TestListIssues_ProviderErrorPropagates(t *testing.T) {
	provider := &fakeIssueTrackerProvider{listErr: errors.New("jira unavailable")}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{}

	uc := NewListIssues(registry, credentials)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected error to propagate from provider failure")
	}
}

func TestListIssues_CredentialResolutionFailurePropagates(t *testing.T) {
	provider := &fakeIssueTrackerProvider{}
	registry := &fakeProviderRegistry{provider: domain.ProviderJira, impl: provider}
	credentials := &fakeCredentialResolver{resolveErr: errors.New("no credential on file")}

	uc := NewListIssues(registry, credentials)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, ListIssuesInput{Provider: domain.ProviderJira})
	if err == nil {
		t.Fatal("expected error to propagate from credential resolution failure")
	}
}
