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
//
// AddEdge does NOT open its own transaction — it trusts whatever tasks/edges
// pair it was constructed with are already correctly scoped (either the
// pool-backed repos for a standalone call, or tx-scoped repos when
// constructed inside a usecase.TxRunner.RunInTx closure), mirroring
// AIApply's existing relationship with CreateTask. The standalone AddEdge
// RPC path gets its atomicity from the gRPC handler wrapping this usecase in
// a RunInTx call instead — see server.go's AddEdge handler — closing this
// usecase's previous check-then-write race (cycle check + edge write, now
// also + auto-block write, were 3 separate calls with no atomicity
// guarantee between them).
type AddEdge struct {
	tasks TaskRepository
	edges EdgeRepository
}

func NewAddEdge(tasks TaskRepository, edges EdgeRepository) *AddEdge {
	return &AddEdge{tasks: tasks, edges: edges}
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

	// Auto-block: a fresh depends_on edge onto a not-yet-done dependency
	// blocks the dependent task immediately — BE-SOL-001's auto-block
	// design. Runs against the same tasks/edges pair as the checks/write
	// above, so it shares their transactional scope when AddEdge is
	// constructed inside a RunInTx closure.
	if edge.Kind == domain.EdgeKindDependsOn {
		fromTask, err := uc.tasks.Get(ctx, tenantID, in.FromTaskID)
		if err != nil {
			return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_AUTOBLOCK_LOOKUP_FAILED", "failed to load dependency task for auto-block check", err)
		}
		if fromTask.Status != domain.StatusDone {
			if err := uc.tasks.UpdateStatus(ctx, tenantID, in.ToTaskID, domain.StatusBlocked); err != nil {
				return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_AUTOBLOCK_FAILED", "failed to auto-block dependent task", err)
			}
		}
	}
	return edge, nil
}
