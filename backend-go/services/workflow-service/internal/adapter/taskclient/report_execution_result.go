// Package taskclient implements workflow-service's outbound TaskClient port
// (BE-SOL-002/TASK-FT-002-05) against task-service's real
// ReportTaskExecutionResult RPC (TASK-FT-002-04) — same convention as
// internal/adapter/infrafleetclient's existing dial pattern in this
// service.
package taskclient

import (
	"context"
	"fmt"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// Client implements usecase.TaskClient.
type Client struct {
	task taskv1.TaskServiceClient
}

func New(task taskv1.TaskServiceClient) *Client {
	return &Client{task: task}
}

func (c *Client) ReportTaskExecutionResult(ctx context.Context, in usecase.ReportTaskExecutionResultInput) error {
	_, err := c.task.ReportTaskExecutionResult(ctx, &taskv1.ReportTaskExecutionResultRequest{
		TaskId: in.TaskID, ExecutionRef: in.ExecutionRef, Success: in.Success,
		ActualHours: in.ActualHours, ErrorMessage: in.ErrorMessage, Engine: in.Engine,
	})
	if err != nil {
		return fmt.Errorf("taskclient: report execution result: %w", err)
	}
	return nil
}
