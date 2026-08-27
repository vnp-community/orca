package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// fakeAutomationServiceClient follows fakeInfraFleetClient's
// embed-and-override pattern (channels_test.go): embeds the (nil)
// interface so it satisfies every method, overriding only what this
// file's tests actually call.
type fakeAutomationServiceClient struct {
	automationv1.AutomationServiceClient

	createAutomationFunc func(ctx context.Context, in *automationv1.CreateAutomationRequest) (*automationv1.CreateAutomationResponse, error)
	listRunsFunc         func(ctx context.Context, in *automationv1.ListRunsRequest) (*automationv1.ListRunsResponse, error)
	listAutomationsFunc  func(ctx context.Context, in *automationv1.ListAutomationsRequest) (*automationv1.ListAutomationsResponse, error)
	updateAutomationFunc func(ctx context.Context, in *automationv1.UpdateAutomationRequest) (*automationv1.UpdateAutomationResponse, error)
	deleteAutomationFunc func(ctx context.Context, in *automationv1.DeleteAutomationRequest) (*emptypb.Empty, error)
}

func (f *fakeAutomationServiceClient) CreateAutomation(ctx context.Context, in *automationv1.CreateAutomationRequest, _ ...grpc.CallOption) (*automationv1.CreateAutomationResponse, error) {
	return f.createAutomationFunc(ctx, in)
}

func (f *fakeAutomationServiceClient) ListRuns(ctx context.Context, in *automationv1.ListRunsRequest, _ ...grpc.CallOption) (*automationv1.ListRunsResponse, error) {
	return f.listRunsFunc(ctx, in)
}

func (f *fakeAutomationServiceClient) ListAutomations(ctx context.Context, in *automationv1.ListAutomationsRequest, _ ...grpc.CallOption) (*automationv1.ListAutomationsResponse, error) {
	return f.listAutomationsFunc(ctx, in)
}

func (f *fakeAutomationServiceClient) UpdateAutomation(ctx context.Context, in *automationv1.UpdateAutomationRequest, _ ...grpc.CallOption) (*automationv1.UpdateAutomationResponse, error) {
	return f.updateAutomationFunc(ctx, in)
}

func (f *fakeAutomationServiceClient) DeleteAutomation(ctx context.Context, in *automationv1.DeleteAutomationRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteAutomationFunc(ctx, in)
}

func TestAutomationCreateChannel_Success(t *testing.T) {
	var gotReq *automationv1.CreateAutomationRequest
	fake := &fakeAutomationServiceClient{
		createAutomationFunc: func(ctx context.Context, in *automationv1.CreateAutomationRequest) (*automationv1.CreateAutomationResponse, error) {
			gotReq = in
			return &automationv1.CreateAutomationResponse{Automation: &automationv1.Automation{Id: "a1", Name: in.Name}}, nil
		},
	}
	r := NewRegistry()
	registerAutomationCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "automation.create",
		argsJSON(t, map[string]any{"name": "nightly-build", "rrule": "FREQ=DAILY", "stepConfigJson": "{}", "stepType": "shell"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.TenantId != "tenant-1" {
		t.Errorf("expected TenantId from Identity, got %q", gotReq.TenantId)
	}
	if gotReq.StepType != workflowv1.StepType_STEP_TYPE_SHELL {
		t.Errorf("expected StepType shell, got %v", gotReq.StepType)
	}
	automation, ok := result.(*automationv1.Automation)
	if !ok || automation.GetId() != "a1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestAutomationListChannel_Success(t *testing.T) {
	fake := &fakeAutomationServiceClient{
		listAutomationsFunc: func(ctx context.Context, in *automationv1.ListAutomationsRequest) (*automationv1.ListAutomationsResponse, error) {
			return &automationv1.ListAutomationsResponse{Automations: []*automationv1.Automation{{Id: "a1"}, {Id: "a2"}}}, nil
		},
	}
	r := NewRegistry()
	registerAutomationCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "automation.list", argsJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*automationv1.ListAutomationsResponse)
	if !ok || len(resp.GetAutomations()) != 2 {
		t.Errorf("unexpected result: %+v", result)
	}
}

// TestAutomationUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues is the
// regression guard SOL-033/TASK-221 calls for: only fields the caller
// actually sent must reach the wrapper-typed request as non-nil, so an
// unrelated partial edit (e.g. toggling "enabled") never clobbers other
// fields with a zero-value overwrite.
func TestAutomationUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues(t *testing.T) {
	var gotReq *automationv1.UpdateAutomationRequest
	fake := &fakeAutomationServiceClient{
		updateAutomationFunc: func(ctx context.Context, in *automationv1.UpdateAutomationRequest) (*automationv1.UpdateAutomationResponse, error) {
			gotReq = in
			return &automationv1.UpdateAutomationResponse{Automation: &automationv1.Automation{Id: in.GetId()}}, nil
		},
	}
	r := NewRegistry()
	registerAutomationCRUDChannels(r, fake)

	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "automation.update",
		argsJSON(t, map[string]any{"id": "a1", "enabled": true})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetName() != nil {
		t.Errorf("expected Name to remain a nil wrapper value, got %v", gotReq.GetName())
	}
	if gotReq.GetRrule() != nil {
		t.Errorf("expected Rrule to remain a nil wrapper value, got %v", gotReq.GetRrule())
	}
	if gotReq.GetEnabled() == nil || !gotReq.GetEnabled().GetValue() {
		t.Errorf("expected Enabled=true wrapper value, got %v", gotReq.GetEnabled())
	}
}

func TestAutomationDeleteChannel_Success(t *testing.T) {
	var gotReq *automationv1.DeleteAutomationRequest
	fake := &fakeAutomationServiceClient{
		deleteAutomationFunc: func(ctx context.Context, in *automationv1.DeleteAutomationRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerAutomationCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "automation.delete", argsJSON(t, map[string]any{"id": "a1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Id != "a1" || gotReq.TenantId != "tenant-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	success, ok := result.(map[string]bool)
	if !ok || !success["success"] {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestAutomationRunsChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("automation-service unavailable")
	fake := &fakeAutomationServiceClient{
		listRunsFunc: func(ctx context.Context, in *automationv1.ListRunsRequest) (*automationv1.ListRunsResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerAutomationCRUDChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "automation.runs", argsJSON(t, map[string]any{"automationId": "a1"}))
	if !errors.Is(err, wantErr) {
		t.Fatalf("want error %v, got %v", wantErr, err)
	}
}

// ── task.* fakes/tests ────────────────────────────────────────────────

type fakeTaskServiceClient struct {
	taskv1.TaskServiceClient

	createTaskFunc          func(ctx context.Context, in *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error)
	getTaskFunc             func(ctx context.Context, in *taskv1.GetTaskRequest) (*taskv1.GetTaskResponse, error)
	executeFunc             func(ctx context.Context, in *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error)
	listTasksFunc           func(ctx context.Context, in *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error)
	updateTaskFunc          func(ctx context.Context, in *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error)
	deleteTaskFunc          func(ctx context.Context, in *taskv1.DeleteTaskRequest) (*emptypb.Empty, error)
	getDependenciesFunc     func(ctx context.Context, in *taskv1.GetDependenciesRequest) (*taskv1.GetDependenciesResponse, error)
	aiDecomposeFunc         func(ctx context.Context, in *taskv1.AIDecomposeRequest) (*taskv1.AIDecomposeResponse, error)
	aiApplyFunc             func(ctx context.Context, in *taskv1.AIApplyRequest) (*taskv1.AIApplyResponse, error)
	hasActiveExecutionsFunc func(ctx context.Context, in *taskv1.HasActiveExecutionsRequest) (*taskv1.HasActiveExecutionsResponse, error)

	lastHasActiveExecutionsRequest *taskv1.HasActiveExecutionsRequest
}

func (f *fakeTaskServiceClient) CreateTask(ctx context.Context, in *taskv1.CreateTaskRequest, _ ...grpc.CallOption) (*taskv1.CreateTaskResponse, error) {
	return f.createTaskFunc(ctx, in)
}

func (f *fakeTaskServiceClient) GetTask(ctx context.Context, in *taskv1.GetTaskRequest, _ ...grpc.CallOption) (*taskv1.GetTaskResponse, error) {
	return f.getTaskFunc(ctx, in)
}

func (f *fakeTaskServiceClient) Execute(ctx context.Context, in *taskv1.TaskServiceExecuteRequest, _ ...grpc.CallOption) (*taskv1.TaskServiceExecuteResponse, error) {
	return f.executeFunc(ctx, in)
}

func (f *fakeTaskServiceClient) ListTasks(ctx context.Context, in *taskv1.ListTasksRequest, _ ...grpc.CallOption) (*taskv1.ListTasksResponse, error) {
	return f.listTasksFunc(ctx, in)
}

func (f *fakeTaskServiceClient) UpdateTask(ctx context.Context, in *taskv1.UpdateTaskRequest, _ ...grpc.CallOption) (*taskv1.UpdateTaskResponse, error) {
	return f.updateTaskFunc(ctx, in)
}

func (f *fakeTaskServiceClient) DeleteTask(ctx context.Context, in *taskv1.DeleteTaskRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteTaskFunc(ctx, in)
}

func (f *fakeTaskServiceClient) GetDependencies(ctx context.Context, in *taskv1.GetDependenciesRequest, _ ...grpc.CallOption) (*taskv1.GetDependenciesResponse, error) {
	return f.getDependenciesFunc(ctx, in)
}

func (f *fakeTaskServiceClient) AIDecompose(ctx context.Context, in *taskv1.AIDecomposeRequest, _ ...grpc.CallOption) (*taskv1.AIDecomposeResponse, error) {
	return f.aiDecomposeFunc(ctx, in)
}

func (f *fakeTaskServiceClient) AIApply(ctx context.Context, in *taskv1.AIApplyRequest, _ ...grpc.CallOption) (*taskv1.AIApplyResponse, error) {
	return f.aiApplyFunc(ctx, in)
}

func (f *fakeTaskServiceClient) HasActiveExecutions(ctx context.Context, in *taskv1.HasActiveExecutionsRequest, _ ...grpc.CallOption) (*taskv1.HasActiveExecutionsResponse, error) {
	f.lastHasActiveExecutionsRequest = in
	if f.hasActiveExecutionsFunc != nil {
		return f.hasActiveExecutionsFunc(ctx, in)
	}
	return &taskv1.HasActiveExecutionsResponse{HasActive: false}, nil
}

// TestTaskCreateGetChannels_StillRegistered guards the "keep, don't
// remove" decision (TASK-222) against a future contributor treating
// BUG-034's dead-code finding as license to delete these two channels —
// registered by channels.go's registerTaskChannels, not this file, but
// this file's tests are where the rest of task.* coverage lives.
func TestTaskCreateGetChannels_StillRegistered(t *testing.T) {
	r := NewRegistry()
	registerTaskChannels(r, &fakeTaskServiceClient{
		createTaskFunc: func(ctx context.Context, in *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
			return &taskv1.CreateTaskResponse{Task: &taskv1.Task{Id: "t1"}}, nil
		},
		getTaskFunc: func(ctx context.Context, in *taskv1.GetTaskRequest) (*taskv1.GetTaskResponse, error) {
			return &taskv1.GetTaskResponse{Task: &taskv1.Task{Id: in.GetId()}}, nil
		},
	})

	if _, err := r.Dispatch(context.Background(), Identity{}, "task.create", argsJSON(t, map[string]any{"title": "x"})); err != nil {
		t.Errorf("expected task.create to remain registered: %v", err)
	}
	if _, err := r.Dispatch(context.Background(), Identity{}, "task.get", argsJSON(t, map[string]any{"id": "t1"})); err != nil {
		t.Errorf("expected task.get to remain registered: %v", err)
	}
}

// TestRegisterTaskChannels_HasActiveExecutions guards the response
// envelope key (hasActiveExecutions, not the wire field name has_active)
// and that projectId is forwarded to the outbound request.
func TestRegisterTaskChannels_HasActiveExecutions(t *testing.T) {
	fake := &fakeTaskServiceClient{}
	r := NewRegistry()
	registerTaskChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"},
		"task.hasActiveExecutions", argsJSON(t, map[string]any{"projectId": "p1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := result.(map[string]bool)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if got["hasActiveExecutions"] != false {
		t.Errorf("unexpected result: %+v", got)
	}
	if fake.lastHasActiveExecutionsRequest.GetProjectId() != "p1" {
		t.Errorf("want projectId p1 forwarded, got %q", fake.lastHasActiveExecutionsRequest.GetProjectId())
	}
}

func TestTaskExecuteChannel_Success(t *testing.T) {
	var gotReq *taskv1.TaskServiceExecuteRequest
	fake := &fakeTaskServiceClient{
		executeFunc: func(ctx context.Context, in *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error) {
			gotReq = in
			return &taskv1.TaskServiceExecuteResponse{ExecutionRef: "exec-1"}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.execute", argsJSON(t, map[string]any{"taskId": "t1", "requestId": "req-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.TaskId != "t1" || gotReq.RequestId != "req-1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	resp, ok := result.(*taskv1.TaskServiceExecuteResponse)
	if !ok || resp.GetExecutionRef() != "exec-1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskListChannel_Success(t *testing.T) {
	fake := &fakeTaskServiceClient{
		listTasksFunc: func(ctx context.Context, in *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
			return &taskv1.ListTasksResponse{Tasks: []*taskv1.Task{{Id: "t1"}, {Id: "t2"}}}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "task.list", argsJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, ok := result.(*taskv1.ListTasksResponse)
	if !ok || len(resp.GetTasks()) != 2 {
		t.Errorf("unexpected result: %+v", result)
	}
}

// TestTaskUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues is task.update's
// analog of TestAutomationUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues
// — only a status-only edit should reach UpdateTaskRequest, title must stay
// a nil wrapper value.
func TestTaskUpdateChannel_LeavesUnsetFieldsAsNilWrapperValues(t *testing.T) {
	var gotReq *taskv1.UpdateTaskRequest
	fake := &fakeTaskServiceClient{
		updateTaskFunc: func(ctx context.Context, in *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
			gotReq = in
			return &taskv1.UpdateTaskResponse{Task: &taskv1.Task{Id: in.GetId()}}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	if _, err := r.Dispatch(context.Background(), Identity{}, "task.update", argsJSON(t, map[string]any{"id": "t1", "status": "done"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTitle() != nil {
		t.Errorf("expected Title to remain a nil wrapper value, got %v", gotReq.GetTitle())
	}
	if gotReq.GetStatus() == nil || gotReq.GetStatus().GetValue() != "done" {
		t.Errorf("expected Status=done wrapper value, got %v", gotReq.GetStatus())
	}
}

func TestTaskDeleteChannel_Success(t *testing.T) {
	var gotReq *taskv1.DeleteTaskRequest
	fake := &fakeTaskServiceClient{
		deleteTaskFunc: func(ctx context.Context, in *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{}, "task.delete", argsJSON(t, map[string]any{"id": "t1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Id != "t1" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	success, ok := result.(map[string]bool)
	if !ok || !success["success"] {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskGetDependenciesChannel_Success(t *testing.T) {
	fake := &fakeTaskServiceClient{
		getDependenciesFunc: func(ctx context.Context, in *taskv1.GetDependenciesRequest) (*taskv1.GetDependenciesResponse, error) {
			return &taskv1.GetDependenciesResponse{Dependencies: []*taskv1.Task{{Id: "dep1"}}}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{}, "task.getDependencies", argsJSON(t, map[string]any{"taskId": "t1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deps, ok := result.([]*taskv1.Task)
	if !ok || len(deps) != 1 || deps[0].GetId() != "dep1" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskAIDecomposeChannel_Success(t *testing.T) {
	fake := &fakeTaskServiceClient{
		aiDecomposeFunc: func(ctx context.Context, in *taskv1.AIDecomposeRequest) (*taskv1.AIDecomposeResponse, error) {
			return &taskv1.AIDecomposeResponse{Proposals: []*taskv1.SubtaskProposal{{Title: "Design API"}}}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{}, "task.aiDecompose", argsJSON(t, map[string]any{"taskId": "t1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	proposals, ok := result.([]*taskv1.SubtaskProposal)
	if !ok || len(proposals) != 1 || proposals[0].GetTitle() != "Design API" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestTaskAIApplyChannel_Success(t *testing.T) {
	var gotReq *taskv1.AIApplyRequest
	fake := &fakeTaskServiceClient{
		aiApplyFunc: func(ctx context.Context, in *taskv1.AIApplyRequest) (*taskv1.AIApplyResponse, error) {
			gotReq = in
			return &taskv1.AIApplyResponse{CreatedSubtasks: []*taskv1.Task{{Id: "sub1"}}}, nil
		},
	}
	r := NewRegistry()
	registerTaskCRUDChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{}, "task.aiApply",
		argsJSON(t, map[string]any{"taskId": "t1", "proposals": []map[string]any{{"title": "Design API", "description": "d"}}}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.TaskId != "t1" || len(gotReq.Proposals) != 1 || gotReq.Proposals[0].GetTitle() != "Design API" {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	created, ok := result.([]*taskv1.Task)
	if !ok || len(created) != 1 || created[0].GetId() != "sub1" {
		t.Errorf("unexpected result: %+v", result)
	}
}
