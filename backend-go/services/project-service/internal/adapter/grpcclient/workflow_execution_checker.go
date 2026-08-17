// Package grpcclient holds project-service's outbound gRPC client adapters
// toward other services — see project-service.md §7 ("Calls" table:
// tenant-service, infra-fleet-service, workflow-service, task-service).
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WorkflowExecutionChecker implements usecase.WorkflowExecutionChecker by
// dialing workflow-service.
//
// STUB — workflow-service doesn't yet expose a HasActiveExecutions RPC; this
// must be replaced with a real gRPC client call once that RPC exists. Do not
// deploy this stub to production — see project-service.md's RebindDevServer
// section (§3) for the saga this port participates in. The ClientConn is
// dialed for real (so config/wiring doesn't have to change later) but
// HasActiveExecutions doesn't use it yet — it always reports "no active
// executions", which means RebindDevServer's guard is currently a no-op for
// the workflow-service half of the check.
type WorkflowExecutionChecker struct {
	conn *grpc.ClientConn
}

// NewWorkflowExecutionChecker dials workflow-service at addr. The connection
// is lazy (grpc.NewClient doesn't block on connect), so a workflow-service
// that isn't up yet doesn't fail startup here.
func NewWorkflowExecutionChecker(addr string) (*WorkflowExecutionChecker, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial workflow-service at %q: %w", addr, err)
	}
	return &WorkflowExecutionChecker{conn: conn}, nil
}

func (c *WorkflowExecutionChecker) Close() error {
	return c.conn.Close()
}

// HasActiveExecutions always returns (false, nil) — see the STUB warning on
// WorkflowExecutionChecker. Replace this body with the real unary RPC call
// once workflow-service defines HasActiveExecutions(projectId) returns (bool).
func (c *WorkflowExecutionChecker) HasActiveExecutions(ctx context.Context, projectID string) (bool, error) {
	_ = c.conn // reserved for the real RPC call once workflow-service exposes it
	return false, nil
}
