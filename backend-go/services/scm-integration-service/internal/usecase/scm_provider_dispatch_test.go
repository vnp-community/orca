package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// fakeProvider is an in-memory ScmProvider — the "test against fakes, not a
// real HTTP client" pattern from usage-service's usecase tests. It records
// what it was called with so tests can assert the usecase layer dispatched
// to the *right* provider with the *right* credential — the valuable,
// real-logic part of this layer, even though the actual HTTP calls
// (internal/adapter/{github,gitlab,...}) are stubbed or real elsewhere.
type fakeProvider struct {
	issues    []domain.Issue
	issuesErr error

	pr    domain.PullRequest
	prErr error

	prs    []domain.PullRequest
	prsErr error

	rateLimit    domain.RateLimitStatus
	rateLimitErr error

	lastCred Credential
	lastRepo string
	calls    int
}

func (f *fakeProvider) ListIssues(ctx context.Context, cred Credential, repo string, filter IssueFilter) ([]domain.Issue, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.issuesErr != nil {
		return nil, f.issuesErr
	}
	return f.issues, nil
}

func (f *fakeProvider) CreatePullRequest(ctx context.Context, cred Credential, repo string, input CreatePullRequestInput) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.prErr != nil {
		return domain.PullRequest{}, f.prErr
	}
	return f.pr, nil
}

func (f *fakeProvider) ListPullRequests(ctx context.Context, cred Credential, repo string) ([]domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.prsErr != nil {
		return nil, f.prsErr
	}
	return f.prs, nil
}

func (f *fakeProvider) GetRateLimitStatus(ctx context.Context, cred Credential) (domain.RateLimitStatus, error) {
	f.lastCred = cred
	f.calls++
	if f.rateLimitErr != nil {
		return domain.RateLimitStatus{}, f.rateLimitErr
	}
	return f.rateLimit, nil
}

// fakeRegistry is an in-memory ProviderRegistry.
type fakeRegistry struct {
	providers map[domain.ScmProvider]ScmProvider
}

func (r *fakeRegistry) Resolve(provider domain.ScmProvider) (ScmProvider, error) {
	p, ok := r.providers[provider]
	if !ok {
		return nil, errors.New("fakeRegistry: no adapter registered for provider")
	}
	return p, nil
}

// fakeCredentialResolver is an in-memory CredentialResolver — stands in for
// the real credential-broker-service stub in internal/adapter/credentialbroker.
type fakeCredentialResolver struct {
	token        string
	err          error
	lastTenant   string
	lastProvider domain.ScmProvider
}

func (f *fakeCredentialResolver) Resolve(ctx context.Context, tenantID string, provider domain.ScmProvider) (Credential, error) {
	f.lastTenant, f.lastProvider = tenantID, provider
	if f.err != nil {
		return Credential{}, f.err
	}
	return Credential{Token: f.token}, nil
}

func TestListIssues_DispatchesToResolvedProviderWithCredential(t *testing.T) {
	githubIssue, err := domain.NewIssue("1", domain.ScmProviderGitHub, "octocat/hello-world", "bug", "open", "https://example.invalid/1")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	github := &fakeProvider{issues: []domain.Issue{githubIssue}}
	gitlab := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{
		domain.ScmProviderGitHub: github,
		domain.ScmProviderGitLab: gitlab,
	}}
	creds := &fakeCredentialResolver{token: "tok-123"}

	uc := NewListIssues(creds, registry)
	got, err := uc.Execute(context.Background(), ListIssuesInput{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "octocat/hello-world",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("expected the github fake's issue back, got %+v", got)
	}
	if github.calls != 1 || gitlab.calls != 0 {
		t.Fatalf("expected exactly the github adapter to be called, github.calls=%d gitlab.calls=%d", github.calls, gitlab.calls)
	}
	if github.lastCred.Token != "tok-123" {
		t.Errorf("expected resolved credential to reach the adapter, got %q", github.lastCred.Token)
	}
	if creds.lastTenant != "tenant-1" || creds.lastProvider != domain.ScmProviderGitHub {
		t.Errorf("expected credential resolver to be called with tenant/provider, got tenant=%q provider=%q", creds.lastTenant, creds.lastProvider)
	}
}

func TestListIssues_RequiresTenantAndRepo(t *testing.T) {
	uc := NewListIssues(&fakeCredentialResolver{}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}})

	if _, err := uc.Execute(context.Background(), ListIssuesInput{Repo: "a/b"}); err == nil {
		t.Error("expected error when tenant_id is missing")
	}
	if _, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1"}); err == nil {
		t.Error("expected error when repo is missing")
	}
}

func TestListIssues_PropagatesCredentialResolutionFailure(t *testing.T) {
	creds := &fakeCredentialResolver{err: errors.New("broker unavailable")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: &fakeProvider{}}}
	uc := NewListIssues(creds, registry)

	_, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b"})
	if err == nil {
		t.Fatal("expected credential resolution failure to propagate")
	}
}

func TestListIssues_UnregisteredProviderFails(t *testing.T) {
	uc := NewListIssues(&fakeCredentialResolver{token: "tok"}, &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{}})

	_, err := uc.Execute(context.Background(), ListIssuesInput{TenantID: "t1", Provider: domain.ScmProviderBitbucket, Repo: "a/b"})
	if err == nil {
		t.Fatal("expected error when no adapter is registered for the provider")
	}
}

func TestCreatePullRequest_DispatchesToResolvedProviderWithCredential(t *testing.T) {
	want, err := domain.NewPullRequest("9", domain.ScmProviderGitLab, "group/project", "feature", "opened", "https://example.invalid/mr/9", "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	gitlab := &fakeProvider{pr: want}
	github := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{
		domain.ScmProviderGitHub: github,
		domain.ScmProviderGitLab: gitlab,
	}}
	creds := &fakeCredentialResolver{token: "tok-456"}

	uc := NewCreatePullRequest(creds, registry)
	got, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitLab, Repo: "group/project",
		Title: "feature", HeadBranch: "feature", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "9" {
		t.Fatalf("expected the gitlab fake's pull request back, got %+v", got)
	}
	if gitlab.calls != 1 || github.calls != 0 {
		t.Fatalf("expected exactly the gitlab adapter to be called, gitlab.calls=%d github.calls=%d", gitlab.calls, github.calls)
	}
	if gitlab.lastCred.Token != "tok-456" {
		t.Errorf("expected resolved credential to reach the adapter, got %q", gitlab.lastCred.Token)
	}
}

func TestCreatePullRequest_RequiresTitle(t *testing.T) {
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: &fakeProvider{}}}
	uc := NewCreatePullRequest(&fakeCredentialResolver{}, registry)

	_, err := uc.Execute(context.Background(), CreatePullRequestParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b"})
	if err == nil {
		t.Error("expected error when title is missing")
	}
}

func TestListPullRequests_DispatchesToResolvedProviderWithCredential(t *testing.T) {
	pr, err := domain.NewPullRequest("2", domain.ScmProviderGitHub, "octocat/hello-world", "fix", "open", "https://example.invalid/pull/2", "fix", "main")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	github := &fakeProvider{prs: []domain.PullRequest{pr}}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	creds := &fakeCredentialResolver{token: "tok-789"}

	uc := NewListPullRequests(creds, registry)
	got, err := uc.Execute(context.Background(), ListPullRequestsInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub, Repo: "octocat/hello-world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("expected the github fake's pull request back, got %+v", got)
	}
	if github.lastCred.Token != "tok-789" {
		t.Errorf("expected resolved credential to reach the adapter, got %q", github.lastCred.Token)
	}
}

func TestGetRateLimitStatus_DispatchesToResolvedProviderWithCredential(t *testing.T) {
	want := domain.RateLimitStatus{Provider: domain.ScmProviderGitHub, Remaining: 340, Limit: 5000, ResetAt: time.Now().Add(12 * time.Minute)}
	github := &fakeProvider{rateLimit: want}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	creds := &fakeCredentialResolver{token: "tok-abc"}

	uc := NewGetRateLimitStatus(creds, registry)
	got, err := uc.Execute(context.Background(), GetRateLimitStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Remaining != 340 || got.Limit != 5000 {
		t.Fatalf("expected the github fake's rate limit status back, got %+v", got)
	}
	if github.lastCred.Token != "tok-abc" {
		t.Errorf("expected resolved credential to reach the adapter, got %q", github.lastCred.Token)
	}
}

func TestGetRateLimitStatus_PropagatesAdapterFailure(t *testing.T) {
	github := &fakeProvider{rateLimitErr: errors.New("github: unexpected status 503")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	uc := NewGetRateLimitStatus(&fakeCredentialResolver{token: "tok"}, registry)

	_, err := uc.Execute(context.Background(), GetRateLimitStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err == nil {
		t.Fatal("expected adapter failure to propagate")
	}
}
