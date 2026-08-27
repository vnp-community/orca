package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// HasActiveExecutionsInput carries the HasActiveExecutions RPC's one
// argument — see that RPC's doc comment in the generated
// taskv1.TaskServiceServer for the cross-service context (Epic C,
// backend-go/docs/execution-plan.md, closing project-service.RebindDevServer's
// active-execution guard).
type HasActiveExecutionsInput struct {
	ProjectID string
}

// HasActiveExecutions answers "does this project have a task currently
// dispatched for execution". This is a REAL query against a REAL, persisted
// column (task.tasks.project_id / status) — not a stub — but it is honestly
// subject to this scaffold's one-way status-transition limitation: the only
// place a task's status ever becomes StatusInProgress is ExecuteTask's
// dispatch (see that usecase's doc comment), and nothing in this scaffold
// ever transitions it back out again, because task-service has no
// execution-completion callback and the generated proto has no
// UpdateTask/SetStatus RPC to drive one manually. Concretely: this will
// answer "active" for a task forever after it is first executed, even long
// after the underlying work has actually finished. Closing that requires a
// real completion path — see this service's README "Known gaps" — which is
// a separate, later piece of work, not this usecase's scope.
type HasActiveExecutions struct {
	repo TaskRepository
}

func NewHasActiveExecutions(repo TaskRepository) *HasActiveExecutions {
	return &HasActiveExecutions{repo: repo}
}

func (uc *HasActiveExecutions) Execute(ctx context.Context, in HasActiveExecutionsInput) (bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return false, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.ProjectID == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "TASK_HAS_ACTIVE_EXECUTIONS_INVALID", "project_id is required", nil)
	}

	hasActive, err := uc.repo.HasActiveExecutions(ctx, tenantID, in.ProjectID)
	if err != nil {
		return false, apperrors.New(apperrors.KindInternal, "TASK_HAS_ACTIVE_EXECUTIONS_QUERY_FAILED", "failed to query active executions", err)
	}
	return hasActive, nil
}
