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

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// fakeWorkflowServiceClient implements workflowv1.WorkflowServiceClient with
// per-method canned responses/errors, configurable per test.
type fakeWorkflowServiceClient struct {
	createTemplateResp *workflowv1.CreateTemplateResponse
	createTemplateErr  error
	createTemplateReq  *workflowv1.CreateTemplateRequest // captures the last request for assertions

	executeResp *workflowv1.ExecuteResponse
	executeErr  error

	getExecutionResp *workflowv1.GetExecutionResponse
	getExecutionErr  error

	pauseExecutionResp *workflowv1.PauseExecutionResponse
	pauseExecutionErr  error

	resumeExecutionResp *workflowv1.ResumeExecutionResponse
	resumeExecutionErr  error

	executeAdHocStepResp *workflowv1.ExecuteAdHocStepResponse
	executeAdHocStepErr  error

	cancelExecutionResp *workflowv1.CancelExecutionResponse
	cancelExecutionErr  error

	listTemplatesResp *workflowv1.ListTemplatesResponse
	listTemplatesErr  error

	resolveTemplateResp *workflowv1.ResolveTemplateResponse
	resolveTemplateErr  error

	hasActiveExecutionsResp *workflowv1.HasActiveExecutionsResponse
	hasActiveExecutionsErr  error
}

func (f *fakeWorkflowServiceClient) CreateTemplate(_ context.Context, in *workflowv1.CreateTemplateRequest, _ ...grpc.CallOption) (*workflowv1.CreateTemplateResponse, error) {
	f.createTemplateReq = in
	if f.createTemplateErr != nil {
		return nil, f.createTemplateErr
	}
	return f.createTemplateResp, nil
}

// UpdateTemplate: no route in this file's tests exercises it, so this exists
// only to satisfy the workflowv1.WorkflowServiceClient interface this fake
// must implement in full.
func (f *fakeWorkflowServiceClient) UpdateTemplate(_ context.Context, _ *workflowv1.UpdateTemplateRequest, _ ...grpc.CallOption) (*workflowv1.UpdateTemplateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) Execute(_ context.Context, _ *workflowv1.ExecuteRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteResponse, error) {
	if f.executeErr != nil {
		return nil, f.executeErr
	}
	return f.executeResp, nil
}

func (f *fakeWorkflowServiceClient) GetExecution(_ context.Context, _ *workflowv1.GetExecutionRequest, _ ...grpc.CallOption) (*workflowv1.GetExecutionResponse, error) {
	if f.getExecutionErr != nil {
		return nil, f.getExecutionErr
	}
	return f.getExecutionResp, nil
}

func (f *fakeWorkflowServiceClient) PauseExecution(_ context.Context, _ *workflowv1.PauseExecutionRequest, _ ...grpc.CallOption) (*workflowv1.PauseExecutionResponse, error) {
	if f.pauseExecutionErr != nil {
		return nil, f.pauseExecutionErr
	}
	return f.pauseExecutionResp, nil
}

func (f *fakeWorkflowServiceClient) ResumeExecution(_ context.Context, _ *workflowv1.ResumeExecutionRequest, _ ...grpc.CallOption) (*workflowv1.ResumeExecutionResponse, error) {
	if f.resumeExecutionErr != nil {
		return nil, f.resumeExecutionErr
	}
	return f.resumeExecutionResp, nil
}

func (f *fakeWorkflowServiceClient) ExecuteAdHocStep(_ context.Context, _ *workflowv1.ExecuteAdHocStepRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error) {
	if f.executeAdHocStepErr != nil {
		return nil, f.executeAdHocStepErr
	}
	return f.executeAdHocStepResp, nil
}

func (f *fakeWorkflowServiceClient) CancelExecution(_ context.Context, _ *workflowv1.CancelExecutionRequest, _ ...grpc.CallOption) (*workflowv1.CancelExecutionResponse, error) {
	if f.cancelExecutionErr != nil {
		return nil, f.cancelExecutionErr
	}
	return f.cancelExecutionResp, nil
}

func (f *fakeWorkflowServiceClient) ListTemplates(_ context.Context, _ *workflowv1.ListTemplatesRequest, _ ...grpc.CallOption) (*workflowv1.ListTemplatesResponse, error) {
	if f.listTemplatesErr != nil {
		return nil, f.listTemplatesErr
	}
	return f.listTemplatesResp, nil
}

func (f *fakeWorkflowServiceClient) ResolveTemplate(_ context.Context, _ *workflowv1.ResolveTemplateRequest, _ ...grpc.CallOption) (*workflowv1.ResolveTemplateResponse, error) {
	if f.resolveTemplateErr != nil {
		return nil, f.resolveTemplateErr
	}
	return f.resolveTemplateResp, nil
}

func (f *fakeWorkflowServiceClient) HasActiveExecutions(_ context.Context, _ *workflowv1.HasActiveExecutionsRequest, _ ...grpc.CallOption) (*workflowv1.HasActiveExecutionsResponse, error) {
	if f.hasActiveExecutionsErr != nil {
		return nil, f.hasActiveExecutionsErr
	}
	return f.hasActiveExecutionsResp, nil
}

// CloneTemplate/PublishTemplate/ListPendingApprovals/ResolveApproval/
// GenerateShareLink/PreviewSharedTemplate/ImportSharedTemplate/
// RateTemplate/StreamExecutionEvents (TASK-WF-01-03/02-01/03-03): no route
// in this file's tests exercises any of these yet, so each exists only to
// satisfy the workflowv1.WorkflowServiceClient interface this fake must
// implement in full — same convention as UpdateTemplate above.
func (f *fakeWorkflowServiceClient) CloneTemplate(_ context.Context, _ *workflowv1.CloneTemplateRequest, _ ...grpc.CallOption) (*workflowv1.CloneTemplateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) PublishTemplate(_ context.Context, _ *workflowv1.PublishTemplateRequest, _ ...grpc.CallOption) (*workflowv1.WorkflowTemplate, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) ListPendingApprovals(_ context.Context, _ *workflowv1.ListPendingApprovalsRequest, _ ...grpc.CallOption) (*workflowv1.ListPendingApprovalsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) ResolveApproval(_ context.Context, _ *workflowv1.ResolveApprovalRequest, _ ...grpc.CallOption) (*workflowv1.Approval, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) GenerateShareLink(_ context.Context, _ *workflowv1.GenerateShareLinkRequest, _ ...grpc.CallOption) (*workflowv1.GenerateShareLinkResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) PreviewSharedTemplate(_ context.Context, _ *workflowv1.PreviewSharedTemplateRequest, _ ...grpc.CallOption) (*workflowv1.SharedTemplatePreview, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) ImportSharedTemplate(_ context.Context, _ *workflowv1.ImportSharedTemplateRequest, _ ...grpc.CallOption) (*workflowv1.WorkflowTemplate, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) RateTemplate(_ context.Context, _ *workflowv1.RateTemplateRequest, _ ...grpc.CallOption) (*workflowv1.RateTemplateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

func (f *fakeWorkflowServiceClient) StreamExecutionEvents(_ context.Context, _ *workflowv1.StreamExecutionEventsRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[workflowv1.ExecutionEvent], error) {
	return nil, status.Error(codes.Unimplemented, "not used by workflow_routes_test.go")
}

// workflowTestRouter mounts mountWorkflowRoutes standalone (router.go isn't
// touched by this package's tests, per task instructions).
func workflowTestRouter(client workflowv1.WorkflowServiceClient) chi.Router {
	r := chi.NewRouter()
	mountWorkflowRoutes(r, client)
	return r
}

// withTestIdentity is shared across this package's route tests —
// git_routes_test.go defines it.

func TestHandleCreateTemplate_SuccessRoundTrip(t *testing.T) {
	fake := &fakeWorkflowServiceClient{
		createTemplateResp: &workflowv1.CreateTemplateResponse{
			Template: &workflowv1.WorkflowTemplate{
				Id:       "tmpl-1",
				TenantId: "tenant-1",
				Name:     "Release Flow",
			},
		},
	}
	router := workflowTestRouter(fake)

	body, _ := json.Marshal(createTemplateRequestBody{Name: "Release Flow", DagJSON: `{"steps":[]}`})
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/templates", bytes.NewReader(body))
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var gotTemplate workflowv1.WorkflowTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &gotTemplate); err != nil {
		t.Fatalf("response body is not the expected WorkflowTemplate JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if gotTemplate.GetId() != "tmpl-1" || gotTemplate.GetName() != "Release Flow" {
		t.Fatalf("unexpected template in response: %+v", &gotTemplate)
	}
}

// TestHandleCreateTemplate_TenantIDComesFromIdentityNotBody is the security
// regression test: tenant_id must NEVER be trusted from the JSON request
// body, only from identityFromContext — matching every existing handler in
// usage_routes.go and task_routes_test.go.
func TestHandleCreateTemplate_TenantIDComesFromIdentityNotBody(t *testing.T) {
	fake := &fakeWorkflowServiceClient{
		createTemplateResp: &workflowv1.CreateTemplateResponse{Template: &workflowv1.WorkflowTemplate{Id: "tmpl-1"}},
	}
	router := workflowTestRouter(fake)

	// Attacker-controlled body claims a different tenant than the caller's
	// validated identity.
	rawBody := []byte(`{"name":"steal data","tenant_id":"attacker-tenant"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows/templates", bytes.NewReader(rawBody))
	req = withTestIdentity(req, usecase.Identity{TenantID: "real-tenant", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if fake.createTemplateReq == nil {
		t.Fatal("CreateTemplate was never called")
	}
	if fake.createTemplateReq.GetTenantId() != "real-tenant" {
		t.Fatalf("TenantId sent upstream = %q, want %q (from identity, not body)", fake.createTemplateReq.GetTenantId(), "real-tenant")
	}
}

func TestHandleGetExecution_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	fake := &fakeWorkflowServiceClient{
		getExecutionErr: status.Error(codes.NotFound, "execution not found"),
	}
	router := workflowTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/workflows/executions/missing-id", nil)
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

func TestHandleWorkflowHasActiveExecutions_SuccessRoundTrip(t *testing.T) {
	fake := &fakeWorkflowServiceClient{
		hasActiveExecutionsResp: &workflowv1.HasActiveExecutionsResponse{HasActive: true},
	}
	router := workflowTestRouter(fake)

	req := httptest.NewRequest(http.MethodGet, "/v1/workflows/project-1/active-executions", nil)
	req = withTestIdentity(req, usecase.Identity{TenantID: "tenant-1", UserID: "user-1"})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workflowHasActiveExecutionsResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
	}
	if !got.HasActive {
		t.Fatal("has_active = false, want true")
	}
}
