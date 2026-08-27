package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
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

	mergedPR        domain.PullRequest
	merged          bool
	mergeSHA        string
	mergeErr        error
	reviewersPR     domain.PullRequest
	reviewersErr    error
	autoMergePR     domain.PullRequest
	autoMergeErr    error
	updatedIssue    domain.Issue
	updateIssueErr  error
	branchPR        domain.PullRequest
	branchFound     bool
	branchErr       error
	slugOwner       string
	slugName        string
	slugErr         error
	branchExists    bool
	branchExistsErr error

	repoFileContent     string
	repoFileFound       bool
	repoFileErr         error
	getRepoFileCalls    int
	lastGetRepoFilePath string

	lastCred Credential
	lastRepo string
	calls    int
}

func (f *fakeProvider) MergePullRequest(ctx context.Context, cred Credential, repo string, number int32, input MergePullRequestInput) (domain.PullRequest, bool, string, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.mergeErr != nil {
		return domain.PullRequest{}, false, "", f.mergeErr
	}
	return f.mergedPR, f.merged, f.mergeSHA, nil
}

func (f *fakeProvider) RequestPullRequestReviewers(ctx context.Context, cred Credential, repo string, number int32, reviewerLogins, teamSlugs []string) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.reviewersErr != nil {
		return domain.PullRequest{}, f.reviewersErr
	}
	return f.reviewersPR, nil
}

func (f *fakeProvider) RemovePullRequestReviewers(ctx context.Context, cred Credential, repo string, number int32, reviewerLogins []string) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.reviewersErr != nil {
		return domain.PullRequest{}, f.reviewersErr
	}
	return f.reviewersPR, nil
}

func (f *fakeProvider) SetPullRequestAutoMerge(ctx context.Context, cred Credential, repo string, number int32, enabled bool, mergeMethod string) (domain.PullRequest, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.autoMergeErr != nil {
		return domain.PullRequest{}, f.autoMergeErr
	}
	return f.autoMergePR, nil
}

func (f *fakeProvider) UpdateIssue(ctx context.Context, cred Credential, repo string, number int32, patch IssuePatch) (domain.Issue, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.updateIssueErr != nil {
		return domain.Issue{}, f.updateIssueErr
	}
	return f.updatedIssue, nil
}

func (f *fakeProvider) GetPullRequestForBranch(ctx context.Context, cred Credential, repo, headBranch string) (domain.PullRequest, bool, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.branchErr != nil {
		return domain.PullRequest{}, false, f.branchErr
	}
	return f.branchPR, f.branchFound, nil
}

func (f *fakeProvider) ResolveRepoSlug(ctx context.Context, cred Credential, candidate string) (string, string, error) {
	f.lastCred = cred
	f.calls++
	if f.slugErr != nil {
		return "", "", f.slugErr
	}
	return f.slugOwner, f.slugName, nil
}

func (f *fakeProvider) BranchExists(ctx context.Context, cred Credential, repo, branch string) (bool, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.branchExistsErr != nil {
		return false, f.branchExistsErr
	}
	return f.branchExists, nil
}

func (f *fakeProvider) GetRepoFileContent(ctx context.Context, cred Credential, repo, path, ref string) (string, bool, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.lastGetRepoFilePath = path
	f.getRepoFileCalls++
	f.calls++
	if f.repoFileErr != nil {
		return "", false, f.repoFileErr
	}
	return f.repoFileContent, f.repoFileFound, nil
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

// fakeCredentialResolverConnectedFlag is a small fake specific to
// CheckHostedReviewEligibility's tests — GetAuthStatus.Execute calls
// CredentialResolver.Resolve and treats a nil error as "connected" (see
// get_auth_status.go), so this fake returns nil when connected: true and a
// sentinel error otherwise.
type fakeCredentialResolverConnectedFlag struct {
	connected bool
}

func (f *fakeCredentialResolverConnectedFlag) Resolve(ctx context.Context, tenantID string, provider domain.ScmProvider) (Credential, error) {
	if f.connected {
		return Credential{Token: "tok"}, nil
	}
	return Credential{}, errors.New("not connected")
}

// fakeGitHubProjects is an in-memory GitHubProjectsProvider — mirrors
// fakeProvider's recording-fields pattern.
type fakeGitHubProjects struct {
	projects    []Project
	projectsErr error

	project    Project
	projectErr error

	views    []ProjectView
	viewsErr error

	items         []ProjectItem
	nextPageToken string
	itemsErr      error

	item    ProjectItem
	itemErr error

	details    WorkItemDetails
	detailsErr error

	issueTypes    []IssueType
	issueTypesErr error

	users    []AssignableUser
	usersErr error

	labels    []Label
	labelsErr error

	comment    ProjectComment
	commentErr error

	deleteErr error

	lastItemSlug string
	calls        int
}

func (f *fakeGitHubProjects) ListAccessibleProjects(ctx context.Context, cred Credential) ([]Project, error) {
	f.calls++
	if f.projectsErr != nil {
		return nil, f.projectsErr
	}
	return f.projects, nil
}

func (f *fakeGitHubProjects) ResolveProjectRef(ctx context.Context, cred Credential, owner string, number int32) (Project, error) {
	f.calls++
	if f.projectErr != nil {
		return Project{}, f.projectErr
	}
	return f.project, nil
}

func (f *fakeGitHubProjects) ListProjectViews(ctx context.Context, cred Credential, projectSlug string) ([]ProjectView, error) {
	f.calls++
	if f.viewsErr != nil {
		return nil, f.viewsErr
	}
	return f.views, nil
}

func (f *fakeGitHubProjects) ViewProjectTable(ctx context.Context, cred Credential, projectSlug, viewID, pageToken string, pageSize int32) ([]ProjectItem, string, error) {
	f.calls++
	if f.itemsErr != nil {
		return nil, "", f.itemsErr
	}
	return f.items, f.nextPageToken, nil
}

func (f *fakeGitHubProjects) UpdateProjectItemField(ctx context.Context, cred Credential, projectSlug, itemID string, field ProjectFieldValue) (ProjectItem, error) {
	f.calls++
	if f.itemErr != nil {
		return ProjectItem{}, f.itemErr
	}
	return f.item, nil
}

func (f *fakeGitHubProjects) ClearProjectItemField(ctx context.Context, cred Credential, projectSlug, itemID, fieldID string) (ProjectItem, error) {
	f.calls++
	if f.itemErr != nil {
		return ProjectItem{}, f.itemErr
	}
	return f.item, nil
}

func (f *fakeGitHubProjects) GetWorkItemDetailsBySlug(ctx context.Context, cred Credential, itemSlug string) (WorkItemDetails, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.detailsErr != nil {
		return WorkItemDetails{}, f.detailsErr
	}
	return f.details, nil
}

func (f *fakeGitHubProjects) UpdateIssueBySlug(ctx context.Context, cred Credential, itemSlug string, patch WorkItemPatch) (WorkItemDetails, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.detailsErr != nil {
		return WorkItemDetails{}, f.detailsErr
	}
	return f.details, nil
}

func (f *fakeGitHubProjects) UpdatePullRequestBySlug(ctx context.Context, cred Credential, itemSlug string, patch WorkItemPatch) (WorkItemDetails, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.detailsErr != nil {
		return WorkItemDetails{}, f.detailsErr
	}
	return f.details, nil
}

func (f *fakeGitHubProjects) UpdateIssueTypeBySlug(ctx context.Context, cred Credential, itemSlug, issueType string) (WorkItemDetails, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.detailsErr != nil {
		return WorkItemDetails{}, f.detailsErr
	}
	return f.details, nil
}

func (f *fakeGitHubProjects) ListIssueTypesBySlug(ctx context.Context, cred Credential, itemSlug string) ([]IssueType, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.issueTypesErr != nil {
		return nil, f.issueTypesErr
	}
	return f.issueTypes, nil
}

func (f *fakeGitHubProjects) ListAssignableUsersBySlug(ctx context.Context, cred Credential, itemSlug string) ([]AssignableUser, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.usersErr != nil {
		return nil, f.usersErr
	}
	return f.users, nil
}

func (f *fakeGitHubProjects) ListLabelsBySlug(ctx context.Context, cred Credential, itemSlug string) ([]Label, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.labelsErr != nil {
		return nil, f.labelsErr
	}
	return f.labels, nil
}

func (f *fakeGitHubProjects) AddIssueCommentBySlug(ctx context.Context, cred Credential, itemSlug, body string) (ProjectComment, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.commentErr != nil {
		return ProjectComment{}, f.commentErr
	}
	return f.comment, nil
}

func (f *fakeGitHubProjects) UpdateIssueCommentBySlug(ctx context.Context, cred Credential, itemSlug, commentID, body string) (ProjectComment, error) {
	f.lastItemSlug = itemSlug
	f.calls++
	if f.commentErr != nil {
		return ProjectComment{}, f.commentErr
	}
	return f.comment, nil
}

func (f *fakeGitHubProjects) DeleteIssueCommentBySlug(ctx context.Context, cred Credential, itemSlug, commentID string) error {
	f.lastItemSlug = itemSlug
	f.calls++
	return f.deleteErr
}

// fakeGitLabMergeRequestProvider is an in-memory GitLabMergeRequestProvider
// — mirrors fakeProvider's recording-fields pattern.
type fakeGitLabMergeRequestProvider struct {
	mrs    []domain.MergeRequest
	mrsErr error

	disc    domain.MergeRequestDiscussion
	discErr error

	details    domain.WorkItemDetailsGitLab
	detailsErr error

	lastCred   Credential
	lastRepo   string
	lastFilter MRFilter
	calls      int
}

func (f *fakeGitLabMergeRequestProvider) ListMergeRequests(ctx context.Context, cred Credential, repo string, filter MRFilter) ([]domain.MergeRequest, error) {
	f.lastCred, f.lastRepo, f.lastFilter = cred, repo, filter
	f.calls++
	if f.mrsErr != nil {
		return nil, f.mrsErr
	}
	return f.mrs, nil
}

func (f *fakeGitLabMergeRequestProvider) ResolveDiscussion(ctx context.Context, cred Credential, repo string, mrIID int32, discussionID string, resolved bool) (domain.MergeRequestDiscussion, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.discErr != nil {
		return domain.MergeRequestDiscussion{}, f.discErr
	}
	return f.disc, nil
}

func (f *fakeGitLabMergeRequestProvider) GetWorkItemDetails(ctx context.Context, cred Credential, repo string, iid int32, itemType string) (domain.WorkItemDetailsGitLab, error) {
	f.lastCred, f.lastRepo = cred, repo
	f.calls++
	if f.detailsErr != nil {
		return domain.WorkItemDetailsGitLab{}, f.detailsErr
	}
	return f.details, nil
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
	gitlab := &fakeProvider{pr: want, branchExists: true}
	github := &fakeProvider{}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{
		domain.ScmProviderGitHub: github,
		domain.ScmProviderGitLab: gitlab,
	}}
	creds := &fakeCredentialResolver{token: "tok-456"}

	uc := NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry))
	result, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "tenant-1", Provider: domain.ScmProviderGitLab, Repo: "group/project",
		Title: "feature", HeadBranch: "feature", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PullRequest.ID != "9" {
		t.Fatalf("expected the gitlab fake's pull request back, got %+v", result.PullRequest)
	}
	// gitlab.calls counts both BranchExists and CreatePullRequest.
	if gitlab.calls != 2 || github.calls != 0 {
		t.Fatalf("expected exactly the gitlab adapter to be called, gitlab.calls=%d github.calls=%d", gitlab.calls, github.calls)
	}
	if gitlab.lastCred.Token != "tok-456" {
		t.Errorf("expected resolved credential to reach the adapter, got %q", gitlab.lastCred.Token)
	}
}

func TestCreatePullRequest_RequiresTitle(t *testing.T) {
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: &fakeProvider{}}}
	creds := &fakeCredentialResolver{}
	uc := NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry))

	_, err := uc.Execute(context.Background(), CreatePullRequestParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b"})
	if err == nil {
		t.Error("expected error when title is missing")
	}
}

// TestCreatePullRequest_BranchNotPushed_ReturnsFailedPreconditionAndSkipsCreate
// is a regression guard for BR-CR-17: a branch that hasn't been pushed
// must reject before ever calling the provider's CreatePullRequest.
func TestCreatePullRequest_BranchNotPushed_ReturnsFailedPreconditionAndSkipsCreate(t *testing.T) {
	provider := &fakeProvider{branchExists: false}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	uc := NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry))

	_, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b",
		Title: "t", HeadBranch: "h", BaseBranch: "main",
	})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "SCM_BRANCH_NOT_PUSHED" {
		t.Fatalf("expected SCM_BRANCH_NOT_PUSHED, got %v", err)
	}
	// Only BranchExists should have been called — CreatePullRequest never
	// reached.
	if provider.calls != 1 {
		t.Errorf("expected exactly 1 call (BranchExists only), got %d", provider.calls)
	}
}

// TestCreatePullRequest_DraftUnsupported_MapsToTypedPrecondition covers
// BR-CR-20: a provider adapter returning an error wrapping
// domain.ErrCapabilityUnsupported for a draft request maps to
// SCM_DRAFT_UNSUPPORTED, not a generic internal error.
func TestCreatePullRequest_DraftUnsupported_MapsToTypedPrecondition(t *testing.T) {
	provider := &fakeProvider{branchExists: true, prErr: fmt.Errorf("bitbucket: draft not supported: %w", domain.ErrCapabilityUnsupported)}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderBitbucket: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	uc := NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry))

	_, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderBitbucket, Repo: "a/b",
		Title: "t", HeadBranch: "h", BaseBranch: "main", Draft: true,
	})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "SCM_DRAFT_UNSUPPORTED" {
		t.Fatalf("expected SCM_DRAFT_UNSUPPORTED, got %v", err)
	}
}

// TestCreatePullRequest_LinkedIssueUpdateFailure_DoesNotFailPRCreation
// covers BR-CR-19's failure mode: a failed linked-issue update must not
// roll back or mask a successful PR creation.
func TestCreatePullRequest_LinkedIssueUpdateFailure_DoesNotFailPRCreation(t *testing.T) {
	created, err := domain.NewPullRequest("9", domain.ScmProviderGitHub, "a/b", "feature", "open", "https://example.invalid/pull/9", "h", "main")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	provider := &fakeProvider{branchExists: true, pr: created, updateIssueErr: errors.New("issue update failed")}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	uc := NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry))

	result, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b",
		Title: "t", HeadBranch: "h", BaseBranch: "main", LinkedIssueNumber: 42,
	})
	if err != nil {
		t.Fatalf("expected PR creation to succeed despite the linked-issue update failing, got %v", err)
	}
	if result.PullRequest.ID != "9" {
		t.Errorf("expected the created PR to be returned, got %+v", result.PullRequest)
	}
	if result.LinkedIssueUpdateError == "" {
		t.Error("expected a non-empty LinkedIssueUpdateError")
	}
}

// TestCreatePullRequest_NoLinkedIssue_DoesNotCallUpdateIssue is a
// regression guard: LinkedIssueNumber unset (0) must never call UpdateIssue.
func TestCreatePullRequest_NoLinkedIssue_DoesNotCallUpdateIssue(t *testing.T) {
	created, err := domain.NewPullRequest("9", domain.ScmProviderGitHub, "a/b", "feature", "open", "https://example.invalid/pull/9", "h", "main")
	if err != nil {
		t.Fatalf("unexpected error building fixture: %v", err)
	}
	provider := &fakeProvider{branchExists: true, pr: created}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	uc := NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry))

	result, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "a/b",
		Title: "t", HeadBranch: "h", BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LinkedIssueUpdateError != "" {
		t.Errorf("expected no linked-issue-update error when LinkedIssueNumber is unset, got %q", result.LinkedIssueUpdateError)
	}
	// provider.calls: BranchExists + CreatePullRequest only — UpdateIssue
	// would be an additional call on the same fake since registry resolves
	// to the same provider for both ScmProvider and UpdateIssue's own
	// resolution.
	if provider.calls != 2 {
		t.Errorf("expected exactly 2 calls (BranchExists + CreatePullRequest), got %d — UpdateIssue must not have been called", provider.calls)
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

// fakeRateLimitCache is an in-memory RateLimitCache — stands in for the
// real postgres-backed one in internal/adapter/postgres.
type fakeRateLimitCache struct {
	stored     map[string]domain.RateLimitStatus
	getCalls   int
	setCalls   int
	forceStale bool // when true, Get always reports a miss regardless of stored
}

func (f *fakeRateLimitCache) Get(_ context.Context, tenantID string, provider domain.ScmProvider, _ time.Duration) (domain.RateLimitStatus, bool, error) {
	f.getCalls++
	if f.forceStale {
		return domain.RateLimitStatus{}, false, nil
	}
	status, ok := f.stored[tenantID+"/"+string(provider)]
	return status, ok, nil
}

func (f *fakeRateLimitCache) Set(_ context.Context, tenantID string, provider domain.ScmProvider, status domain.RateLimitStatus) error {
	f.setCalls++
	if f.stored == nil {
		f.stored = map[string]domain.RateLimitStatus{}
	}
	f.stored[tenantID+"/"+string(provider)] = status
	return nil
}

func TestGetRateLimitStatus_DispatchesToResolvedProviderWithCredential(t *testing.T) {
	want := domain.RateLimitStatus{Provider: domain.ScmProviderGitHub, Remaining: 340, Limit: 5000, ResetAt: time.Now().Add(12 * time.Minute)}
	github := &fakeProvider{rateLimit: want}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	creds := &fakeCredentialResolver{token: "tok-abc"}

	uc := NewGetRateLimitStatus(creds, registry, nil)
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
	uc := NewGetRateLimitStatus(&fakeCredentialResolver{token: "tok"}, registry, nil)

	_, err := uc.Execute(context.Background(), GetRateLimitStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err == nil {
		t.Fatal("expected adapter failure to propagate")
	}
}

func TestGetRateLimitStatus_ReturnsCachedValueWithoutCallingProvider(t *testing.T) {
	github := &fakeProvider{rateLimit: domain.RateLimitStatus{Remaining: 1, Limit: 1}} // would be wrong if hit — proves the cache short-circuits
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	cached := domain.RateLimitStatus{Provider: domain.ScmProviderGitHub, Remaining: 340, Limit: 5000, ResetAt: time.Now().Add(12 * time.Minute)}
	cache := &fakeRateLimitCache{stored: map[string]domain.RateLimitStatus{"tenant-1/github": cached}}

	uc := NewGetRateLimitStatus(&fakeCredentialResolver{token: "tok"}, registry, cache)
	got, err := uc.Execute(context.Background(), GetRateLimitStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Remaining != 340 || github.calls != 0 {
		t.Fatalf("expected a cache hit to skip the provider call entirely, got %+v (provider calls=%d)", got, github.calls)
	}
}

func TestGetRateLimitStatus_CacheMissCallsProviderAndPopulatesCache(t *testing.T) {
	live := domain.RateLimitStatus{Provider: domain.ScmProviderGitHub, Remaining: 100, Limit: 5000, ResetAt: time.Now().Add(time.Hour)}
	github := &fakeProvider{rateLimit: live}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: github}}
	cache := &fakeRateLimitCache{forceStale: true}

	uc := NewGetRateLimitStatus(&fakeCredentialResolver{token: "tok"}, registry, cache)
	got, err := uc.Execute(context.Background(), GetRateLimitStatusInput{TenantID: "tenant-1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Remaining != 100 || github.calls != 1 {
		t.Fatalf("expected a live provider call on a cache miss, got %+v (provider calls=%d)", got, github.calls)
	}
	if cache.setCalls != 1 {
		t.Errorf("expected the live result to be written back to the cache, setCalls=%d", cache.setCalls)
	}
}
