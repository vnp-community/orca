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

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
)

// fakeIssueTrackingServiceClient implements
// issuetrackingv1.IssueTrackingServiceClient with per-method canned
// responses/errors, configurable per test.
type fakeIssueTrackingServiceClient struct {
	listIssuesResp *issuetrackingv1.ListIssuesResponse
	listIssuesErr  error

	createIssueResp *issuetrackingv1.CreateIssueResponse
	createIssueErr  error
	createIssueReq  *issuetrackingv1.CreateIssueRequest // captures the last request for assertions

	linkIssueResp *issuetrackingv1.LinkIssueResponse
	linkIssueErr  error
}

func (f *fakeIssueTrackingServiceClient) ListIssues(_ context.Context, _ *issuetrackingv1.ListIssuesRequest, _ ...grpc.CallOption) (*issuetrackingv1.ListIssuesResponse, error) {
	if f.listIssuesErr != nil {
		return nil, f.listIssuesErr
	}
	return f.listIssuesResp, nil
}

func (f *fakeIssueTrackingServiceClient) CreateIssue(_ context.Context, in *issuetrackingv1.CreateIssueRequest, _ ...grpc.CallOption) (*issuetrackingv1.CreateIssueResponse, error) {
	f.createIssueReq = in
	if f.createIssueErr != nil {
		return nil, f.createIssueErr
	}
	return f.createIssueResp, nil
}

func (f *fakeIssueTrackingServiceClient) LinkIssue(_ context.Context, _ *issuetrackingv1.LinkIssueRequest, _ ...grpc.CallOption) (*issuetrackingv1.LinkIssueResponse, error) {
	if f.linkIssueErr != nil {
		return nil, f.linkIssueErr
	}
	return f.linkIssueResp, nil
}

// issueTrackingTestRouter mounts mountIssueTrackingRoutes standalone
// (router.go isn't touched by this package's tests, per task instructions)
// and injects an identity into the request context the same way
// authMiddleware would (see withTestIdentity in task_routes_test.go).
func issueTrackingTestRouter(client issuetrackingv1.IssueTrackingServiceClient) chi.Router {
	r := chi.NewRouter()
	mountIssueTrackingRoutes(r, client)
	return r
}

func TestHandleCreateIssue_SuccessRoundTrip(t *testing.T) {
	fake := &fakeIssueTrackingServiceClient{
		createIssueResp: &issuetrackingv1.CreateIssueResponse{
			Issue: &issuetrackingv1.Issue{
				Id:    "issue-1",
				Title: "Fix the bug",
				State: "open",
				Url:   "https://example.atlassian.net/browse/ISSUE-1",
			},
		},
	}
	router := issueTrackingTestRouter(fake)

	body, _ := json.Marshal(createIssueRequestBody{
		Provider:   "jira",
		ProjectKey: "ISSUE",
		Title:      "Fix the bug",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/issues/", bytes.NewReader(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var gotIssue issuetrackingv1.Issue
	if err := json.Unmarshal(rec.Body.Bytes(), &gotIssue); err != nil {
		t.Fatalf("response body is not the expected Issue JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if gotIssue.GetId() != "issue-1" || gotIssue.GetTitle() != "Fix the bug" {
		t.Fatalf("unexpected issue in response: %+v", &gotIssue)
	}

	if fake.createIssueReq == nil {
		t.Fatal("CreateIssue was never called")
	}
	if fake.createIssueReq.GetTenantId() != "tenant-1" {
		t.Fatalf("TenantId sent upstream = %q, want %q (from identity, not body)", fake.createIssueReq.GetTenantId(), "tenant-1")
	}
	if fake.createIssueReq.GetProvider() != issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA {
		t.Fatalf("Provider sent upstream = %v, want %v", fake.createIssueReq.GetProvider(), issuetrackingv1.IssueProvider_ISSUE_PROVIDER_JIRA)
	}
}

// TestHandleCreateIssue_TenantIDComesFromIdentityNotBody is the security
// regression test: tenant_id must NEVER be trusted from the JSON request
// body, only from identityFromContext — matching task_routes_test.go's
// equivalent test.
func TestHandleCreateIssue_TenantIDComesFromIdentityNotBody(t *testing.T) {
	fake := &fakeIssueTrackingServiceClient{
		createIssueResp: &issuetrackingv1.CreateIssueResponse{Issue: &issuetrackingv1.Issue{Id: "issue-1"}},
	}
	router := issueTrackingTestRouter(fake)

	// Attacker-controlled body claims a different tenant than the caller's
	// validated identity.
	rawBody := []byte(`{"title":"steal data","tenant_id":"attacker-tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/issues/", bytes.NewReader(rawBody))
	req = withTestIdentity(req, usecase.Identity{TenantID: "real-tenant", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if fake.createIssueReq == nil {
		t.Fatal("CreateIssue was never called")
	}
	if fake.createIssueReq.GetTenantId() != "real-tenant" {
		t.Fatalf("TenantId sent upstream = %q, want %q (from identity, not body)", fake.createIssueReq.GetTenantId(), "real-tenant")
	}
}

func TestHandleListIssues_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	fake := &fakeIssueTrackingServiceClient{
		listIssuesErr: status.Error(codes.Unavailable, "issue-tracking-service unreachable"),
	}
	router := issueTrackingTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/issues/?provider=jira&project_key=ISSUE", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected error JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.Unavailable.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.Unavailable.String())
	}
}

func TestHandleLinkIssue_SuccessRoundTrip(t *testing.T) {
	fake := &fakeIssueTrackingServiceClient{
		linkIssueResp: &issuetrackingv1.LinkIssueResponse{},
	}
	router := issueTrackingTestRouter(fake)

	body, _ := json.Marshal(linkIssueRequestBody{IssueID: "issue-1", TaskID: "task-1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/issues/link", bytes.NewReader(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
