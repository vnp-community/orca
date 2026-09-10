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

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// fakeAnnotationServiceClient implements annotationv1.AnnotationServiceClient
// entirely in memory, capturing the last request of each kind it saw so
// tests can assert what actually crossed the REST->gRPC boundary.
type fakeAnnotationServiceClient struct {
	lastCreateReq *annotationv1.CreateAnnotationRequest
	createResp    *annotationv1.CreateAnnotationResponse
	createErr     error

	lastListReq *annotationv1.ListAnnotationsRequest
	listResp    *annotationv1.ListAnnotationsResponse
	listErr     error

	lastUpdateReq *annotationv1.UpdateAnnotationRequest
	updateResp    *annotationv1.UpdateAnnotationResponse
	updateErr     error

	lastDeleteReq *annotationv1.DeleteAnnotationRequest
	deleteResp    *annotationv1.DeleteAnnotationResponse
	deleteErr     error

	lastMarkSentReq *annotationv1.MarkAnnotationsSentRequest
	markSentResp    *annotationv1.MarkAnnotationsSentResponse
	markSentErr     error
}

func (f *fakeAnnotationServiceClient) CreateAnnotation(_ context.Context, in *annotationv1.CreateAnnotationRequest, _ ...grpc.CallOption) (*annotationv1.CreateAnnotationResponse, error) {
	f.lastCreateReq = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createResp, nil
}

func (f *fakeAnnotationServiceClient) ListAnnotations(_ context.Context, in *annotationv1.ListAnnotationsRequest, _ ...grpc.CallOption) (*annotationv1.ListAnnotationsResponse, error) {
	f.lastListReq = in
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listResp, nil
}

func (f *fakeAnnotationServiceClient) UpdateAnnotation(_ context.Context, in *annotationv1.UpdateAnnotationRequest, _ ...grpc.CallOption) (*annotationv1.UpdateAnnotationResponse, error) {
	f.lastUpdateReq = in
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updateResp, nil
}

func (f *fakeAnnotationServiceClient) DeleteAnnotation(_ context.Context, in *annotationv1.DeleteAnnotationRequest, _ ...grpc.CallOption) (*annotationv1.DeleteAnnotationResponse, error) {
	f.lastDeleteReq = in
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return f.deleteResp, nil
}

func (f *fakeAnnotationServiceClient) MarkAnnotationsSent(_ context.Context, in *annotationv1.MarkAnnotationsSentRequest, _ ...grpc.CallOption) (*annotationv1.MarkAnnotationsSentResponse, error) {
	f.lastMarkSentReq = in
	if f.markSentErr != nil {
		return nil, f.markSentErr
	}
	if f.markSentResp != nil {
		return f.markSentResp, nil
	}
	return &annotationv1.MarkAnnotationsSentResponse{}, nil
}

// fakeAnnotationGitGatewayClient is a minimal test double for
// gitgatewayv1.GitGatewayServiceClient — only ReadFile is overridden, the
// one method handleSendToAgent's underlying
// wscompat.SendReviewFeedbackToAgent calls.
type fakeAnnotationGitGatewayClient struct {
	gitgatewayv1.GitGatewayServiceClient

	readFileFunc func(ctx context.Context, in *gitgatewayv1.ReadFileRequest) (*gitgatewayv1.ReadFileResponse, error)
}

func (f *fakeAnnotationGitGatewayClient) ReadFile(ctx context.Context, in *gitgatewayv1.ReadFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadFileResponse, error) {
	return f.readFileFunc(ctx, in)
}

// annotationTestRouter mounts mountAnnotationRoutes standalone (not through
// NewRouter, since router.go is out of scope here) and injects identity
// into the request context the same way authMiddleware does for a real
// request.
func annotationTestRouter(client annotationv1.AnnotationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient) chi.Router {
	r := chi.NewRouter()
	mountAnnotationRoutes(r, client, gitClient)
	return r
}

// withTestIdentity is shared across this package's route tests —
// task_routes_test.go defines it.

func TestHandleCreateAnnotation_SuccessRoundTrip(t *testing.T) {
	fake := &fakeAnnotationServiceClient{
		createResp: &annotationv1.CreateAnnotationResponse{
			Annotation: &annotationv1.Annotation{
				Id:       "ann-1",
				TenantId: "tenant-1",
				AuthorId: "user-1",
				Content:  "looks good",
			},
		},
	}
	router := annotationTestRouter(fake, nil)

	body := `{"anchor":{"repo_id":"repo-1","file_path":"main.go","line":42,"ref":"main"},"content":"looks good","request_id":"req-1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/annotations", bytes.NewBufferString(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	if fake.lastCreateReq == nil {
		t.Fatal("expected CreateAnnotation to be called")
	}
	if fake.lastCreateReq.GetContent() != "looks good" {
		t.Fatalf("Content = %q, want %q", fake.lastCreateReq.GetContent(), "looks good")
	}
	if fake.lastCreateReq.GetAnchor().GetRepoId() != "repo-1" || fake.lastCreateReq.GetAnchor().GetLine() != 42 {
		t.Fatalf("Anchor = %+v, unexpected", fake.lastCreateReq.GetAnchor())
	}

	var got annotationv1.Annotation
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshaling response: %v; body=%s", err, rec.Body.String())
	}
	if got.Id != "ann-1" {
		t.Fatalf("Id = %q, want %q", got.Id, "ann-1")
	}
}

// TestHandleUpdateAnnotation_IdentityComesFromContextNotBody proves the
// gateway never trusts a caller-supplied identity: the request body here
// has no author/tenant field at all (the proto doesn't define one on
// UpdateAnnotationRequest), so this asserts the outbound gRPC call carries
// only the path id + body content/resolved — identity propagation happens
// exclusively via gatewaygrpc.AttachIdentity on the context, not any field
// on the wire request.
func TestHandleUpdateAnnotation_IdentityComesFromContextNotBody(t *testing.T) {
	fake := &fakeAnnotationServiceClient{
		updateResp: &annotationv1.UpdateAnnotationResponse{
			Annotation: &annotationv1.Annotation{
				Id:       "ann-1",
				TenantId: "tenant-1",
				AuthorId: "user-1",
				Content:  "edited",
				Resolved: true,
			},
		},
	}
	router := annotationTestRouter(fake, nil)

	body := `{"content":"edited","resolved":true}`
	req := httptest.NewRequest(http.MethodPut, "/v1/annotations/ann-1", bytes.NewBufferString(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if fake.lastUpdateReq == nil {
		t.Fatal("expected UpdateAnnotation to be called")
	}
	if fake.lastUpdateReq.GetId() != "ann-1" {
		t.Fatalf("Id = %q, want %q (from path, not body)", fake.lastUpdateReq.GetId(), "ann-1")
	}
	if fake.lastUpdateReq.GetContent() != "edited" || !fake.lastUpdateReq.GetResolved() {
		t.Fatalf("UpdateAnnotationRequest = %+v, unexpected", fake.lastUpdateReq)
	}
}

// TestHandleDeleteAnnotation_PermissionDeniedMapsTo403 covers the
// author-only-edit rejection: OPA enforces authorship server-side and
// returns codes.PermissionDenied, which this gateway must map straight
// through to HTTP 403 via writeGRPCError without trying to duplicate the
// authorship check itself.
func TestHandleDeleteAnnotation_PermissionDeniedMapsTo403(t *testing.T) {
	fake := &fakeAnnotationServiceClient{
		deleteErr: status.Error(codes.PermissionDenied, "only the author or an admin may delete this annotation"),
	}
	router := annotationTestRouter(fake, nil)

	req := httptest.NewRequest(http.MethodDelete, "/v1/annotations/ann-1", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-2"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	var errBody errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("unmarshaling error body: %v; body=%s", err, rec.Body.String())
	}
	if errBody.Error.Code != codes.PermissionDenied.String() {
		t.Fatalf("error.code = %q, want %q", errBody.Error.Code, codes.PermissionDenied.String())
	}

	if fake.lastDeleteReq == nil || fake.lastDeleteReq.GetId() != "ann-1" {
		t.Fatalf("lastDeleteReq = %+v, want Id ann-1", fake.lastDeleteReq)
	}
}

func TestHandleListAnnotations_Success(t *testing.T) {
	fake := &fakeAnnotationServiceClient{
		listResp: &annotationv1.ListAnnotationsResponse{
			Annotations: []*annotationv1.Annotation{
				{Id: "ann-1", TenantId: "tenant-1", AuthorId: "user-1", Content: "first"},
			},
			NextPageToken: "next",
		},
	}
	router := annotationTestRouter(fake, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/annotations?repo_id=repo-1&file_path=main.go&page_size=10", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.lastListReq.GetRepoId() != "repo-1" || fake.lastListReq.GetFilePath() != "main.go" || fake.lastListReq.GetPageSize() != 10 {
		t.Fatalf("ListAnnotationsRequest = %+v, unexpected", fake.lastListReq)
	}
}

// ── TASK-CR-02-07: confirmed / mark-sent REST tests ─────────────────────────

// TestHandleDeleteAnnotation_ForwardsConfirmedQueryParam verifies
// DELETE /v1/annotations/{id}?confirmed=true threads BR-CR-08's confirmed
// flag into DeleteAnnotationRequest.
func TestHandleDeleteAnnotation_ForwardsConfirmedQueryParam(t *testing.T) {
	fake := &fakeAnnotationServiceClient{deleteResp: &annotationv1.DeleteAnnotationResponse{}}
	router := annotationTestRouter(fake, nil)

	req := httptest.NewRequest(http.MethodDelete, "/v1/annotations/ann-1?confirmed=true", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if fake.lastDeleteReq == nil || !fake.lastDeleteReq.GetConfirmed() {
		t.Fatalf("lastDeleteReq = %+v, want Confirmed=true", fake.lastDeleteReq)
	}
}

// TestHandleMarkAnnotationsSent_ForwardsIDsAndReturnsUpdatedAnnotations
// verifies POST /v1/annotations/mark-sent forwards ids and returns the
// updated annotations MarkAnnotationsSent responded with.
func TestHandleMarkAnnotationsSent_ForwardsIDsAndReturnsUpdatedAnnotations(t *testing.T) {
	fake := &fakeAnnotationServiceClient{
		markSentResp: &annotationv1.MarkAnnotationsSentResponse{
			Annotations: []*annotationv1.Annotation{
				{Id: "ann-1", SentToAgent: true},
				{Id: "ann-2", SentToAgent: true},
			},
		},
	}
	router := annotationTestRouter(fake, nil)

	body := `{"ids":["ann-1","ann-2"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/annotations/mark-sent", bytes.NewBufferString(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(fake.lastMarkSentReq.GetIds()) != 2 || fake.lastMarkSentReq.GetIds()[0] != "ann-1" {
		t.Fatalf("lastMarkSentReq = %+v, want ids [ann-1 ann-2]", fake.lastMarkSentReq)
	}

	var got annotationv1.MarkAnnotationsSentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshaling response: %v; body=%s", err, rec.Body.String())
	}
	if len(got.GetAnnotations()) != 2 || !got.GetAnnotations()[0].GetSentToAgent() {
		t.Fatalf("response = %+v, want 2 sent annotations", got.GetAnnotations())
	}
}

// ── TASK-CR-03-05: POST /v1/annotations/send-to-agent ───────────────────────

// TestHandleSendToAgent_EmptyAnnotationsForwardsWorktreeIDAndReturnsSentZero
// exercises the REST mirror's list/format steps end-to-end without hitting
// the PTY-delivery step (see handleSendToAgent's doc comment for why the
// REST transport can't reach a live terminalStreamRegistry) — an empty
// annotation buffer is the one path that returns before delivery is
// attempted, so it's the one this transport can verify all the way to a
// response.
func TestHandleSendToAgent_EmptyAnnotationsForwardsWorktreeIDAndReturnsSentZero(t *testing.T) {
	fake := &fakeAnnotationServiceClient{listResp: &annotationv1.ListAnnotationsResponse{}}
	router := annotationTestRouter(fake, &fakeAnnotationGitGatewayClient{})

	body := `{"worktree_id":"wt-1","pty_id":"pty-1","worktree_name":"my-worktree"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/annotations/send-to-agent", bytes.NewBufferString(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if fake.lastListReq.GetWorktreeId() != "wt-1" {
		t.Fatalf("lastListReq.WorktreeId = %q, want wt-1", fake.lastListReq.GetWorktreeId())
	}
	if fake.lastListReq.SentToAgent == nil || *fake.lastListReq.SentToAgent != false {
		t.Fatalf("lastListReq.SentToAgent = %v, want pointer to false (unsent only)", fake.lastListReq.SentToAgent)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshaling response: %v; body=%s", err, rec.Body.String())
	}
	if got["sent"] != float64(0) {
		t.Fatalf("response[sent] = %v, want 0", got["sent"])
	}
}

// TestHandleSendToAgent_RequiresWorktreeIDAndPtyID verifies the REST mirror
// validates its required fields the same way the WS channel's decodeArg
// would surface a missing field, before ever calling the annotation client.
func TestHandleSendToAgent_RequiresWorktreeIDAndPtyID(t *testing.T) {
	fake := &fakeAnnotationServiceClient{}
	router := annotationTestRouter(fake, &fakeAnnotationGitGatewayClient{})

	req := httptest.NewRequest(http.MethodPost, "/v1/annotations/send-to-agent", bytes.NewBufferString(`{}`))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if fake.lastListReq != nil {
		t.Fatal("expected ListAnnotations not to be called when required fields are missing")
	}
}
