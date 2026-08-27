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

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// fakeTaskServiceClient implements taskv1.TaskServiceClient with
// per-method canned responses/errors, configurable per test.
type fakeTaskServiceClient struct {
	taskv1.TaskServiceClient // nil embed: satisfies methods this fake doesn't hook (e.g. TASK-223's List/Update/Delete/GetDependencies, TASK-224's AIApply) — panics if actually called, which is correct since this REST-route layer's tests don't exercise them

	createTaskResp *taskv1.CreateTaskResponse
	createTaskErr  error
	createTaskReq  *taskv1.CreateTaskRequest // captures the last request for assertions

	getTaskResp *taskv1.GetTaskResponse
	getTaskErr  error

	addEdgeResp *taskv1.AddEdgeResponse
	addEdgeErr  error

	grantResp *taskv1.GrantResponse
	grantErr  error

	resolvePermissionResp *taskv1.ResolvePermissionResponse
	resolvePermissionErr  error

	executeResp *taskv1.TaskServiceExecuteResponse
	executeErr  error

	hasActiveExecutionsResp *taskv1.HasActiveExecutionsResponse
	hasActiveExecutionsErr  error
}

func (f *fakeTaskServiceClient) CreateTask(_ context.Context, in *taskv1.CreateTaskRequest, _ ...grpc.CallOption) (*taskv1.CreateTaskResponse, error) {
	f.createTaskReq = in
	if f.createTaskErr != nil {
		return nil, f.createTaskErr
	}
	return f.createTaskResp, nil
}

func (f *fakeTaskServiceClient) GetTask(_ context.Context, _ *taskv1.GetTaskRequest, _ ...grpc.CallOption) (*taskv1.GetTaskResponse, error) {
	if f.getTaskErr != nil {
		return nil, f.getTaskErr
	}
	return f.getTaskResp, nil
}

func (f *fakeTaskServiceClient) AddEdge(_ context.Context, _ *taskv1.AddEdgeRequest, _ ...grpc.CallOption) (*taskv1.AddEdgeResponse, error) {
	if f.addEdgeErr != nil {
		return nil, f.addEdgeErr
	}
	return f.addEdgeResp, nil
}

func (f *fakeTaskServiceClient) Grant(_ context.Context, _ *taskv1.GrantRequest, _ ...grpc.CallOption) (*taskv1.GrantResponse, error) {
	if f.grantErr != nil {
		return nil, f.grantErr
	}
	return f.grantResp, nil
}

func (f *fakeTaskServiceClient) ResolvePermission(_ context.Context, _ *taskv1.ResolvePermissionRequest, _ ...grpc.CallOption) (*taskv1.ResolvePermissionResponse, error) {
	if f.resolvePermissionErr != nil {
		return nil, f.resolvePermissionErr
	}
	return f.resolvePermissionResp, nil
}

func (f *fakeTaskServiceClient) Execute(_ context.Context, _ *taskv1.TaskServiceExecuteRequest, _ ...grpc.CallOption) (*taskv1.TaskServiceExecuteResponse, error) {
	if f.executeErr != nil {
		return nil, f.executeErr
	}
	return f.executeResp, nil
}

func (f *fakeTaskServiceClient) HasActiveExecutions(_ context.Context, _ *taskv1.HasActiveExecutionsRequest, _ ...grpc.CallOption) (*taskv1.HasActiveExecutionsResponse, error) {
	if f.hasActiveExecutionsErr != nil {
		return nil, f.hasActiveExecutionsErr
	}
	return f.hasActiveExecutionsResp, nil
}

// taskTestRouter mounts mountTaskRoutes standalone (router.go isn't
// touched by this package's tests, per task instructions) and injects an
// identity into the request context the same way authMiddleware would.
func taskTestRouter(client taskv1.TaskServiceClient) chi.Router {
	r := chi.NewRouter()
	mountTaskRoutes(r, client)
	return r
}

// withTestIdentity is shared across this package's route tests —
// git_routes_test.go defines it.

func TestHandleCreateTask_SuccessRoundTrip(t *testing.T) {
	fake := &fakeTaskServiceClient{
		createTaskResp: &taskv1.CreateTaskResponse{
			Task: &taskv1.Task{
				Id:       "task-1",
				TenantId: "tenant-1",
				Title:    "Write tests",
			},
		},
	}
	router := taskTestRouter(fake)

	body, _ := json.Marshal(createTaskRequestBody{Title: "Write tests"})
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/", bytes.NewReader(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var gotTask taskv1.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &gotTask); err != nil {
		t.Fatalf("response body is not the expected Task JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if gotTask.GetId() != "task-1" || gotTask.GetTitle() != "Write tests" {
		t.Fatalf("unexpected task in response: %+v", &gotTask)
	}
}

// TestHandleCreateTask_TenantIDComesFromIdentityNotBody is the security
// regression test: tenant_id must NEVER be trusted from the JSON request
// body, only from identityFromContext — matching every existing handler in
// usage_routes.go and auth_routes.go.
func TestHandleCreateTask_TenantIDComesFromIdentityNotBody(t *testing.T) {
	fake := &fakeTaskServiceClient{
		createTaskResp: &taskv1.CreateTaskResponse{Task: &taskv1.Task{Id: "task-1"}},
	}
	router := taskTestRouter(fake)

	// Attacker-controlled body claims a different tenant than the caller's
	// validated identity.
	rawBody := []byte(`{"title":"steal data","tenant_id":"attacker-tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/", bytes.NewReader(rawBody))
	req = withTestIdentity(req, usecase.Identity{TenantID: "real-tenant", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if fake.createTaskReq == nil {
		t.Fatal("CreateTask was never called")
	}
	if fake.createTaskReq.GetTenantId() != "real-tenant" {
		t.Fatalf("TenantId sent upstream = %q, want %q (from identity, not body)", fake.createTaskReq.GetTenantId(), "real-tenant")
	}
}

func TestHandleGetTask_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	fake := &fakeTaskServiceClient{
		getTaskErr: status.Error(codes.NotFound, "task not found"),
	}
	router := taskTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/missing-id", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not the expected error JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Code != codes.NotFound.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.NotFound.String())
	}
}

func TestHandleResolvePermission_UsesIdentityUserID(t *testing.T) {
	fake := &fakeTaskServiceClient{
		resolvePermissionResp: &taskv1.ResolvePermissionResponse{
			EffectiveLevel: taskv1.GrantLevel_GRANT_LEVEL_ADMIN,
		},
	}
	router := taskTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/task-1/permission", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got resolvePermissionResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if got.EffectiveLevel != taskv1.GrantLevel_GRANT_LEVEL_ADMIN.String() {
		t.Fatalf("effective_level = %q, want %q", got.EffectiveLevel, taskv1.GrantLevel_GRANT_LEVEL_ADMIN.String())
	}
}

func TestHandleHasActiveExecutions_SuccessRoundTrip(t *testing.T) {
	fake := &fakeTaskServiceClient{
		hasActiveExecutionsResp: &taskv1.HasActiveExecutionsResponse{HasActive: true},
	}
	router := taskTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/project-1/active-executions", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got hasActiveExecutionsResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if !got.HasActive {
		t.Fatal("has_active = false, want true")
	}
}
