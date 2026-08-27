package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// TaskExecutionChecker implements usecase.TaskExecutionChecker by dialing
// task-service's real HasActiveExecutions RPC — Epic C
// (docs/execution-plan.md §10, 2026-08-17) closed the gap this was
// previously a stub for.
//
// Known, honestly-documented limit inherited from task-service itself (see
// that service's README): task-service has no execution-completion
// callback, so "active" there means "status=in_progress", which is
// currently one-way — nothing transitions a task back out of in_progress
// yet. This checker will therefore over-report "active" (fail RebindDevServer
// closed) for any project with a task ever dispatched, until task-service
// gains a completion/status-update path. That's a real, tracked limitation
// of the underlying data, not a bug in this client.
type TaskExecutionChecker struct {
	conn   *grpc.ClientConn
	client taskv1.TaskServiceClient
}

// NewTaskExecutionChecker dials task-service at addr. The connection is lazy
// (grpc.NewClient doesn't block on connect), so a task-service that isn't up
// yet doesn't fail startup here.
func NewTaskExecutionChecker(addr string) (*TaskExecutionChecker, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial task-service at %q: %w", addr, err)
	}
	return &TaskExecutionChecker{conn: conn, client: taskv1.NewTaskServiceClient(conn)}, nil
}

func (c *TaskExecutionChecker) Close() error {
	return c.conn.Close()
}

// HasActiveExecutions calls task-service's real HasActiveExecutions RPC —
// see this type's doc comment for the one-way in_progress caveat the
// answer is subject to today.
func (c *TaskExecutionChecker) HasActiveExecutions(ctx context.Context, projectID string) (bool, error) {
	resp, err := c.client.HasActiveExecutions(ctx, &taskv1.HasActiveExecutionsRequest{ProjectId: projectID})
	if err != nil {
		return false, fmt.Errorf("grpcclient: task-service HasActiveExecutions: %w", err)
	}
	return resp.GetHasActive(), nil
}
