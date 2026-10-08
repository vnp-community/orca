package httpgateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// fakeRequestRoutesClient is a request-service double for both clients; unscripted RPCs panic.
type fakeRequestRoutesClient struct {
	requestv1.RequestServiceClient
	requestv1.ApprovalServiceClient

	mu       sync.Mutex
	ctx      context.Context
	creates  []*requestv1.CreateRequestRequest
	lists    []*requestv1.ListRequestsRequest
	pendings []*requestv1.ListPendingForUserRequest
	approves []*requestv1.ApproveRequest
	rejects  []*requestv1.RejectRequest
	gets     []string
	created  bool
	err      error
}

func (f *fakeRequestRoutesClient) CreateRequest(ctx context.Context, in *requestv1.CreateRequestRequest, _ ...grpc.CallOption) (*requestv1.CreateRequestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctx, f.creates = ctx, append(f.creates, in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.CreateRequestResponse{Request: &requestv1.Request{Id: "r1", Title: in.GetTitle(), Body: "must-not-be-returned", SourceProvider: in.GetSource().GetProvider()}, Created: f.created}, nil
}

func (f *fakeRequestRoutesClient) GetRequest(ctx context.Context, in *requestv1.GetRequestRequest, _ ...grpc.CallOption) (*requestv1.GetRequestResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctx, f.gets = ctx, append(f.gets, in.GetId())
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetRequestResponse{Request: &requestv1.Request{Id: in.GetId(), ProjectId: "p1", Title: "T", Body: "the body"}}, nil
}

func (f *fakeRequestRoutesClient) ListRequests(ctx context.Context, in *requestv1.ListRequestsRequest, _ ...grpc.CallOption) (*requestv1.ListRequestsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctx, f.lists = ctx, append(f.lists, in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListRequestsResponse{NextPageToken: "n"}, nil
}

func (f *fakeRequestRoutesClient) ListPendingForUser(ctx context.Context, in *requestv1.ListPendingForUserRequest, _ ...grpc.CallOption) (*requestv1.ListPendingForUserResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctx, f.pendings = ctx, append(f.pendings, in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListPendingForUserResponse{Approvals: []*requestv1.Approval{{Id: "a1", RequestId: "r1", RequestTitle: "T"}}}, nil
}

func (f *fakeRequestRoutesClient) Approve(ctx context.Context, in *requestv1.ApproveRequest, _ ...grpc.CallOption) (*requestv1.ApproveResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctx, f.approves = ctx, append(f.approves, in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ApproveResponse{Approval: &requestv1.Approval{Id: in.GetId()}, RequestStatus: "planning"}, nil
}

func (f *fakeRequestRoutesClient) Reject(ctx context.Context, in *requestv1.RejectRequest, _ ...grpc.CallOption) (*requestv1.RejectResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctx, f.rejects = ctx, append(f.rejects, in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.RejectResponse{Approval: &requestv1.Approval{Id: in.GetId()}, RequestStatus: "analyzing"}, nil
}

func requestRoutesRouter(c *fakeRequestRoutesClient) chi.Router {
	r := chi.NewRouter()
	mountRequestRoutes(r, c, c)
	return r
}

func doRequestRoute(r http.Handler, method, path, body string, id *usecase.Identity) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if id != nil {
		req = withTestIdentity(req, *id)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

var routesIdentity = &usecase.Identity{TenantID: "tenant-1", UserID: "user-1", Role: "user"}

func decodeJSONMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON: %s", rec.Body.String())
	}
	return m
}

func TestRequestRoutes_CreateRoundTrip(t *testing.T) {
	fake := &fakeRequestRoutesClient{created: true}
	rec := doRequestRoute(requestRoutesRouter(fake), http.MethodPost, "/v1/requests/",
		`{"projectId":"p1","title":"T","body":"B","clientRequestId":"c1","tenantId":"EVIL","userId":"EVIL","reporterId":"EVIL"}`, routesIdentity)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	in := fake.creates[0]
	if in.GetProjectId() != "p1" || in.GetClientRequestId() != "c1" || in.GetSource().GetProvider() != "manual" {
		t.Errorf("rpc = %+v", in)
	}
	if got := outgoingTenantID(t, fake.ctx); got != "tenant-1" {
		t.Errorf("tenant on the call = %q, want the session tenant", got)
	}
	if strings.Contains(rec.Body.String(), "EVIL") || strings.Contains(rec.Body.String(), "must-not-be-returned") {
		t.Errorf("response leaks: %s", rec.Body.String())
	}
	if m := decodeJSONMap(t, rec); m["created"] != true || m["request"].(map[string]any)["sourceProvider"] != "manual" {
		t.Errorf("body = %v", m)
	}
}

func TestRequestRoutes_CreateRepeatIs200(t *testing.T) {
	fake := &fakeRequestRoutesClient{created: false}
	rec := doRequestRoute(requestRoutesRouter(fake), http.MethodPost, "/v1/requests/", `{"projectId":"p1","title":"T"}`, routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRequestRoutes_CreateSourceRules(t *testing.T) {
	for _, provider := range []string{"manual", "mcp", "webhook", "x"} {
		fake := &fakeRequestRoutesClient{}
		rec := doRequestRoute(requestRoutesRouter(fake), http.MethodPost, "/v1/requests/",
			`{"projectId":"p1","title":"T","source":{"provider":"`+provider+`"}}`, routesIdentity)
		m := decodeJSONMap(t, rec)
		if rec.Code != http.StatusForbidden || m["error"].(map[string]any)["code"] != "REQUEST_SOURCE_FORBIDDEN" || len(fake.creates) != 0 {
			t.Errorf("provider %s: status=%d body=%s rpcs=%d", provider, rec.Code, rec.Body.String(), len(fake.creates))
		}
	}
	fake := &fakeRequestRoutesClient{created: true}
	rec := doRequestRoute(requestRoutesRouter(fake), http.MethodPost, "/v1/requests/",
		`{"projectId":"p1","title":"T","source":{"provider":"github","ref":"o/r#1","url":"https://github.com/o/r/issues/1"}}`, routesIdentity)
	if rec.Code != http.StatusCreated || fake.creates[0].GetSource().GetProvider() != "github" || fake.creates[0].GetSource().GetRef() != "o/r#1" {
		t.Errorf("tracker source: status=%d rpc=%+v", rec.Code, fake.creates)
	}
}

func TestRequestRoutes_GetAndList(t *testing.T) {
	fake := &fakeRequestRoutesClient{}
	r := requestRoutesRouter(fake)
	rec := doRequestRoute(r, http.MethodGet, "/v1/requests/r9", "", routesIdentity)
	if rec.Code != http.StatusOK || fake.gets[0] != "r9" || decodeJSONMap(t, rec)["request"].(map[string]any)["body"] != "the body" {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	rec = doRequestRoute(r, http.MethodGet, "/v1/requests/?projectId=p1&status=new&status=analyzing,planning&type=bug&pageSize=500&pageToken=tk&sourceProvider=jira", "", routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	in := fake.lists[0]
	if strings.Join(in.GetStatus(), ",") != "new,analyzing,planning" || strings.Join(in.GetType(), ",") != "bug" ||
		in.GetPageSize() != 100 || in.GetPageToken() != "tk" || in.GetProjectId() != "p1" || in.GetSourceProvider() != "jira" {
		t.Errorf("list rpc = %+v", in)
	}
	m := decodeJSONMap(t, rec)
	if reqs, ok := m["requests"].([]any); !ok || len(reqs) != 0 || m["nextPageToken"] != "n" {
		t.Errorf("list body = %v", m)
	}
	if rec = doRequestRoute(r, http.MethodGet, "/v1/requests/?pageSize=abc", "", routesIdentity); rec.Code != http.StatusBadRequest {
		t.Errorf("bad pageSize: %d", rec.Code)
	}
	if rec = doRequestRoute(r, http.MethodGet, "/v1/requests/?status=odd", "", routesIdentity); rec.Code != http.StatusBadRequest {
		t.Errorf("bad status: %d", rec.Code)
	}
}

func TestRequestRoutes_PendingApprovals(t *testing.T) {
	fake := &fakeRequestRoutesClient{}
	rec := doRequestRoute(requestRoutesRouter(fake), http.MethodGet, "/v1/approvals/pending?subjectType=solution&pageSize=1&userId=EVIL", "", routesIdentity)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	in := fake.pendings[0]
	if in.GetSubjectType() != requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_SOLUTION || in.GetPageSize() != 1 {
		t.Errorf("rpc = %+v", in)
	}
	if outgoingTenantID(t, fake.ctx) != "tenant-1" {
		t.Error("tenant must come from the session")
	}
	if a := decodeJSONMap(t, rec)["approvals"].([]any)[0].(map[string]any); a["requestTitle"] != "T" {
		t.Errorf("approval = %v", a)
	}
}

func TestRequestRoutes_ApproveAndReject(t *testing.T) {
	fake := &fakeRequestRoutesClient{}
	r := requestRoutesRouter(fake)
	rec := doRequestRoute(r, http.MethodPost, "/v1/approvals/a1/approve", `{"version":2,"digest":"dg","comment":"ok","id":"EVIL"}`, routesIdentity)
	if rec.Code != http.StatusOK || decodeJSONMap(t, rec)["requestStatus"] != "planning" {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	if a := fake.approves[0]; a.GetId() != "a1" || a.GetExpectedVersion() != 2 || a.GetExpectedDigest() != "dg" || a.GetComment() != "ok" {
		t.Errorf("approve rpc = %+v", a)
	}
	rec = doRequestRoute(r, http.MethodPost, "/v1/approvals/a2/reject", `{"version":3,"digest":"dg","comment":"no"}`, routesIdentity)
	if rec.Code != http.StatusOK || fake.rejects[0].GetId() != "a2" {
		t.Fatalf("reject: %d %s", rec.Code, rec.Body.String())
	}

	// The old CR body {version, comment} has no digest: refused before any RPC.
	rec = doRequestRoute(r, http.MethodPost, "/v1/approvals/a1/approve", `{"version":2,"comment":"ok"}`, routesIdentity)
	if rec.Code != http.StatusBadRequest || len(fake.approves) != 1 {
		t.Errorf("approve without digest: %d rpcs=%d", rec.Code, len(fake.approves))
	}
	rec = doRequestRoute(r, http.MethodPost, "/v1/approvals/a1/reject", `{"version":2,"digest":"dg"}`, routesIdentity)
	if rec.Code != http.StatusBadRequest || len(fake.rejects) != 1 || decodeJSONMap(t, rec)["error"].(map[string]any)["code"] != "REQUEST_APPROVAL_COMMENT_REQUIRED" {
		t.Errorf("reject without comment: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRequestRoutes_BadJSONAndNoIdentity(t *testing.T) {
	fake := &fakeRequestRoutesClient{}
	r := requestRoutesRouter(fake)
	for _, path := range []string{"/v1/requests/", "/v1/approvals/a1/approve", "/v1/approvals/a1/reject"} {
		rec := doRequestRoute(r, http.MethodPost, path, `{not json`, routesIdentity)
		if rec.Code != http.StatusBadRequest || decodeJSONMap(t, rec)["error"].(map[string]any)["code"] != "INVALID_ARGUMENT" {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	for _, c := range [][2]string{{http.MethodGet, "/v1/requests/r1"}, {http.MethodGet, "/v1/requests/"}, {http.MethodPost, "/v1/requests/"}, {http.MethodGet, "/v1/approvals/pending"}, {http.MethodPost, "/v1/approvals/a/approve"}} {
		if rec := doRequestRoute(r, c[0], c[1], `{}`, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without identity: %d", c[0], c[1], rec.Code)
		}
	}
}

func TestRequestRoutes_ServiceErrorsKeepCodeAndMapStatus(t *testing.T) {
	cases := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{status.Error(codes.NotFound, "REQUEST_NOT_FOUND: no such request"), http.StatusNotFound, "REQUEST_NOT_FOUND"},
		{status.Error(codes.FailedPrecondition, "REQUEST_APPROVAL_VERSION_CONFLICT: reload"), http.StatusConflict, "REQUEST_APPROVAL_VERSION_CONFLICT"},
		{status.Error(codes.FailedPrecondition, "REQUEST_TRANSITION_NOT_ALLOWED: nope"), http.StatusPreconditionFailed, "REQUEST_TRANSITION_NOT_ALLOWED"},
		{status.Error(codes.PermissionDenied, "REQUEST_APPROVAL_NOT_APPROVER: no"), http.StatusForbidden, "REQUEST_APPROVAL_NOT_APPROVER"},
		{status.Error(codes.PermissionDenied, "x"), http.StatusForbidden, "REQUEST_FORBIDDEN"},
		{status.Error(codes.Unavailable, "dial tcp 10.0.0.1:9090 refused"), http.StatusServiceUnavailable, "REQUEST_UNAVAILABLE"},
		{status.Error(codes.Unimplemented, "unimplemented"), http.StatusNotImplemented, "REQUEST_NOT_IMPLEMENTED"},
		{status.Error(codes.ResourceExhausted, "REQUEST_PENDING_LIMIT: slow down"), http.StatusTooManyRequests, "REQUEST_PENDING_LIMIT"},
		{status.Error(codes.Internal, "pq: broken SECRET"), http.StatusInternalServerError, "REQUEST_INTERNAL"},
	}
	for _, c := range cases {
		fake := &fakeRequestRoutesClient{err: c.err}
		rec := doRequestRoute(requestRoutesRouter(fake), http.MethodGet, "/v1/requests/r1", "", routesIdentity)
		e, _ := decodeJSONMap(t, rec)["error"].(map[string]any)
		if rec.Code != c.wantStatus || e["code"] != c.wantCode || strings.Contains(rec.Body.String(), "SECRET") || strings.Contains(rec.Body.String(), "10.0.0.1") {
			t.Errorf("%v: status=%d body=%s, want %d %s", c.err, rec.Code, rec.Body.String(), c.wantStatus, c.wantCode)
		}
	}
}

func TestRouter_MountsRequestRoutesOnlyWithBothClients(t *testing.T) {
	auth := newTestAuth(t)
	build := func(c *fakeRequestRoutesClient) http.Handler {
		d := Deps{
			Logger: slog.Default(), Registry: domain.NewDefaultServiceRegistry(), AuthValidator: usecase.NewAuthValidator(auth.jwks),
			RateLimiter:     usecase.NewRateLimiter(1000, 1000),
			CookieValidator: &fakeCookieValidator{identity: wscompatIdentity("tenant-1", "user-1")},
		}
		if c != nil {
			d.RequestClient, d.ApprovalClient = c, c
		}
		return NewRouter(d)
	}
	get := func(h http.Handler) int {
		req := httptest.NewRequest(http.MethodGet, "/v1/requests/r1", nil)
		req.AddCookie(&http.Cookie{Name: "orca_session", Value: "s"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := get(build(&fakeRequestRoutesClient{})); got != http.StatusOK {
		t.Errorf("with clients: %d, want 200", got)
	}
	if got := get(build(nil)); got == http.StatusOK {
		t.Errorf("without clients the route must not serve (got %d)", got)
	}
}

func wscompatIdentity(tenant, user string) wscompat.Identity {
	return wscompat.Identity{TenantID: tenant, UserID: user, Role: "user"}
}
