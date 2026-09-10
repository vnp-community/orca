package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// defaultListExecutionsLimit matches ListTemplates' lack of an explicit
// default in its own usecase — no equivalent constant exists there today,
// so this value is this usecase's own, documented choice, not copied from
// an existing default.
const defaultListExecutionsLimit = 20

type ListExecutionsInput struct {
	ProjectID string
	Cursor    string
	Limit     int32
}

type ListExecutionsOutput struct {
	Executions []domain.WorkflowExecution
	NextCursor string
}

type ListExecutions struct {
	executions ExecutionRepository
}

func NewListExecutions(executions ExecutionRepository) *ListExecutions {
	return &ListExecutions{executions: executions}
}

// Execute lists tenantID's (from ctx)/in.ProjectID's workflow executions,
// newest first — backs TASK-WF-005-01's ListExecutions RPC, modeled
// directly on GetExecution's tenant-then-repo-call-then-error-mapping
// pattern.
func (uc *ListExecutions) Execute(ctx context.Context, in ListExecutionsInput) (ListExecutionsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListExecutionsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	limit := in.Limit
	if limit <= 0 {
		limit = defaultListExecutionsLimit
	}
	execs, next, err := uc.executions.ListExecutions(ctx, tenantID, in.ProjectID, in.Cursor, limit)
	if err != nil {
		return ListExecutionsOutput{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_LIST_EXECUTIONS_FAILED", "failed to list workflow executions", err)
	}
	return ListExecutionsOutput{Executions: execs, NextCursor: next}, nil
}
