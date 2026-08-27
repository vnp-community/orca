package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type ResumeExecutionInput struct {
	ID string
}

// ResumeExecution is the counterpart to PauseExecution. It must never be
// confused with the boot-time recovery scan (workflow-service.md §8), which
// re-attaches to a still-running execution's root_trace_id directly rather
// than calling Resume — this usecase is only for a deliberate user action
// against a paused execution.
type ResumeExecution struct {
	executions ExecutionRepository
}

func NewResumeExecution(executions ExecutionRepository) *ResumeExecution {
	return &ResumeExecution{executions: executions}
}

func (uc *ResumeExecution) Execute(ctx context.Context, in ResumeExecutionInput) (domain.WorkflowExecution, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	exec, err := uc.executions.GetExecution(ctx, tenantID, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrExecutionNotFound) {
			return domain.WorkflowExecution{}, apperrors.New(apperrors.KindNotFound, "WORKFLOW_EXECUTION_NOT_FOUND", "workflow execution not found", err)
		}
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_FETCH_FAILED", "failed to fetch workflow execution", err)
	}

	// A real invariant check, not just a status flip: resuming a
	// non-paused execution (e.g. already running, or completed) is
	// rejected rather than silently succeeding.
	if err := exec.Resume(); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_CANNOT_RESUME", err.Error(), err)
	}

	if err := uc.executions.UpdateExecution(ctx, exec, nil); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_UPDATE_FAILED", "failed to persist resumed execution", err)
	}

	return exec, nil
}
