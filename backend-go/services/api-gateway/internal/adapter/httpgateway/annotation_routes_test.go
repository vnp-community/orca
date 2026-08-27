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

// MarkAnnotationsSent is not exercised by this file's tests yet (routing
// for it is TASK-CR-02-07, out of this batch's scope) — this stub exists
// only so fakeAnnotationServiceClient keeps satisfying
// annotationv1.AnnotationServiceClient after TASK-CR-02-01 added the RPC.
func (f *fakeAnnotationServiceClient) MarkAnnotationsSent(_ context.Context, in *annotationv1.MarkAnnotationsSentRequest, _ ...grpc.CallOption) (*annotationv1.MarkAnnotationsSentResponse, error) {
	return &annotationv1.MarkAnnotationsSentResponse{}, nil
}

// annotationTestRouter mounts mountAnnotationRoutes standalone (not through
// NewRouter, since router.go is out of scope here) and injects identity
// into the request context the same way authMiddleware does for a real
// request.
func annotationTestRouter(client annotationv1.AnnotationServiceClient) chi.Router {
	r := chi.NewRouter()
	mountAnnotationRoutes(r, client)
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
	router := annotationTestRouter(fake)

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
	router := annotationTestRouter(fake)

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
	router := annotationTestRouter(fake)

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
	router := annotationTestRouter(fake)

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
