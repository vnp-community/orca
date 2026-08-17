package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TaskExecutionChecker implements usecase.TaskExecutionChecker by dialing
// task-service.
//
// STUB — task-service doesn't yet expose a HasActiveExecutions RPC; this
// must be replaced with a real gRPC client call once that RPC exists. Do not
// deploy this stub to production — see project-service.md's RebindDevServer
// section (§3) for the saga this port participates in. The ClientConn is
// dialed for real (so config/wiring doesn't have to change later) but
// HasActiveExecutions doesn't use it yet — it always reports "no active
// executions", which means RebindDevServer's guard is currently a no-op for
// the task-service half of the check.
type TaskExecutionChecker struct {
	conn *grpc.ClientConn
}

// NewTaskExecutionChecker dials task-service at addr. The connection is lazy
// (grpc.NewClient doesn't block on connect), so a task-service that isn't up
// yet doesn't fail startup here.
func NewTaskExecutionChecker(addr string) (*TaskExecutionChecker, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial task-service at %q: %w", addr, err)
	}
	return &TaskExecutionChecker{conn: conn}, nil
}

func (c *TaskExecutionChecker) Close() error {
	return c.conn.Close()
}

// HasActiveExecutions always returns (false, nil) — see the STUB warning on
// TaskExecutionChecker. Replace this body with the real unary RPC call once
// task-service defines HasActiveExecutions(projectId) returns (bool).
func (c *TaskExecutionChecker) HasActiveExecutions(ctx context.Context, projectID string) (bool, error) {
	_ = c.conn // reserved for the real RPC call once task-service exposes it
	return false, nil
}
