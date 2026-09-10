package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ReportTaskExecutionResultInput struct {
	TaskID           string
	CoordinatorRunID string
	Success          bool
	ActualHours      float64
}

// ReportTaskExecutionResult is the complex path's inbound completion
// callback — orchestration-service calls this when a coordinator_run
// reaches a terminal state (orchestration-service.md §2.2: "task-service
// calls in to start a run and read its terminal result; it never touches
// this service's tables directly", the orch --> task dependency-graph
// edge). See this file's own security note below — SOL-TG-04.
//
// Security note (flagged, not resolved here): this RPC must only be
// callable BY orchestration-service, never a browser/mobile client via
// api-gateway. Checked common/grpcmw (this codebase's shared interceptor
// package) for an existing mTLS/mesh-identity-extraction interceptor to
// reuse, per this task's own instruction not to guess — there isn't one:
// grpcmw today only has TenantExtractionInterceptor (trusts caller-supplied
// metadata), RecoveryInterceptor, and LoggingInterceptor, no service-identity
// check at all. A placeholder guard is intentionally NOT substituted here
// (asserting the wrong thing would be worse than flagging the gap) — see
// internal/adapter/grpc.Server.ReportTaskExecutionResult's doc comment for
// where that check belongs once a real mesh-identity mechanism exists in
// this codebase.
type ReportTaskExecutionResult struct {
	tasks TaskRepository
}

func NewReportTaskExecutionResult(tasks TaskRepository) *ReportTaskExecutionResult {
	return &ReportTaskExecutionResult{tasks: tasks}
}

func (uc *ReportTaskExecutionResult) Execute(ctx context.Context, in ReportTaskExecutionResultInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if task.ActiveExecutionID != in.CoordinatorRunID {
		// Stale/duplicate callback (retried delivery, or a callback for a
		// run this task was re-dispatched away from) — ignored, not an
		// error, per at-least-once consumer idempotence.
		return nil
	}
	if in.Success {
		return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusReview, in.ActualHours)
	}
	// Failed complex execution -> Blocked, not silently left in_progress or
	// auto-Done.
	return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusBlocked, in.ActualHours)
}
