// Package taskserviceclient implements orchestration-service's outbound
// TaskServiceReporter port against task-service's real
// ReportTaskExecutionResult RPC (TASK-FT-002-04).
package taskserviceclient

import (
	"context"
	"fmt"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// Reporter implements usecase.TaskServiceReporter for real, against
// task-service's now-real ReportTaskExecutionResult handler
// (TASK-FT-002-04, replacing the prior compile-only-stub gap TASK-TASKV1-005
// flagged here).
//
// engine is hardcoded to "orchestration" — this adapter is Engine 2's own
// reporter, matching TASK-FT-002-01's engine-neutral proto shape (BE-SOL-002
// generalizes SOL-TG-04/TASK-TG-04-05's original 5-field
// task_id/coordinator_run_id/success/actual_hours/error_message shape by
// renaming field 2 to execution_ref and adding field 6, engine). This port's
// own ReportResult signature carries no actual_hours parameter — orchestration-
// service's CoordinatorRun domain has no such field to report yet, so it is
// sent as 0, an honest pre-existing gap, not something this rename
// introduces.
type Reporter struct {
	tasks taskv1.TaskServiceClient
}

func NewReporter(tasks taskv1.TaskServiceClient) *Reporter {
	return &Reporter{tasks: tasks}
}

func (r *Reporter) ReportResult(ctx context.Context, taskID, coordinatorRunID string, success bool, errMsg string) error {
	_, err := r.tasks.ReportTaskExecutionResult(ctx, &taskv1.ReportTaskExecutionResultRequest{
		TaskId:       taskID,
		ExecutionRef: coordinatorRunID,
		Success:      success,
		ErrorMessage: errMsg,
		Engine:       "orchestration",
	})
	if err != nil {
		return fmt.Errorf("taskserviceclient: report task execution result: %w", err)
	}
	return nil
}
