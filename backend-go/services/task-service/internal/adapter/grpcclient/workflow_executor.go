package grpcclient

import (
	"context"
	"fmt"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// WorkflowExecutor implements usecase.WorkflowExecutor against
// workflow-service's real Execute RPC (BE-SOL-002/TASK-FT-002-03) — unlike
// ComplexExecutor before this change, workflow-service.Execute was already
// real code with no stub phase, so this client is wired for real from the
// start.
//
// Execute's project_id/root_trace_id are left unset — task-service has no
// ProjectID-to-workflow-inputs mapping designed yet, matching
// CR-FLOW-TASK-002's explicit "if input interpolation isn't implemented
// yet, empty inputs, don't block this CR" instruction.
type WorkflowExecutor struct {
	workflow workflowv1.WorkflowServiceClient
}

func NewWorkflowExecutor(workflow workflowv1.WorkflowServiceClient) *WorkflowExecutor {
	return &WorkflowExecutor{workflow: workflow}
}

func (w *WorkflowExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, workflowTemplateID string) (string, error) {
	resp, err := w.workflow.Execute(ctx, &workflowv1.ExecuteRequest{
		TemplateId:   workflowTemplateID,
		RequestId:    requestID,
		OriginTaskId: taskID,
	})
	if err != nil {
		return "", fmt.Errorf("workflow_executor: execute: %w", err)
	}
	return resp.GetExecution().GetId(), nil
}
