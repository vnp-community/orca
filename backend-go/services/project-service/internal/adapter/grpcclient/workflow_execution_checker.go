// Package grpcclient holds project-service's outbound gRPC client adapters
// toward other services — see project-service.md §7 ("Calls" table:
// tenant-service, infra-fleet-service, workflow-service, task-service).
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// WorkflowExecutionChecker implements usecase.WorkflowExecutionChecker by
// dialing workflow-service's real HasActiveExecutions RPC — Epic C
// (docs/execution-plan.md §10, 2026-08-17) closed the gap this was
// previously a stub for. See project-service.md §3 for the saga
// RebindDevServer runs this port as part of.
type WorkflowExecutionChecker struct {
	conn   *grpc.ClientConn
	client workflowv1.WorkflowServiceClient
}

// NewWorkflowExecutionChecker dials workflow-service at addr. The connection
// is lazy (grpc.NewClient doesn't block on connect), so a workflow-service
// that isn't up yet doesn't fail startup here.
func NewWorkflowExecutionChecker(addr string) (*WorkflowExecutionChecker, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial workflow-service at %q: %w", addr, err)
	}
	return &WorkflowExecutionChecker{conn: conn, client: workflowv1.NewWorkflowServiceClient(conn)}, nil
}

func (c *WorkflowExecutionChecker) Close() error {
	return c.conn.Close()
}

// HasActiveExecutions calls workflow-service's real HasActiveExecutions RPC
// — see that RPC's proto doc comment: true iff projectID has a workflow
// execution in a non-terminal (pending/running/paused) status.
func (c *WorkflowExecutionChecker) HasActiveExecutions(ctx context.Context, projectID string) (bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return false, fmt.Errorf("grpcclient: workflow-service HasActiveExecutions: %w", err)
	}
	resp, err := c.client.HasActiveExecutions(ctx, &workflowv1.HasActiveExecutionsRequest{ProjectId: projectID})
	if err != nil {
		return false, fmt.Errorf("grpcclient: workflow-service HasActiveExecutions: %w", err)
	}
	return resp.GetHasActive(), nil
}
