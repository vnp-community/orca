package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type PauseExecutionInput struct {
	ID string
}

// PauseExecution is the user-triggered pause path (TS's paused_at,
// migration 0014 — see workflow-service.md §4-5). The running->paused
// invariant lives on domain.WorkflowExecution.Pause; this usecase's job is
// fetch/authorize/persist around that domain call.
type PauseExecution struct {
	executions ExecutionRepository
}

func NewPauseExecution(executions ExecutionRepository) *PauseExecution {
	return &PauseExecution{executions: executions}
}

func (uc *PauseExecution) Execute(ctx context.Context, in PauseExecutionInput) (domain.WorkflowExecution, error) {
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

	if err := exec.Pause(time.Now().UTC()); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_CANNOT_PAUSE", err.Error(), err)
	}

	if err := uc.executions.UpdateExecution(ctx, exec, nil); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_UPDATE_FAILED", "failed to persist paused execution", err)
	}

	return exec, nil
}
