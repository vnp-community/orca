package wscompat

import (
	"context"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// fakeScmIntegrationClient is a minimal test double for
// scmintegrationv1.ScmIntegrationServiceClient — same embed-and-override
// shape as this package's other fake gRPC clients (channels_test.go).
type fakeScmIntegrationClient struct {
	scmintegrationv1.ScmIntegrationServiceClient

	mergePullRequestFunc              func(ctx context.Context, in *scmintegrationv1.MergePullRequestRequest) (*scmintegrationv1.MergePullRequestResponse, error)
	requestPullRequestReviewersFunc   func(ctx context.Context, in *scmintegrationv1.RequestPullRequestReviewersRequest) (*scmintegrationv1.PullRequest, error)
	updateIssueFunc                   func(ctx context.Context, in *scmintegrationv1.UpdateIssueRequest) (*scmintegrationv1.Issue, error)
	getPullRequestForBranchFunc       func(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error)
	resolveRepoSlugFunc               func(ctx context.Context, in *scmintegrationv1.ResolveRepoSlugRequest) (*scmintegrationv1.ResolveRepoSlugResponse, error)
	getRateLimitStatusFunc            func(ctx context.Context, in *scmintegrationv1.GetRateLimitStatusRequest) (*scmintegrationv1.GetRateLimitStatusResponse, error)
	listAccessibleProjectsFunc        func(ctx context.Context, in *scmintegrationv1.ListAccessibleProjectsRequest) (*scmintegrationv1.ListAccessibleProjectsResponse, error)
	updateProjectItemFieldFunc        func(ctx context.Context, in *scmintegrationv1.UpdateProjectItemFieldRequest) (*scmintegrationv1.ProjectItem, error)
	deleteIssueCommentBySlugFunc      func(ctx context.Context, in *scmintegrationv1.DeleteIssueCommentBySlugRequest) (*emptypb.Empty, error)
	listMergeRequestsFunc             func(ctx context.Context, in *scmintegrationv1.ListMergeRequestsRequest) (*scmintegrationv1.ListMergeRequestsResponse, error)
	resolveMergeRequestDiscussionFunc func(ctx context.Context, in *scmintegrationv1.ResolveMergeRequestDiscussionRequest) (*scmintegrationv1.MergeRequestDiscussion, error)
	getWorkItemDetailsFunc            func(ctx context.Context, in *scmintegrationv1.GetWorkItemDetailsRequest) (*scmintegrationv1.WorkItemDetailsGitLab, error)
	createPullRequestFunc             func(ctx context.Context, in *scmintegrationv1.CreatePullRequestRequest) (*scmintegrationv1.CreatePullRequestResponse, error)
	checkHostedReviewEligibilityFunc  func(ctx context.Context, in *scmintegrationv1.CheckHostedReviewEligibilityRequest) (*scmintegrationv1.HostedReviewEligibility, error)
}

func (f *fakeScmIntegrationClient) MergePullRequest(ctx context.Context, in *scmintegrationv1.MergePullRequestRequest, _ ...grpc.CallOption) (*scmintegrationv1.MergePullRequestResponse, error) {
	return f.mergePullRequestFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) RequestPullRequestReviewers(ctx context.Context, in *scmintegrationv1.RequestPullRequestReviewersRequest, _ ...grpc.CallOption) (*scmintegrationv1.PullRequest, error) {
	return f.requestPullRequestReviewersFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) UpdateIssue(ctx context.Context, in *scmintegrationv1.UpdateIssueRequest, _ ...grpc.CallOption) (*scmintegrationv1.Issue, error) {
	return f.updateIssueFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) GetPullRequestForBranch(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest, _ ...grpc.CallOption) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
	return f.getPullRequestForBranchFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) ResolveRepoSlug(ctx context.Context, in *scmintegrationv1.ResolveRepoSlugRequest, _ ...grpc.CallOption) (*scmintegrationv1.ResolveRepoSlugResponse, error) {
	return f.resolveRepoSlugFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) GetRateLimitStatus(ctx context.Context, in *scmintegrationv1.GetRateLimitStatusRequest, _ ...grpc.CallOption) (*scmintegrationv1.GetRateLimitStatusResponse, error) {
	return f.getRateLimitStatusFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) ListAccessibleProjects(ctx context.Context, in *scmintegrationv1.ListAccessibleProjectsRequest, _ ...grpc.CallOption) (*scmintegrationv1.ListAccessibleProjectsResponse, error) {
	return f.listAccessibleProjectsFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) UpdateProjectItemField(ctx context.Context, in *scmintegrationv1.UpdateProjectItemFieldRequest, _ ...grpc.CallOption) (*scmintegrationv1.ProjectItem, error) {
	return f.updateProjectItemFieldFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) DeleteIssueCommentBySlug(ctx context.Context, in *scmintegrationv1.DeleteIssueCommentBySlugRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteIssueCommentBySlugFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) ListMergeRequests(ctx context.Context, in *scmintegrationv1.ListMergeRequestsRequest, _ ...grpc.CallOption) (*scmintegrationv1.ListMergeRequestsResponse, error) {
	return f.listMergeRequestsFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) ResolveMergeRequestDiscussion(ctx context.Context, in *scmintegrationv1.ResolveMergeRequestDiscussionRequest, _ ...grpc.CallOption) (*scmintegrationv1.MergeRequestDiscussion, error) {
	return f.resolveMergeRequestDiscussionFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) GetWorkItemDetails(ctx context.Context, in *scmintegrationv1.GetWorkItemDetailsRequest, _ ...grpc.CallOption) (*scmintegrationv1.WorkItemDetailsGitLab, error) {
	return f.getWorkItemDetailsFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) CreatePullRequest(ctx context.Context, in *scmintegrationv1.CreatePullRequestRequest, _ ...grpc.CallOption) (*scmintegrationv1.CreatePullRequestResponse, error) {
	return f.createPullRequestFunc(ctx, in)
}

func (f *fakeScmIntegrationClient) CheckHostedReviewEligibility(ctx context.Context, in *scmintegrationv1.CheckHostedReviewEligibilityRequest, _ ...grpc.CallOption) (*scmintegrationv1.HostedReviewEligibility, error) {
	return f.checkHostedReviewEligibilityFunc(ctx, in)
}

// ── github.* ──────────────────────────────────────────────────────────────

func TestGitHubMergePRChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.MergePullRequestRequest
	fake := &fakeScmIntegrationClient{
		mergePullRequestFunc: func(ctx context.Context, in *scmintegrationv1.MergePullRequestRequest) (*scmintegrationv1.MergePullRequestResponse, error) {
			gotReq = in
			return &scmintegrationv1.MergePullRequestResponse{Merged: true, Sha: "abc123"}, nil
		},
	}

	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "github.mergePR",
		argsJSON(t, map[string]any{"repo": "o/r", "number": 42, "mergeMethod": "squash"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.MergePullRequestResponse)
	if !ok || !resp.GetMerged() || resp.GetSha() != "abc123" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB {
		t.Errorf("expected SCM_PROVIDER_GITHUB, got %v", gotReq.GetProvider())
	}
	if gotReq.GetRepo() != "o/r" || gotReq.GetNumber() != 42 {
		t.Errorf("expected repo=o/r number=42, got repo=%s number=%d", gotReq.GetRepo(), gotReq.GetNumber())
	}
}

func TestGitHubRequestPRReviewersChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.RequestPullRequestReviewersRequest
	fake := &fakeScmIntegrationClient{
		requestPullRequestReviewersFunc: func(ctx context.Context, in *scmintegrationv1.RequestPullRequestReviewersRequest) (*scmintegrationv1.PullRequest, error) {
			gotReq = in
			return &scmintegrationv1.PullRequest{Id: "1", Number: 42}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.requestPRReviewers",
		argsJSON(t, map[string]any{"repo": "o/r", "number": 42, "reviewerLogins": []string{"alice"}, "teamSlugs": []string{"team-a"}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr, ok := result.(*scmintegrationv1.PullRequest); !ok || pr.GetNumber() != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(gotReq.GetReviewerLogins()) != 1 || gotReq.GetReviewerLogins()[0] != "alice" {
		t.Errorf("expected reviewerLogins passed through, got %+v", gotReq.GetReviewerLogins())
	}
}

func TestGitHubPRForBranchChannel_ReturnsNilWhenNotFound(t *testing.T) {
	fake := &fakeScmIntegrationClient{
		getPullRequestForBranchFunc: func(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
			return &scmintegrationv1.GetPullRequestForBranchResponse{Found: false}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.prForBranch",
		argsJSON(t, map[string]any{"repo": "o/r", "headBranch": "feature-x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result when not found, got %+v", result)
	}
}

func TestGitHubPRForBranchChannel_ReturnsPRWhenFound(t *testing.T) {
	fake := &fakeScmIntegrationClient{
		getPullRequestForBranchFunc: func(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
			return &scmintegrationv1.GetPullRequestForBranchResponse{Found: true, PullRequest: &scmintegrationv1.PullRequest{Id: "1", Number: 7}}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.prForBranch",
		argsJSON(t, map[string]any{"repo": "o/r", "headBranch": "feature-x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pr, ok := result.(*scmintegrationv1.PullRequest)
	if !ok || pr.GetNumber() != 7 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGitHubUpdateIssueChannel_OmitsUnsetOptionalFields(t *testing.T) {
	var gotReq *scmintegrationv1.UpdateIssueRequest
	fake := &fakeScmIntegrationClient{
		updateIssueFunc: func(ctx context.Context, in *scmintegrationv1.UpdateIssueRequest) (*scmintegrationv1.Issue, error) {
			gotReq = in
			return &scmintegrationv1.Issue{Id: "1"}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	// title/body/state absent from args[0] entirely.
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.updateIssue",
		argsJSON(t, map[string]any{"repo": "o/r", "number": 1, "addLabels": []string{"bug"}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Title != nil || gotReq.Body != nil || gotReq.State != nil {
		t.Errorf("expected unset optional fields to stay nil, got title=%v body=%v state=%v", gotReq.Title, gotReq.Body, gotReq.State)
	}
	if len(gotReq.GetAddLabels()) != 1 || gotReq.GetAddLabels()[0] != "bug" {
		t.Errorf("expected addLabels passed through, got %+v", gotReq.GetAddLabels())
	}
}

func TestGitHubUpdateIssueChannel_SetsProvidedOptionalFields(t *testing.T) {
	var gotReq *scmintegrationv1.UpdateIssueRequest
	fake := &fakeScmIntegrationClient{
		updateIssueFunc: func(ctx context.Context, in *scmintegrationv1.UpdateIssueRequest) (*scmintegrationv1.Issue, error) {
			gotReq = in
			return &scmintegrationv1.Issue{Id: "1"}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.updateIssue",
		argsJSON(t, map[string]any{"repo": "o/r", "number": 1, "title": "new title"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Title == nil || gotReq.GetTitle() != "new title" {
		t.Errorf("expected title set to %q, got %v", "new title", gotReq.Title)
	}
}

func TestGitHubRepoSlugChannel_Success(t *testing.T) {
	fake := &fakeScmIntegrationClient{
		resolveRepoSlugFunc: func(ctx context.Context, in *scmintegrationv1.ResolveRepoSlugRequest) (*scmintegrationv1.ResolveRepoSlugResponse, error) {
			return &scmintegrationv1.ResolveRepoSlugResponse{Owner: "octocat", Name: "hello-world", Slug: "octocat/hello-world"}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.repoSlug",
		argsJSON(t, map[string]any{"candidate": "git@github.com:octocat/hello-world.git"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.ResolveRepoSlugResponse)
	if !ok || resp.GetSlug() != "octocat/hello-world" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// ── github.project.* ─────────────────────────────────────────────────────

func TestGitHubProjectListAccessibleChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.ListAccessibleProjectsRequest
	fake := &fakeScmIntegrationClient{
		listAccessibleProjectsFunc: func(ctx context.Context, in *scmintegrationv1.ListAccessibleProjectsRequest) (*scmintegrationv1.ListAccessibleProjectsResponse, error) {
			gotReq = in
			return &scmintegrationv1.ListAccessibleProjectsResponse{Projects: []*scmintegrationv1.Project{{Slug: "acme/7"}}}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.project.listAccessible", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.ListAccessibleProjectsResponse)
	if !ok || len(resp.GetProjects()) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetTenantId() != "tenant-1" {
		t.Errorf("expected tenant_id passed through, got %q", gotReq.GetTenantId())
	}
}

func TestGitHubProjectUpdateItemFieldChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.UpdateProjectItemFieldRequest
	fake := &fakeScmIntegrationClient{
		updateProjectItemFieldFunc: func(ctx context.Context, in *scmintegrationv1.UpdateProjectItemFieldRequest) (*scmintegrationv1.ProjectItem, error) {
			gotReq = in
			return &scmintegrationv1.ProjectItem{Id: "item-1"}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.project.updateItemField",
		argsJSON(t, map[string]any{"projectSlug": "acme/7", "itemId": "item-1", "fieldId": "f1", "kind": "text", "value": "hi"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item, ok := result.(*scmintegrationv1.ProjectItem); !ok || item.GetId() != "item-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetField().GetKind() != "text" || gotReq.GetField().GetValue() != "hi" {
		t.Errorf("unexpected field: %+v", gotReq.GetField())
	}
}

func TestGitHubProjectDeleteIssueCommentBySlugChannel_ReturnsNil(t *testing.T) {
	var gotReq *scmintegrationv1.DeleteIssueCommentBySlugRequest
	fake := &fakeScmIntegrationClient{
		deleteIssueCommentBySlugFunc: func(ctx context.Context, in *scmintegrationv1.DeleteIssueCommentBySlugRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "github.project.deleteIssueCommentBySlug",
		argsJSON(t, map[string]any{"itemSlug": "acme/repo#1", "commentId": "c1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if gotReq.GetItemSlug() != "acme/repo#1" || gotReq.GetCommentId() != "c1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
}

// ── gitlab.* ──────────────────────────────────────────────────────────────

func TestGitLabListMRsChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.ListMergeRequestsRequest
	fake := &fakeScmIntegrationClient{
		listMergeRequestsFunc: func(ctx context.Context, in *scmintegrationv1.ListMergeRequestsRequest) (*scmintegrationv1.ListMergeRequestsResponse, error) {
			gotReq = in
			return &scmintegrationv1.ListMergeRequestsResponse{
				MergeRequests: []*scmintegrationv1.MergeRequest{{Iid: 42, Title: "Fix bug"}},
			}, nil
		},
	}

	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "gitlab.listMRs",
		argsJSON(t, map[string]any{"repo": "group/project", "state": "opened"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.ListMergeRequestsResponse)
	if !ok || len(resp.GetMergeRequests()) != 1 || resp.GetMergeRequests()[0].GetIid() != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetRepo() != "group/project" || gotReq.GetState() != "opened" {
		t.Errorf("expected repo=group/project state=opened, got repo=%s state=%s", gotReq.GetRepo(), gotReq.GetState())
	}
}

func TestGitLabResolveMRDiscussionChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.ResolveMergeRequestDiscussionRequest
	fake := &fakeScmIntegrationClient{
		resolveMergeRequestDiscussionFunc: func(ctx context.Context, in *scmintegrationv1.ResolveMergeRequestDiscussionRequest) (*scmintegrationv1.MergeRequestDiscussion, error) {
			gotReq = in
			return &scmintegrationv1.MergeRequestDiscussion{Id: "disc-1", Resolved: true}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "gitlab.resolveMRDiscussion",
		argsJSON(t, map[string]any{"repo": "group/project", "mergeRequestIid": 42, "discussionId": "disc-1", "resolved": true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if disc, ok := result.(*scmintegrationv1.MergeRequestDiscussion); !ok || !disc.GetResolved() {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetMergeRequestIid() != 42 {
		t.Errorf("expected mergeRequestIid=42, got %d", gotReq.GetMergeRequestIid())
	}
}

func TestGitLabWorkItemDetailsChannel_Success(t *testing.T) {
	fake := &fakeScmIntegrationClient{
		getWorkItemDetailsFunc: func(ctx context.Context, in *scmintegrationv1.GetWorkItemDetailsRequest) (*scmintegrationv1.WorkItemDetailsGitLab, error) {
			return &scmintegrationv1.WorkItemDetailsGitLab{Iid: in.GetIid(), Title: "Bug"}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "gitlab.workItemDetails",
		argsJSON(t, map[string]any{"repo": "group/project", "iid": 42, "itemType": "issue"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if details, ok := result.(*scmintegrationv1.WorkItemDetailsGitLab); !ok || details.GetIid() != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGitLabRateLimitChannelMatchesRESTContract(t *testing.T) {
	// Both github.rateLimit/gitlab.rateLimit and the REST route
	// (GET /v1/scm/rate-limit?provider=...) resolve to the identical
	// GetRateLimitStatus RPC and return its response verbatim — assert the
	// channel's result equals a direct call against the same fake, the same
	// regression-guard shape scm_routes_test.go uses for the REST side.
	want := &scmintegrationv1.GetRateLimitStatusResponse{Remaining: 100, Limit: 5000, ResetUnix: 1234}
	fake := &fakeScmIntegrationClient{
		getRateLimitStatusFunc: func(ctx context.Context, in *scmintegrationv1.GetRateLimitStatusRequest) (*scmintegrationv1.GetRateLimitStatusResponse, error) {
			if in.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB {
				t.Fatalf("expected SCM_PROVIDER_GITLAB, got %v", in.GetProvider())
			}
			return want, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "gitlab.rateLimit", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	direct, err := fake.GetRateLimitStatus(context.Background(), &scmintegrationv1.GetRateLimitStatusRequest{TenantId: "tenant-1", Provider: scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB})
	if err != nil {
		t.Fatalf("unexpected error calling the RPC directly: %v", err)
	}
	if !reflect.DeepEqual(result, direct) {
		t.Fatalf("expected channel result to match a direct GetRateLimitStatus call, got %+v vs %+v", result, direct)
	}
}

// ── hostedReview.* ────────────────────────────────────────────────────────

func TestHostedReviewCreateChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.CreatePullRequestRequest
	fake := &fakeScmIntegrationClient{
		createPullRequestFunc: func(ctx context.Context, in *scmintegrationv1.CreatePullRequestRequest) (*scmintegrationv1.CreatePullRequestResponse, error) {
			gotReq = in
			return &scmintegrationv1.CreatePullRequestResponse{PullRequest: &scmintegrationv1.PullRequest{Id: "1", Number: 1}}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "hostedReview.create",
		argsJSON(t, map[string]any{"provider": "gitlab", "repo": "group/project", "title": "t", "headBranch": "h", "baseBranch": "b"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pr, ok := result.(*scmintegrationv1.PullRequest)
	if !ok || pr.GetId() != "1" {
		t.Fatalf("expected the unwrapped PullRequest, got %+v", result)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB {
		t.Errorf("expected SCM_PROVIDER_GITLAB from parseWSProvider(\"gitlab\"), got %v", gotReq.GetProvider())
	}
}

func TestHostedReviewForBranchChannel_ReturnsNilWhenNotFound(t *testing.T) {
	fake := &fakeScmIntegrationClient{
		getPullRequestForBranchFunc: func(ctx context.Context, in *scmintegrationv1.GetPullRequestForBranchRequest) (*scmintegrationv1.GetPullRequestForBranchResponse, error) {
			return &scmintegrationv1.GetPullRequestForBranchResponse{Found: false}, nil
		},
	}
	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "hostedReview.forBranch",
		argsJSON(t, map[string]any{"provider": "github", "repo": "o/r", "headBranch": "feature-x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result when not found, got %+v", result)
	}
}

func TestHostedReviewGetCreationEligibilityChannel_Success(t *testing.T) {
	var gotReq *scmintegrationv1.CheckHostedReviewEligibilityRequest
	fake := &fakeScmIntegrationClient{
		checkHostedReviewEligibilityFunc: func(ctx context.Context, in *scmintegrationv1.CheckHostedReviewEligibilityRequest) (*scmintegrationv1.HostedReviewEligibility, error) {
			gotReq = in
			return &scmintegrationv1.HostedReviewEligibility{Eligible: true}, nil
		},
	}

	r := NewRegistry()
	registerSCMChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "hostedReview.getCreationEligibility",
		argsJSON(t, map[string]any{"provider": "gitlab", "repo": "group/project", "headBranch": "feature-x", "baseBranch": "main"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*scmintegrationv1.HostedReviewEligibility)
	if !ok || !resp.GetEligible() {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gotReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITLAB {
		t.Errorf("expected SCM_PROVIDER_GITLAB from parseWSProvider(\"gitlab\"), got %v", gotReq.GetProvider())
	}
}

func TestParseWSProvider_UnknownStringIsUnspecified(t *testing.T) {
	if got := parseWSProvider("not-a-real-provider"); got != scmintegrationv1.ScmProvider_SCM_PROVIDER_UNSPECIFIED {
		t.Errorf("expected SCM_PROVIDER_UNSPECIFIED for an unknown string, got %v", got)
	}
}
