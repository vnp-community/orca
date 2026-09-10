package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RecalculateProgress recomputes done_subtasks/total_subtasks for a task's
// whole ancestor chain in one repository round trip (TaskRepository.
// RecalculateAncestorProgress's WITH RECURSIVE UPDATE) — see BE-SOL-001's
// design section. Called from UpdateTask's status-transition path today
// (see that usecase's call site) whenever a child task with a non-empty
// ParentID transitions to a terminal status; ExecuteTask has no completion
// callback yet to wire this into on the execute path (see
// TASK-TG-005-01/-02 for that gap).
type RecalculateProgress struct {
	tasks TaskRepository
}

func NewRecalculateProgress(tasks TaskRepository) *RecalculateProgress {
	return &RecalculateProgress{tasks: tasks}
}

func (uc *RecalculateProgress) Execute(ctx context.Context, taskID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.tasks.RecalculateAncestorProgress(ctx, tenantID, taskID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_RECALCULATE_PROGRESS_FAILED", "failed to recalculate ancestor progress", err)
	}
	return nil
}
