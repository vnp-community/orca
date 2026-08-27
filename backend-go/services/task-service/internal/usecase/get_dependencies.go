package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// GetDependenciesInput mirrors the GetDependencies RPC request. See
// CreateTaskInput's doc comment for why TenantID isn't a field here.
type GetDependenciesInput struct {
	TaskID string
}

// GetDependencies walks depends_on edges FROM TaskID and hydrates each
// target into a full domain.Task — distinct from AddEdge (write) and from
// GetAncestors (parent_child, not on this proto's surface yet either).
// Reuses EdgeRepository.ListFrom, the same call ExecuteTask's isComplex
// check already makes for the identical edge kind — no new repository
// method needed for the edge read itself, only this hydration step.
type GetDependencies struct {
	tasks TaskRepository
	edges EdgeRepository
}

func NewGetDependencies(tasks TaskRepository, edges EdgeRepository) *GetDependencies {
	return &GetDependencies{tasks: tasks, edges: edges}
}

func (uc *GetDependencies) Execute(ctx context.Context, in GetDependenciesInput) ([]domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "TASK_MISSING_TASK_ID", "task_id is required", nil)
	}

	edges, err := uc.edges.ListFrom(ctx, tenantID, in.TaskID, domain.EdgeKindDependsOn)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TASK_GET_DEPENDENCIES_FAILED", "failed to list dependency edges", err)
	}
	tasks := make([]domain.Task, 0, len(edges))
	for _, e := range edges {
		t, err := uc.tasks.Get(ctx, tenantID, e.ToTaskID)
		if err != nil {
			// A hydration failure on one edge propagates as an error rather
			// than silently skipping it — see TASK-226's test plan.
			return nil, apperrors.New(apperrors.KindInternal, "TASK_GET_DEPENDENCIES_FAILED", "failed to hydrate dependency task", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}
