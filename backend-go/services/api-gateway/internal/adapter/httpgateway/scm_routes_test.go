package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// fakeScmIntegrationServiceClient implements
// scmintegrationv1.ScmIntegrationServiceClient with per-method canned
// responses/errors, configurable per test. Embeds the real client interface
// (nil) so new RPCs added to ScmIntegrationServiceClient (TASK-071..090)
// don't need a hand-written override here unless this file's REST routes
// actually exercise them — same embed-and-override convention
// channels_test.go's wscompat fakes use.
type fakeScmIntegrationServiceClient struct {
	scmintegrationv1.ScmIntegrationServiceClient

	listIssuesResp *scmintegrationv1.ListIssuesResponse
	listIssuesErr  error
	listIssuesReq  *scmintegrationv1.ListIssuesRequest // captures the last request for assertions

	listIssueCommentsBySlugResp *scmintegrationv1.ListIssueCommentsBySlugResponse
	listIssueCommentsBySlugErr  error
	listIssueCommentsBySlugReq  *scmintegrationv1.ListIssueCommentsBySlugRequest

	createPullRequestResp *scmintegrationv1.CreatePullRequestResponse
	createPullRequestErr  error
	createPullRequestReq  *scmintegrationv1.CreatePullRequestRequest // captures the last request for assertions

	listPullRequestsResp *scmintegrationv1.ListPullRequestsResponse
	listPullRequestsErr  error

	getRateLimitStatusResp *scmintegrationv1.GetRateLimitStatusResponse
	getRateLimitStatusErr  error

	getAuthStatusResp *scmintegrationv1.GetAuthStatusResponse
	getAuthStatusErr  error

	startOAuthFlowResp *scmintegrationv1.StartOAuthFlowResponse
	startOAuthFlowErr  error

	completeOAuthFlowResp *scmintegrationv1.CompleteOAuthFlowResponse
	completeOAuthFlowErr  error

	revokeAuthResp *scmintegrationv1.RevokeAuthResponse
	revokeAuthErr  error
}

func (f *fakeScmIntegrationServiceClient) ListIssues(_ context.Context, in *scmintegrationv1.ListIssuesRequest, _ ...grpc.CallOption) (*scmintegrationv1.ListIssuesResponse, error) {
	f.listIssuesReq = in
	if f.listIssuesErr != nil {
		return nil, f.listIssuesErr
	}
	return f.listIssuesResp, nil
}

func (f *fakeScmIntegrationServiceClient) ListIssueCommentsBySlug(_ context.Context, in *scmintegrationv1.ListIssueCommentsBySlugRequest, _ ...grpc.CallOption) (*scmintegrationv1.ListIssueCommentsBySlugResponse, error) {
	f.listIssueCommentsBySlugReq = in
	if f.listIssueCommentsBySlugErr != nil {
		return nil, f.listIssueCommentsBySlugErr
	}
	return f.listIssueCommentsBySlugResp, nil
}

func (f *fakeScmIntegrationServiceClient) CreatePullRequest(_ context.Context, in *scmintegrationv1.CreatePullRequestRequest, _ ...grpc.CallOption) (*scmintegrationv1.CreatePullRequestResponse, error) {
	f.createPullRequestReq = in
	if f.createPullRequestErr != nil {
		return nil, f.createPullRequestErr
	}
	return f.createPullRequestResp, nil
}

func (f *fakeScmIntegrationServiceClient) ListPullRequests(_ context.Context, _ *scmintegrationv1.ListPullRequestsRequest, _ ...grpc.CallOption) (*scmintegrationv1.ListPullRequestsResponse, error) {
	if f.listPullRequestsErr != nil {
		return nil, f.listPullRequestsErr
	}
	return f.listPullRequestsResp, nil
}

func (f *fakeScmIntegrationServiceClient) GetRateLimitStatus(_ context.Context, _ *scmintegrationv1.GetRateLimitStatusRequest, _ ...grpc.CallOption) (*scmintegrationv1.GetRateLimitStatusResponse, error) {
	if f.getRateLimitStatusErr != nil {
		return nil, f.getRateLimitStatusErr
	}
	return f.getRateLimitStatusResp, nil
}

func (f *fakeScmIntegrationServiceClient) GetAuthStatus(_ context.Context, _ *scmintegrationv1.GetAuthStatusRequest, _ ...grpc.CallOption) (*scmintegrationv1.GetAuthStatusResponse, error) {
	if f.getAuthStatusErr != nil {
		return nil, f.getAuthStatusErr
	}
	return f.getAuthStatusResp, nil
}

func (f *fakeScmIntegrationServiceClient) StartOAuthFlow(_ context.Context, _ *scmintegrationv1.StartOAuthFlowRequest, _ ...grpc.CallOption) (*scmintegrationv1.StartOAuthFlowResponse, error) {
	if f.startOAuthFlowErr != nil {
		return nil, f.startOAuthFlowErr
	}
	return f.startOAuthFlowResp, nil
}

func (f *fakeScmIntegrationServiceClient) CompleteOAuthFlow(_ context.Context, _ *scmintegrationv1.CompleteOAuthFlowRequest, _ ...grpc.CallOption) (*scmintegrationv1.CompleteOAuthFlowResponse, error) {
	if f.completeOAuthFlowErr != nil {
		return nil, f.completeOAuthFlowErr
	}
	return f.completeOAuthFlowResp, nil
}

func (f *fakeScmIntegrationServiceClient) RevokeAuth(_ context.Context, _ *scmintegrationv1.RevokeAuthRequest, _ ...grpc.CallOption) (*scmintegrationv1.RevokeAuthResponse, error) {
	if f.revokeAuthErr != nil {
		return nil, f.revokeAuthErr
	}
	return f.revokeAuthResp, nil
}

// scmTestRouter mounts mountSCMRoutes standalone (router.go isn't touched
// by this package's tests, per task instructions).
func scmTestRouter(client scmintegrationv1.ScmIntegrationServiceClient) chi.Router {
	r := chi.NewRouter()
	mountSCMRoutes(r, client)
	return r
}

// withTestIdentity is shared across this package's route tests —
// git_routes_test.go defines it.

func TestHandleCreatePullRequest_SuccessRoundTrip(t *testing.T) {
	fake := &fakeScmIntegrationServiceClient{
		createPullRequestResp: &scmintegrationv1.CreatePullRequestResponse{
			PullRequest: &scmintegrationv1.PullRequest{
				Id:    "pr-1",
				Url:   "https://github.com/acme/repo/pull/1",
				State: "open",
			},
		},
	}
	router := scmTestRouter(fake)

	body, _ := json.Marshal(createPullRequestRequestBody{
		Provider:   "github",
		Repo:       "acme/repo",
		Title:      "Add feature",
		HeadBranch: "feature",
		BaseBranch: "main",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/scm/pull-requests", bytes.NewReader(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var gotPR scmintegrationv1.PullRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &gotPR); err != nil {
		t.Fatalf("response body is not the expected PullRequest JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if gotPR.GetId() != "pr-1" || gotPR.GetState() != "open" {
		t.Fatalf("unexpected pull request in response: %+v", &gotPR)
	}

	if fake.createPullRequestReq == nil {
		t.Fatal("CreatePullRequest was never called")
	}
	if fake.createPullRequestReq.GetProvider() != scmintegrationv1.ScmProvider_SCM_PROVIDER_GITHUB {
		t.Fatalf("provider sent upstream = %v, want SCM_PROVIDER_GITHUB", fake.createPullRequestReq.GetProvider())
	}
}

// TestHandleCreatePullRequest_TenantIDComesFromIdentityNotBody is the
// security regression test: tenant_id must NEVER be trusted from the JSON
// request body, only from identityFromContext — matching every existing
// handler in usage_routes.go and auth_routes.go.
func TestHandleCreatePullRequest_TenantIDComesFromIdentityNotBody(t *testing.T) {
	fake := &fakeScmIntegrationServiceClient{
		createPullRequestResp: &scmintegrationv1.CreatePullRequestResponse{
			PullRequest: &scmintegrationv1.PullRequest{Id: "pr-1"},
		},
	}
	router := scmTestRouter(fake)

	// Attacker-controlled body claims a different tenant than the caller's
	// validated identity.
	rawBody := []byte(`{"repo":"acme/repo","title":"steal data","tenant_id":"attacker-tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/scm/pull-requests", bytes.NewReader(rawBody))
	req = withTestIdentity(req, usecase.Identity{TenantID: "real-tenant", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if fake.createPullRequestReq == nil {
		t.Fatal("CreatePullRequest was never called")
	}
	if fake.createPullRequestReq.GetTenantId() != "real-tenant" {
		t.Fatalf("TenantId sent upstream = %q, want %q (from identity, not body)", fake.createPullRequestReq.GetTenantId(), "real-tenant")
	}
}

func TestHandleGetRateLimitStatus_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	fake := &fakeScmIntegrationServiceClient{
		getRateLimitStatusErr: status.Error(codes.Unimplemented, "not implemented for provider"),
	}
	router := scmTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/scm/rate-limit?provider=gitea", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected error JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.Unimplemented.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.Unimplemented.String())
	}
}

func TestHandleGetAuthStatus_SuccessRoundTrip(t *testing.T) {
	fake := &fakeScmIntegrationServiceClient{
		getAuthStatusResp: &scmintegrationv1.GetAuthStatusResponse{Connected: true},
	}
	router := scmTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/scm/auth-status?provider=github", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got scmintegrationv1.GetAuthStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if !got.GetConnected() {
		t.Fatal("connected = false, want true")
	}
}

// TestScmRoutes_ListIssues_FiltersAndForceRefreshForwarded is TASK-PI-01-08's
// gateway-level regression guard: state/assignee/label(repeated)/milestone
// query params and refresh=true must reach the RPC's Filter/ForceRefresh
// fields, not be silently dropped at the REST edge.
func TestScmRoutes_ListIssues_FiltersAndForceRefreshForwarded(t *testing.T) {
	fake := &fakeScmIntegrationServiceClient{
		listIssuesResp: &scmintegrationv1.ListIssuesResponse{},
	}
	router := scmTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/scm/issues?provider=github&repo=acme/repo&state=open&assignee=octocat&label=bug&label=p0&milestone=v1&refresh=true", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.listIssuesReq == nil {
		t.Fatal("ListIssues was never called")
	}
	f := fake.listIssuesReq.GetFilter()
	if f.GetState() != "open" || f.GetAssignee() != "octocat" || f.GetMilestone() != "v1" {
		t.Fatalf("unexpected filter forwarded: %+v", f)
	}
	if len(f.GetLabels()) != 2 || f.GetLabels()[0] != "bug" || f.GetLabels()[1] != "p0" {
		t.Fatalf("expected repeated label query params to become Filter.Labels, got %v", f.GetLabels())
	}
	if !fake.listIssuesReq.GetForceRefresh() {
		t.Fatal("expected refresh=true to map to ForceRefresh")
	}
}

// TestScmRoutes_ListIssueComments_RoundTrip round-trips the new
// GET /v1/scm/issues/{number}/comments route through a fake
// ScmIntegrationServiceClient.
func TestScmRoutes_ListIssueComments_RoundTrip(t *testing.T) {
	fake := &fakeScmIntegrationServiceClient{
		listIssueCommentsBySlugResp: &scmintegrationv1.ListIssueCommentsBySlugResponse{
			Comments: []*scmintegrationv1.ProjectComment{{Id: "c-1", Body: "looks good"}},
		},
	}
	router := scmTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/scm/issues/42/comments?repo=acme/repo", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.listIssueCommentsBySlugReq == nil {
		t.Fatal("ListIssueCommentsBySlug was never called")
	}
	if fake.listIssueCommentsBySlugReq.GetItemSlug() != "acme/repo#42" {
		t.Fatalf("item_slug = %q, want %q", fake.listIssueCommentsBySlugReq.GetItemSlug(), "acme/repo#42")
	}
	if fake.listIssueCommentsBySlugReq.GetTenantId() != "tenant-1" {
		t.Fatalf("tenant_id = %q, want %q", fake.listIssueCommentsBySlugReq.GetTenantId(), "tenant-1")
	}

	var got scmintegrationv1.ListIssueCommentsBySlugResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if len(got.GetComments()) != 1 || got.GetComments()[0].GetId() != "c-1" {
		t.Fatalf("unexpected comments in response: %+v", got.GetComments())
	}
}
