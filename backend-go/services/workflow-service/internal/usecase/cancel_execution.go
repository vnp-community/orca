package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type CancelExecutionInput struct {
	ID string
}

// CancelExecution is the last item of Epic C's originally-deferred
// workflow-service RPCs (docs/execution-plan.md §2/§10) — unlike
// ListTemplates/ResolveTemplate, it never depended on template
// inheritance existing, so it's a straightforward peer of
// PauseExecution/ResumeExecution: fetch/authorize/persist around the
// running->cancelled (or pending/paused->cancelled) invariant on
// domain.WorkflowExecution.Cancel.
type CancelExecution struct {
	executions ExecutionRepository
}

func NewCancelExecution(executions ExecutionRepository) *CancelExecution {
	return &CancelExecution{executions: executions}
}

func (uc *CancelExecution) Execute(ctx context.Context, in CancelExecutionInput) (domain.WorkflowExecution, error) {
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

	if err := exec.Cancel(); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_CANNOT_CANCEL", err.Error(), err)
	}

	if err := uc.executions.UpdateExecution(ctx, exec); err != nil {
		return domain.WorkflowExecution{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_UPDATE_FAILED", "failed to persist cancelled execution", err)
	}

	return exec, nil
}
