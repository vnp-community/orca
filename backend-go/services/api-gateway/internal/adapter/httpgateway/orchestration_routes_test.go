package httpgateway

import (
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

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

// fakeOrchestrationServiceClient implements
// orchestrationv1.OrchestrationServiceClient entirely in-memory,
// configurable to return either a canned response or a gRPC status error
// per method, and records the last request/context it saw so tests can
// assert on what mountOrchestrationRoutes actually sent upstream.
type fakeOrchestrationServiceClient struct {
	orchestrationv1.OrchestrationServiceClient

	createDispatchContextResp    *orchestrationv1.CreateDispatchContextResponse
	createDispatchContextErr     error
	lastCreateDispatchContextReq *orchestrationv1.CreateDispatchContextRequest

	createGateResp    *orchestrationv1.CreateGateResponse
	createGateErr     error
	lastCreateGateReq *orchestrationv1.CreateGateRequest

	resolveGateResp    *orchestrationv1.ResolveGateResponse
	resolveGateErr     error
	lastResolveGateReq *orchestrationv1.ResolveGateRequest

	updateTaskStatusResp    *orchestrationv1.UpdateTaskStatusAndPromoteResponse
	updateTaskStatusErr     error
	lastUpdateTaskStatusReq *orchestrationv1.UpdateTaskStatusAndPromoteRequest

	lastCtx context.Context
}

func (f *fakeOrchestrationServiceClient) CreateDispatchContext(ctx context.Context, in *orchestrationv1.CreateDispatchContextRequest, _ ...grpc.CallOption) (*orchestrationv1.CreateDispatchContextResponse, error) {
	f.lastCtx = ctx
	f.lastCreateDispatchContextReq = in
	if f.createDispatchContextErr != nil {
		return nil, f.createDispatchContextErr
	}
	return f.createDispatchContextResp, nil
}

func (f *fakeOrchestrationServiceClient) CreateGate(ctx context.Context, in *orchestrationv1.CreateGateRequest, _ ...grpc.CallOption) (*orchestrationv1.CreateGateResponse, error) {
	f.lastCtx = ctx
	f.lastCreateGateReq = in
	if f.createGateErr != nil {
		return nil, f.createGateErr
	}
	return f.createGateResp, nil
}

func (f *fakeOrchestrationServiceClient) ResolveGate(ctx context.Context, in *orchestrationv1.ResolveGateRequest, _ ...grpc.CallOption) (*orchestrationv1.ResolveGateResponse, error) {
	f.lastCtx = ctx
	f.lastResolveGateReq = in
	if f.resolveGateErr != nil {
		return nil, f.resolveGateErr
	}
	return f.resolveGateResp, nil
}

func (f *fakeOrchestrationServiceClient) UpdateTaskStatusAndPromote(ctx context.Context, in *orchestrationv1.UpdateTaskStatusAndPromoteRequest, _ ...grpc.CallOption) (*orchestrationv1.UpdateTaskStatusAndPromoteResponse, error) {
	f.lastCtx = ctx
	f.lastUpdateTaskStatusReq = in
	if f.updateTaskStatusErr != nil {
		return nil, f.updateTaskStatusErr
	}
	return f.updateTaskStatusResp, nil
}

// testOrchestrationRouter mounts mountOrchestrationRoutes alone on a fresh
// chi router — no auth middleware — since these tests inject identity
// directly into the request context, the same way authMiddleware would
// have (see requestWithIdentity in automation_routes_test.go).
func testOrchestrationRouter(client orchestrationv1.OrchestrationServiceClient) http.Handler {
	r := chi.NewRouter()
	mountOrchestrationRoutes(r, client)
	return r
}

func TestHandleCreateDispatchContext_SuccessRoundTrip(t *testing.T) {
	client := &fakeOrchestrationServiceClient{
		createDispatchContextResp: &orchestrationv1.CreateDispatchContextResponse{
			Context: &orchestrationv1.DispatchContext{
				Id:                  "dc-1",
				Handle:              "handle-1",
				CoordinatorRunId:    "run-1",
				OrchestrationTaskId: "task-1",
			},
		},
	}
	router := testOrchestrationRouter(client)

	body, _ := json.Marshal(createDispatchContextRequestBody{
		Handle:              "handle-1",
		CoordinatorRunID:    "run-1",
		OrchestrationTaskID: "task-1",
	})
	req := requestWithIdentity(http.MethodPost, "/v1/orchestration/dispatch-contexts", body, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got orchestrationv1.DispatchContext
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if got.Id != "dc-1" || got.Handle != "handle-1" {
		t.Fatalf("unexpected response: %+v", &got)
	}

	if client.lastCreateDispatchContextReq == nil {
		t.Fatal("expected CreateDispatchContext to be called")
	}
	if client.lastCreateDispatchContextReq.Handle != "handle-1" {
		t.Fatalf("Handle = %q, want %q", client.lastCreateDispatchContextReq.Handle, "handle-1")
	}
}

func TestHandleCreateGate_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	client := &fakeOrchestrationServiceClient{
		createGateErr: status.Error(codes.FailedPrecondition, "dispatch context has no task"),
	}
	router := testOrchestrationRouter(client)

	body, _ := json.Marshal(createGateRequestBody{
		DispatchContextID: "dc-404",
		Question:          "proceed?",
		Options:           []string{"yes", "no"},
	})
	req := requestWithIdentity(http.MethodPost, "/v1/orchestration/gates", body, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusPreconditionFailed, rec.Body.String())
	}

	var got errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal error body: %v; body=%s", err, rec.Body.String())
	}
	if got.Error.Code != codes.FailedPrecondition.String() {
		t.Fatalf("error.code = %q, want %q", got.Error.Code, codes.FailedPrecondition.String())
	}
	if got.Error.Message != "dispatch context has no task" {
		t.Fatalf("error.message = %q, want %q", got.Error.Message, "dispatch context has no task")
	}

	if client.lastCreateGateReq == nil || client.lastCreateGateReq.DispatchContextId != "dc-404" {
		t.Fatalf("expected CreateGate called with dispatch_context_id dc-404, got %+v", client.lastCreateGateReq)
	}
}

func TestHandleResolveGate_GateIDComesFromPath(t *testing.T) {
	client := &fakeOrchestrationServiceClient{
		resolveGateResp: &orchestrationv1.ResolveGateResponse{
			Gate: &orchestrationv1.DecisionGate{Id: "gate-1", Status: "resolved"},
		},
	}
	router := testOrchestrationRouter(client)

	rawBody := []byte(`{"outcome_json":"{\"choice\":\"yes\"}","gate_id":"attacker-supplied"}`)
	req := requestWithIdentity(http.MethodPost, "/v1/orchestration/gates/gate-1/resolve", rawBody, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if client.lastResolveGateReq == nil || client.lastResolveGateReq.GateId != "gate-1" {
		t.Fatalf("GateId must come from the URL path, got %+v", client.lastResolveGateReq)
	}
}

func TestHandleUpdateTaskStatusAndPromote_SuccessRoundTrip(t *testing.T) {
	client := &fakeOrchestrationServiceClient{
		updateTaskStatusResp: &orchestrationv1.UpdateTaskStatusAndPromoteResponse{
			PromotedTaskIds: []string{"task-2", "task-3"},
		},
	}
	router := testOrchestrationRouter(client)

	body, _ := json.Marshal(updateTaskStatusRequestBody{NewStatus: "completed"})
	req := requestWithIdentity(http.MethodPut, "/v1/orchestration/tasks/task-1/status", body, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if client.lastUpdateTaskStatusReq == nil || client.lastUpdateTaskStatusReq.OrchestrationTaskId != "task-1" || client.lastUpdateTaskStatusReq.NewStatus != "completed" {
		t.Fatalf("unexpected UpdateTaskStatusAndPromote request: %+v", client.lastUpdateTaskStatusReq)
	}

	var got orchestrationv1.UpdateTaskStatusAndPromoteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, rec.Body.String())
	}
	if len(got.PromotedTaskIds) != 2 {
		t.Fatalf("unexpected response: %+v", &got)
	}
}
