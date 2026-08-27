package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// HasActiveExecutionsInput mirrors HasActiveExecutionsRequest.
type HasActiveExecutionsInput struct {
	ProjectID string
}

// HasActiveExecutions answers whether projectID has any workflow execution
// in a non-terminal status (pending/running/paused) — the real fix for
// project-service.RebindDevServer's previously-no-op guard (Epic C,
// docs/execution-plan.md). Tenant-scoped via context, like every other
// usecase in this service.
type HasActiveExecutions struct {
	executions ExecutionRepository
}

func NewHasActiveExecutions(executions ExecutionRepository) *HasActiveExecutions {
	return &HasActiveExecutions{executions: executions}
}

func (uc *HasActiveExecutions) Execute(ctx context.Context, in HasActiveExecutionsInput) (bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	if in.ProjectID == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_HAS_ACTIVE_EXECUTIONS_INVALID", "project_id is required", nil)
	}

	hasActive, err := uc.executions.HasActiveExecutions(ctx, tenantID, in.ProjectID)
	if err != nil {
		return false, apperrors.New(apperrors.KindInternal, "WORKFLOW_HAS_ACTIVE_EXECUTIONS_QUERY_FAILED", "failed to query active executions", err)
	}
	return hasActive, nil
}
