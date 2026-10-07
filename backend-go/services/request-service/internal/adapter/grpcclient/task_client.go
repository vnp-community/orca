package grpcclient

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type TaskClient struct {
	// GRPC connection stub
}

func NewTaskClient() *TaskClient {
	return &TaskClient{}
}

var _ usecase.TaskClient = (*TaskClient)(nil)

func (c *TaskClient) ListTasks(ctx context.Context, tenantID string, requestIDs []string, types []string) ([]domain.TaskView, error) {
	return nil, nil // Stub
}

func (c *TaskClient) ListExecutionStates(ctx context.Context, tenantID string, taskIDs []string) (map[string]usecase.ExecutionStateView, error) {
	// chia lô <= 500 id
	return nil, nil // Stub
}
