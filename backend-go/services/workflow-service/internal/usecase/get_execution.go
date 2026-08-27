package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

type GetExecutionInput struct {
	ID string
}

type GetExecution struct {
	executions ExecutionRepository
}

func NewGetExecution(executions ExecutionRepository) *GetExecution {
	return &GetExecution{executions: executions}
}

func (uc *GetExecution) Execute(ctx context.Context, in GetExecutionInput) (domain.WorkflowExecution, error) {
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
	return exec, nil
}
