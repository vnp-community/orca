package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ReportTaskExecutionResultInput mirrors ReportTaskExecutionResultRequest
// (TASK-FT-002-01's engine-neutral, 6-field shape).
type ReportTaskExecutionResultInput struct {
	TaskID       string
	ExecutionRef string
	Success      bool
	ActualHours  float64
	ErrorMessage string
	Engine       string // "orchestration" | "workflow"
}

// ReportTaskExecutionResult is the shared inbound completion callback for
// Engine 2 (orchestration-service's autonomous coordinator) and Engine 3
// (workflow-service's runToCompletion) — generalizes SOL-TG-04/
// TASK-TG-04-05's original Engine-2-only design (see TASK-FT-002-01's
// Context note) to compare execution_ref/engine against TASK-FT-001-01's
// task.execution_links table (via domain.Task.ActiveExecutionLinkID),
// rather than a bare active_execution_id column.
type ReportTaskExecutionResult struct {
	tasks TaskRepository
	links ExecutionLinkRepository
}

func NewReportTaskExecutionResult(tasks TaskRepository, links ExecutionLinkRepository) *ReportTaskExecutionResult {
	return &ReportTaskExecutionResult{tasks: tasks, links: links}
}

// Execute validates the callback against the task's currently active
// execution link and, if it matches, marks the link terminal and completes
// the task. A no-active-link task or a stale/duplicate execution_ref/engine
// is a silent no-op — at-least-once consumer idempotence, per
// 05-data-architecture.md's outbox-consumer convention (the same rule
// SOL-TG-04 already established for Engine 2).
func (uc *ReportTaskExecutionResult) Execute(ctx context.Context, in ReportTaskExecutionResultInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if task.ActiveExecutionLinkID == "" {
		// No active link recorded at all — a callback arriving after the
		// task was already re-dispatched (its link cleared/replaced) or for
		// a task that never went through ExecuteTask. Ignored, not an
		// error, same idempotence posture as the staleness case below.
		return nil
	}
	link, err := uc.links.GetExecutionLink(ctx, tenantID, task.ActiveExecutionLinkID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_EXECUTION_LINK_LOOKUP_FAILED", "failed to load active execution link", err)
	}
	if link.ExternalRefID != in.ExecutionRef || string(link.Engine) != in.Engine {
		// Stale/duplicate callback (retried delivery, or a callback for a
		// run this task was re-dispatched away from) — ignored, not an
		// error.
		return nil
	}

	if in.Success {
		if err := uc.links.Complete(ctx, tenantID, link.ID, "completed"); err != nil {
			return apperrors.New(apperrors.KindInternal, "TASK_EXECUTION_LINK_COMPLETE_FAILED", "failed to complete execution link", err)
		}
		return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, string(domain.StatusReview), in.ActualHours)
	}

	// Failed complex/workflow execution. domain.StatusBlocked does not
	// exist yet in this codebase (BUG-TASKV1-001's scope, not this task's —
	// see TASK-FT-002-04's Context note) and the tasks_status_check
	// constraint (migrations/0003) admits no "failed"-shaped status, so this
	// deliberately does NOT call CompleteExecution: the task is left at
	// StatusInProgress rather than forced into a status that doesn't
	// honestly describe "async dispatch failed." Flagged as a temporary gap
	// until a real blocked/failed task status lands.
	if err := uc.links.Complete(ctx, tenantID, link.ID, "failed"); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_EXECUTION_LINK_COMPLETE_FAILED", "failed to complete execution link", err)
	}
	slog.WarnContext(ctx, "task: async execution failed; leaving task in_progress (no blocked/failed task status exists yet, BUG-TASKV1-001)",
		slog.String("task_id", in.TaskID), slog.String("engine", in.Engine), slog.String("execution_ref", in.ExecutionRef), slog.String("error_message", in.ErrorMessage))
	return nil
}
