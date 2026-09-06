package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// fakeWorkflowServiceClient is a minimal test double for
// workflowv1.WorkflowServiceClient — embeds the (nil) interface, per
// fakeInfraFleetClient's precedent in channels_test.go, and overrides only
// the four methods this file's channel handlers actually call.
type fakeWorkflowServiceClient struct {
	workflowv1.WorkflowServiceClient

	executeFunc             func(ctx context.Context, in *workflowv1.ExecuteRequest) (*workflowv1.ExecuteResponse, error)
	cancelExecutionFunc     func(ctx context.Context, in *workflowv1.CancelExecutionRequest) (*workflowv1.CancelExecutionResponse, error)
	createTemplateFunc      func(ctx context.Context, in *workflowv1.CreateTemplateRequest) (*workflowv1.CreateTemplateResponse, error)
	updateTemplateFunc      func(ctx context.Context, in *workflowv1.UpdateTemplateRequest) (*workflowv1.UpdateTemplateResponse, error)
	getExecutionFunc        func(ctx context.Context, in *workflowv1.GetExecutionRequest) (*workflowv1.GetExecutionResponse, error)
	pauseExecutionFunc      func(ctx context.Context, in *workflowv1.PauseExecutionRequest) (*workflowv1.PauseExecutionResponse, error)
	resumeExecutionFunc     func(ctx context.Context, in *workflowv1.ResumeExecutionRequest) (*workflowv1.ResumeExecutionResponse, error)
	listTemplatesFunc       func(ctx context.Context, in *workflowv1.ListTemplatesRequest) (*workflowv1.ListTemplatesResponse, error)
	resolveTemplateFunc     func(ctx context.Context, in *workflowv1.ResolveTemplateRequest) (*workflowv1.ResolveTemplateResponse, error)
	hasActiveExecutionsFunc func(ctx context.Context, in *workflowv1.HasActiveExecutionsRequest) (*workflowv1.HasActiveExecutionsResponse, error)
	executeAdHocStepFunc    func(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest) (*workflowv1.ExecuteAdHocStepResponse, error)
}

func (f *fakeWorkflowServiceClient) Execute(ctx context.Context, in *workflowv1.ExecuteRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteResponse, error) {
	return f.executeFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) CancelExecution(ctx context.Context, in *workflowv1.CancelExecutionRequest, _ ...grpc.CallOption) (*workflowv1.CancelExecutionResponse, error) {
	return f.cancelExecutionFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) CreateTemplate(ctx context.Context, in *workflowv1.CreateTemplateRequest, _ ...grpc.CallOption) (*workflowv1.CreateTemplateResponse, error) {
	return f.createTemplateFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) UpdateTemplate(ctx context.Context, in *workflowv1.UpdateTemplateRequest, _ ...grpc.CallOption) (*workflowv1.UpdateTemplateResponse, error) {
	return f.updateTemplateFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) GetExecution(ctx context.Context, in *workflowv1.GetExecutionRequest, _ ...grpc.CallOption) (*workflowv1.GetExecutionResponse, error) {
	return f.getExecutionFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) PauseExecution(ctx context.Context, in *workflowv1.PauseExecutionRequest, _ ...grpc.CallOption) (*workflowv1.PauseExecutionResponse, error) {
	return f.pauseExecutionFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) ResumeExecution(ctx context.Context, in *workflowv1.ResumeExecutionRequest, _ ...grpc.CallOption) (*workflowv1.ResumeExecutionResponse, error) {
	return f.resumeExecutionFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) ListTemplates(ctx context.Context, in *workflowv1.ListTemplatesRequest, _ ...grpc.CallOption) (*workflowv1.ListTemplatesResponse, error) {
	return f.listTemplatesFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) ResolveTemplate(ctx context.Context, in *workflowv1.ResolveTemplateRequest, _ ...grpc.CallOption) (*workflowv1.ResolveTemplateResponse, error) {
	return f.resolveTemplateFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) HasActiveExecutions(ctx context.Context, in *workflowv1.HasActiveExecutionsRequest, _ ...grpc.CallOption) (*workflowv1.HasActiveExecutionsResponse, error) {
	return f.hasActiveExecutionsFunc(ctx, in)
}

func (f *fakeWorkflowServiceClient) ExecuteAdHocStep(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteAdHocStepResponse, error) {
	return f.executeAdHocStepFunc(ctx, in)
}

func TestWorkflowExecuteChannel_Success(t *testing.T) {
	var gotReq *workflowv1.ExecuteRequest
	var gotCtx context.Context
	fake := &fakeWorkflowServiceClient{
		executeFunc: func(ctx context.Context, in *workflowv1.ExecuteRequest) (*workflowv1.ExecuteResponse, error) {
			gotCtx = ctx
			gotReq = in
			return &workflowv1.ExecuteResponse{Execution: &workflowv1.WorkflowExecution{Id: "exec-1", TemplateId: in.TemplateId, Status: "running"}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"templateId":  "tmpl-1",
		"projectId":   "proj-1",
		"rootTraceId": "trace-1",
		"requestId":   "req-1",
	})

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.execute", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec, ok := result.(*workflowv1.WorkflowExecution)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if exec.Id != "exec-1" || exec.TemplateId != "tmpl-1" {
		t.Errorf("unexpected execution: %+v", exec)
	}
	if gotReq.ProjectId != "proj-1" || gotReq.RootTraceId != "trace-1" || gotReq.RequestId != "req-1" {
		t.Errorf("decoded args not forwarded to request: %+v", gotReq)
	}

	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

func TestWorkflowCancelChannel_Success(t *testing.T) {
	var gotReq *workflowv1.CancelExecutionRequest
	var gotCtx context.Context
	fake := &fakeWorkflowServiceClient{
		cancelExecutionFunc: func(ctx context.Context, in *workflowv1.CancelExecutionRequest) (*workflowv1.CancelExecutionResponse, error) {
			gotCtx = ctx
			gotReq = in
			return &workflowv1.CancelExecutionResponse{Execution: &workflowv1.WorkflowExecution{Id: in.Id, Status: "cancelled"}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"executionId": "exec-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.cancel", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec, ok := result.(*workflowv1.WorkflowExecution)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if exec.Status != "cancelled" {
		t.Errorf("unexpected execution: %+v", exec)
	}
	if gotReq.Id != "exec-1" {
		t.Errorf("want executionId forwarded as Id, got %q", gotReq.Id)
	}

	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

// TestWorkflowTemplateCreateChannel_TenantIDComesFromIdentityNotArgs is a
// regression guard: a malicious/buggy frontend payload setting a different
// tenantId in args must not leak into the outbound CreateTemplateRequest —
// TenantId must always come from the resolved Identity.
func TestWorkflowTemplateCreateChannel_TenantIDComesFromIdentityNotArgs(t *testing.T) {
	var gotReq *workflowv1.CreateTemplateRequest
	fake := &fakeWorkflowServiceClient{
		createTemplateFunc: func(ctx context.Context, in *workflowv1.CreateTemplateRequest) (*workflowv1.CreateTemplateResponse, error) {
			gotReq = in
			return &workflowv1.CreateTemplateResponse{Template: &workflowv1.WorkflowTemplate{Id: "tmpl-new", TenantId: in.TenantId, Name: in.Name}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"name":             "deploy",
		"dagJson":          `{"steps":[]}`,
		"scope":            "personal",
		"parentTemplateId": "",
		"tenantId":         "attacker-supplied-tenant",
	})

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-real", UserID: "user-1"}, "workflow.template.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReq.TenantId != "tenant-real" {
		t.Fatalf("want TenantId from Identity (tenant-real), got %q — args-supplied tenantId must never win", gotReq.TenantId)
	}

	tmpl, ok := result.(*workflowv1.WorkflowTemplate)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if tmpl.Id != "tmpl-new" {
		t.Errorf("unexpected template: %+v", tmpl)
	}
}

// TestWorkflowTemplateUpdateChannel_FailedPreconditionSurfacesUnmodified
// simulates the version-conflict path: a gRPC FAILED_PRECONDITION status
// from the client must surface as the handler's returned error unmodified,
// not swallowed or replaced with a generic message.
func TestWorkflowTemplateUpdateChannel_FailedPreconditionSurfacesUnmodified(t *testing.T) {
	wantErr := status.Error(codes.FailedPrecondition, "WORKFLOW_TEMPLATE_VERSION_CONFLICT: template was modified by another request")
	var gotReq *workflowv1.UpdateTemplateRequest
	fake := &fakeWorkflowServiceClient{
		updateTemplateFunc: func(ctx context.Context, in *workflowv1.UpdateTemplateRequest) (*workflowv1.UpdateTemplateResponse, error) {
			gotReq = in
			return nil, wantErr
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"id":               "tmpl-1",
		"name":             "deploy-v2",
		"dagJson":          `{"steps":[]}`,
		"scope":            "personal",
		"parentTemplateId": "",
		"expectedVersion":  1,
	})

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.template.update", args)
	if err == nil {
		t.Fatal("expected the FAILED_PRECONDITION error to surface")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("want an unmodified codes.FailedPrecondition status, got %v", err)
	}
	if st.Message() != status.Convert(wantErr).Message() {
		t.Fatalf("error message was altered: got %q, want %q", st.Message(), status.Convert(wantErr).Message())
	}
	if gotReq.ExpectedVersion != 1 {
		t.Errorf("want expectedVersion=1 forwarded, got %d", gotReq.ExpectedVersion)
	}
}

func TestWorkflowTemplateUpdateChannel_Success(t *testing.T) {
	var gotReq *workflowv1.UpdateTemplateRequest
	fake := &fakeWorkflowServiceClient{
		updateTemplateFunc: func(ctx context.Context, in *workflowv1.UpdateTemplateRequest) (*workflowv1.UpdateTemplateResponse, error) {
			gotReq = in
			return &workflowv1.UpdateTemplateResponse{Template: &workflowv1.WorkflowTemplate{Id: in.Id, Name: in.Name, Version: in.ExpectedVersion + 1}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"id":               "tmpl-1",
		"name":             "deploy-v2",
		"dagJson":          `{"steps":[]}`,
		"scope":            "team",
		"parentTemplateId": "tmpl-parent",
		"expectedVersion":  1,
	})

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.template.update", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tmpl, ok := result.(*workflowv1.WorkflowTemplate)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if tmpl.Version != 2 {
		t.Errorf("want bumped version 2, got %d", tmpl.Version)
	}
	if gotReq.ParentTemplateId != "tmpl-parent" || gotReq.Scope != "team" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
}

// --- CR-PW-005: the 7 previously-unregistered channels ---

func TestWorkflowGetExecutionChannel_Success(t *testing.T) {
	var gotReq *workflowv1.GetExecutionRequest
	fake := &fakeWorkflowServiceClient{
		getExecutionFunc: func(ctx context.Context, in *workflowv1.GetExecutionRequest) (*workflowv1.GetExecutionResponse, error) {
			gotReq = in
			return &workflowv1.GetExecutionResponse{Execution: &workflowv1.WorkflowExecution{Id: in.Id, Status: "running"}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"executionId": "exec-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.getExecution", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec, ok := result.(*workflowv1.WorkflowExecution)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if exec.Status != "running" {
		t.Errorf("unexpected execution: %+v", exec)
	}
	if gotReq.Id != "exec-1" {
		t.Errorf("want executionId forwarded as Id, got %q", gotReq.Id)
	}
}

func TestWorkflowPauseChannel_Success(t *testing.T) {
	var gotReq *workflowv1.PauseExecutionRequest
	fake := &fakeWorkflowServiceClient{
		pauseExecutionFunc: func(ctx context.Context, in *workflowv1.PauseExecutionRequest) (*workflowv1.PauseExecutionResponse, error) {
			gotReq = in
			return &workflowv1.PauseExecutionResponse{Execution: &workflowv1.WorkflowExecution{Id: in.Id, Status: "paused"}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"executionId": "exec-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.pause", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exec, ok := result.(*workflowv1.WorkflowExecution)
	if !ok || exec.Status != "paused" {
		t.Fatalf("unexpected result: %+v (type %T)", result, result)
	}
	if gotReq.Id != "exec-1" {
		t.Errorf("want executionId forwarded as Id, got %q", gotReq.Id)
	}
}

func TestWorkflowResumeChannel_Success(t *testing.T) {
	var gotReq *workflowv1.ResumeExecutionRequest
	fake := &fakeWorkflowServiceClient{
		resumeExecutionFunc: func(ctx context.Context, in *workflowv1.ResumeExecutionRequest) (*workflowv1.ResumeExecutionResponse, error) {
			gotReq = in
			return &workflowv1.ResumeExecutionResponse{Execution: &workflowv1.WorkflowExecution{Id: in.Id, Status: "running"}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"executionId": "exec-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.resume", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	exec, ok := result.(*workflowv1.WorkflowExecution)
	if !ok || exec.Status != "running" {
		t.Fatalf("unexpected result: %+v (type %T)", result, result)
	}
	if gotReq.Id != "exec-1" {
		t.Errorf("want executionId forwarded as Id, got %q", gotReq.Id)
	}
}

// TestWorkflowTemplateListChannel_EmptyReturnsEmptyArrayNotNull guards the
// established list-shaped-channel convention (TASK-BE-008's
// devServerGroup.list precedent): a nil proto slice must serialize as `[]`,
// never `null`, so frontend code doing `.map`/`.length` on it doesn't crash.
func TestWorkflowTemplateListChannel_EmptyReturnsEmptyArrayNotNull(t *testing.T) {
	fake := &fakeWorkflowServiceClient{
		listTemplatesFunc: func(ctx context.Context, in *workflowv1.ListTemplatesRequest) (*workflowv1.ListTemplatesResponse, error) {
			return &workflowv1.ListTemplatesResponse{Templates: nil, NextPageToken: ""}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"scope": "", "pageToken": "", "pageSize": 0})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.template.list", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	templates, ok := m["templates"].([]*workflowv1.WorkflowTemplate)
	if !ok || templates == nil || len(templates) != 0 {
		t.Fatalf("want non-nil empty slice, got %#v", m["templates"])
	}
}

func TestWorkflowTemplateListChannel_ForwardsScopeAndPagination(t *testing.T) {
	var gotReq *workflowv1.ListTemplatesRequest
	fake := &fakeWorkflowServiceClient{
		listTemplatesFunc: func(ctx context.Context, in *workflowv1.ListTemplatesRequest) (*workflowv1.ListTemplatesResponse, error) {
			gotReq = in
			return &workflowv1.ListTemplatesResponse{
				Templates:     []*workflowv1.WorkflowTemplate{{Id: "tmpl-1"}},
				NextPageToken: "next-1",
			}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"scope": "team", "pageToken": "tok-0", "pageSize": 20})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.template.list", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Scope != "team" || gotReq.PageToken != "tok-0" || gotReq.PageSize != 20 {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	m, ok := result.(map[string]any)
	if !ok || m["nextPageToken"] != "next-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestWorkflowTemplateResolveChannel_Success(t *testing.T) {
	var gotReq *workflowv1.ResolveTemplateRequest
	fake := &fakeWorkflowServiceClient{
		resolveTemplateFunc: func(ctx context.Context, in *workflowv1.ResolveTemplateRequest) (*workflowv1.ResolveTemplateResponse, error) {
			gotReq = in
			return &workflowv1.ResolveTemplateResponse{
				Template: &workflowv1.WorkflowTemplate{Id: "tmpl-leaf", DagJson: `{"steps":[]}`},
				Chain: []*workflowv1.WorkflowTemplate{
					{Id: "tmpl-root"}, {Id: "tmpl-leaf"},
				},
			}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"templateId": "tmpl-leaf"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.template.resolve", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.TemplateId != "tmpl-leaf" {
		t.Errorf("want templateId forwarded, got %q", gotReq.TemplateId)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	chain, ok := m["chain"].([]*workflowv1.WorkflowTemplate)
	if !ok || len(chain) != 2 {
		t.Fatalf("want 2-element chain, got %#v", m["chain"])
	}
}

func TestWorkflowHasActiveExecutionsChannel_Success(t *testing.T) {
	var gotReq *workflowv1.HasActiveExecutionsRequest
	fake := &fakeWorkflowServiceClient{
		hasActiveExecutionsFunc: func(ctx context.Context, in *workflowv1.HasActiveExecutionsRequest) (*workflowv1.HasActiveExecutionsResponse, error) {
			gotReq = in
			return &workflowv1.HasActiveExecutionsResponse{HasActive: true}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{"projectId": "proj-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "workflow.hasActiveExecutions", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.ProjectId != "proj-1" {
		t.Errorf("want projectId forwarded, got %q", gotReq.ProjectId)
	}
	m, ok := result.(map[string]any)
	if !ok || m["hasActive"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// TestWorkflowExecuteAdHocStepChannel_TenantIDComesFromIdentityNotArgs mirrors
// TestWorkflowTemplateCreateChannel_TenantIDComesFromIdentityNotArgs above —
// same regression class, different RPC (ExecuteAdHocStepRequest also carries
// a tenant_id field).
func TestWorkflowExecuteAdHocStepChannel_TenantIDComesFromIdentityNotArgs(t *testing.T) {
	var gotReq *workflowv1.ExecuteAdHocStepRequest
	fake := &fakeWorkflowServiceClient{
		executeAdHocStepFunc: func(ctx context.Context, in *workflowv1.ExecuteAdHocStepRequest) (*workflowv1.ExecuteAdHocStepResponse, error) {
			gotReq = in
			return &workflowv1.ExecuteAdHocStepResponse{Result: &workflowv1.StepResult{Status: "completed", OutputJson: "{}"}}, nil
		},
	}

	r := NewRegistry()
	registerWorkflowChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"stepType":       "shell",
		"stepConfigJson": `{"command":"echo hi"}`,
		"requestId":      "req-1",
	})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-real", UserID: "user-1"}, "workflow.executeAdHocStep", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.TenantId != "tenant-real" {
		t.Fatalf("want TenantId from Identity (tenant-real), got %q", gotReq.TenantId)
	}
	if gotReq.StepType != workflowv1.StepType_STEP_TYPE_SHELL {
		t.Errorf("want stepType SHELL parsed from %q, got %v", "shell", gotReq.StepType)
	}
	res, ok := result.(*workflowv1.StepResult)
	if !ok || res.Status != "completed" {
		t.Fatalf("unexpected result: %+v (type %T)", result, result)
	}
}
