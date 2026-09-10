package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// fakeWorkflowServiceClient implements workflowv1.WorkflowServiceClient
// directly — same fake-the-generated-client-port convention as
// fakeInfraFleetServiceClient (simple_executor_test.go).
type fakeWorkflowServiceClient struct {
	workflowv1.WorkflowServiceClient // embed: panics on any unimplemented method, intentional for these tests

	executeResp *workflowv1.ExecuteResponse
	executeErr  error
	got         *workflowv1.ExecuteRequest
}

func (f *fakeWorkflowServiceClient) Execute(_ context.Context, in *workflowv1.ExecuteRequest, _ ...grpc.CallOption) (*workflowv1.ExecuteResponse, error) {
	f.got = in
	if f.executeErr != nil {
		return nil, f.executeErr
	}
	return f.executeResp, nil
}

func TestWorkflowExecutor_ForwardsTemplateIDAndOriginTaskID(t *testing.T) {
	client := &fakeWorkflowServiceClient{executeResp: &workflowv1.ExecuteResponse{
		Execution: &workflowv1.WorkflowExecution{Id: "wf-exec-1"},
	}}
	w := NewWorkflowExecutor(client)

	ref, err := w.Execute(context.Background(), "tenant-1", "task-1", "req-1", "wf-tmpl-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "wf-exec-1" {
		t.Errorf("expected the workflow execution id to be returned, got %q", ref)
	}
	if client.got.GetTemplateId() != "wf-tmpl-1" {
		t.Errorf("expected template_id=wf-tmpl-1, got %q", client.got.GetTemplateId())
	}
	if client.got.GetOriginTaskId() != "task-1" {
		t.Errorf("expected origin_task_id=task-1, got %q", client.got.GetOriginTaskId())
	}
	if client.got.GetRequestId() != "req-1" {
		t.Errorf("expected request_id=req-1, got %q", client.got.GetRequestId())
	}
}

func TestWorkflowExecutor_ExecuteFailurePropagates(t *testing.T) {
	client := &fakeWorkflowServiceClient{executeErr: errors.New("workflow-service unavailable")}
	w := NewWorkflowExecutor(client)

	if _, err := w.Execute(context.Background(), "tenant-1", "task-1", "req-1", "wf-tmpl-1"); err == nil {
		t.Fatal("expected the workflow-service failure to propagate")
	}
}
