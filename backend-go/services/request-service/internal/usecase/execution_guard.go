package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TaskExecutionGuard refuses return and cancel while a working task of the Request is running.
// A task-service failure is an error, never "no": returning a Request whose tasks might still be running
// would let agents keep working on something the user believes stopped.
type TaskExecutionGuard struct {
	Tasks TaskClient
}

var _ ExecutionGuard = (*TaskExecutionGuard)(nil)

func (g *TaskExecutionGuard) HasActiveExecution(ctx context.Context, requestID string) (bool, error) {
	tasks, err := g.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{requestID}, TaskTypes: []string{"task", "bug", "feature"}})
	if err != nil {
		return false, domain.ErrExecutionTaskServiceUnavailable(err)
	}
	for _, t := range tasks {
		if t.Status == domain.TaskStatusInProgress {
			return true, nil
		}
	}
	return false, nil
}
