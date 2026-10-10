package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/secretscan"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// MaxExecutionOutputBytes caps what GetTaskExecutionOutput returns: the end of a run says the most, and the
// stored column is already limited to 8 KiB.
const MaxExecutionOutputBytes = 8 * 1024

type TaskExecutionOutput struct {
	Found     bool
	Output    string
	Redacted  bool
	Truncated bool
}

// GetTaskExecutionOutput shows the stdout of a task's last successful run to people who can read the task. Runs
// without a contract result block leave no ExecutionRecord, so this column is all the web has to show them. It is a
// separate read, not a Task field, because GetTask and ListTasks answer without a grant check and must not carry output.
type GetTaskExecutionOutput struct {
	tasks TaskRepository
	perm  taskPermissionChecker
}

func NewGetTaskExecutionOutput(tasks TaskRepository, perm taskPermissionChecker) *GetTaskExecutionOutput {
	return &GetTaskExecutionOutput{tasks: tasks, perm: perm}
}

func (uc *GetTaskExecutionOutput) Execute(ctx context.Context, taskID string) (TaskExecutionOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return TaskExecutionOutput{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if taskID == "" {
		return TaskExecutionOutput{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTION_OUTPUT_BAD_REQUEST", "task_id is required", nil)
	}
	if err := requireGrantWhenUser(ctx, uc.perm, taskID, "read"); err != nil {
		return TaskExecutionOutput{}, err
	}
	task, err := uc.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return TaskExecutionOutput{}, taskRepoError(err, "task not found", "TASK_FETCH_FAILED", "failed to fetch task")
	}
	if task.LastExecutionOutput == "" {
		return TaskExecutionOutput{}, nil
	}
	// Redact the whole stored text first so a secret cut by the cap below is never left half visible.
	text, redacted := secretscan.Redact(task.LastExecutionOutput)
	tail := domain.TailUTF8(text, MaxExecutionOutputBytes)
	return TaskExecutionOutput{Found: true, Output: tail, Redacted: redacted, Truncated: len(tail) < len(text)}, nil
}
