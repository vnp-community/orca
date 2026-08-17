package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AddEdgeInput struct {
	FromTaskID string
	ToTaskID   string
	Kind       domain.EdgeKind
}

// AddEdge is task-service's edge-mutation usecase — the one place
// domain.DetectCycle gets called, per task-service.md §4/§8. Only
// depends_on edges are cycle-checked: parent_child's single-parent
// invariant is a different, DB-enforced constraint (unique index on
// to_task_id), not a cycle in the sense TaskDAGValidator guards against.
type AddEdge struct {
	edges EdgeRepository
}

func NewAddEdge(edges EdgeRepository) *AddEdge {
	return &AddEdge{edges: edges}
}

func (uc *AddEdge) Execute(ctx context.Context, in AddEdgeInput) (domain.TaskEdge, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	edge, err := domain.NewTaskEdge(in.FromTaskID, in.ToTaskID, in.Kind)
	if err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EDGE_INVALID", err.Error(), err)
	}

	if edge.Kind == domain.EdgeKindDependsOn {
		// NOTE: fetching the existing edge set and then writing the new one
		// is two separate calls, not one transaction — task-service.md §8
		// requires the cycle check and the write to be atomic so a
		// concurrent AddEdge can't slip a cycle in between. Not wired in
		// this scaffold; see this service's README's "known gaps" section.
		existing, err := uc.edges.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn)
		if err != nil {
			return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_LIST_FAILED", "failed to list existing edges for cycle check", err)
		}
		if domain.DetectCycle(existing, edge) {
			return domain.TaskEdge{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_CYCLIC_DEPENDENCY", domain.ErrCyclicDependency.Error(), domain.ErrCyclicDependency)
		}
	}

	if err := uc.edges.Add(ctx, tenantID, edge); err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_ADD_FAILED", "failed to persist edge", err)
	}
	return edge, nil
}
