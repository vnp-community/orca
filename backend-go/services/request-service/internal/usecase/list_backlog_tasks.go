package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type BacklogGroup struct {
	RequestID string
	Tasks     []domain.TaskView
}

type TaskClient interface {
	ListTasks(ctx context.Context, tenantID string, requestIDs []string, types []string) ([]domain.TaskView, error)
	ListExecutionStates(ctx context.Context, tenantID string, taskIDs []string) (map[string]ExecutionStateView, error)
}

type ExecutionStateView struct {
	LastEngine      string
	LastLinkStatus  string
	FailedAttempts  int
	BlockedByTaskIDs []string
	LastError       string
}

type ListBacklogTasksInput struct {
	ProjectID    string
	RequestTypes []string
	RequestID    string
	PageToken    string
	PageSize     int
}

type ListBacklogTasks struct {
	ReqReader  BacklogRequestReader
	AppGate    ApprovalGateReader
	TaskClient TaskClient
}

func (uc *ListBacklogTasks) Execute(ctx context.Context, in ListBacklogTasksInput) ([]BacklogGroup, string, error) {
	// Stub implementation
	return nil, "", nil
}
