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
// AIApply's identical relationship with CreateTask. The standalone AddEdge
// RPC path gets its atomicity from the gRPC handler wrapping this usecase in
// a RunInTx call instead — see server.go's AddEdge handler — closing this
// usecase's previous check-then-write race (cycle check + edge write, now
// also + auto-block write, were 3 separate calls with no atomicity
// guarantee between them). Delegates to addEdgeWithinTx, which uses
// ListByKindForUpdate's row-locked cycle check to also close the
// check-then-write race WITHIN the transaction (README.md's "known gap").
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

	if err := addEdgeWithinTx(ctx, tenantID, uc.tasks, uc.edges, edge); err != nil {
		return domain.TaskEdge{}, err
	}
	return edge, nil
}

// addEdgeWithinTx is the cycle-check + write + auto-block core, factored
// out so it can run against an ALREADY-open transaction's TaskRepository/
// EdgeRepository pair — AddEdge.Execute calls it against whatever pair it
// was constructed with; AIApply's own RunInTx-scoped subtask loop
// (ai_apply.go) can call it directly for the same reason: Repository.RunInTx
// always begins a fresh transaction against the pool (not any
// already-open pgx.Tx), so nesting a second RunInTx call there would
// silently open an unrelated transaction and break AIApply's all-or-nothing
// guarantee.
func addEdgeWithinTx(ctx context.Context, tenantID string, tasks TaskRepository, edges EdgeRepository, edge domain.TaskEdge) error {
	if edge.Kind == domain.EdgeKindDependsOn {
		existing, err := edges.ListByKindForUpdate(ctx, tenantID, domain.EdgeKindDependsOn)
		if err != nil {
			return apperrors.New(apperrors.KindInternal, "TASK_EDGE_LIST_FAILED", "failed to list existing edges for cycle check", err)
		}
		if domain.DetectCycle(existing, edge) {
			return apperrors.New(apperrors.KindFailedPrecondition, "TASK_CYCLIC_DEPENDENCY", domain.ErrCyclicDependency.Error(), domain.ErrCyclicDependency)
		}
	}
	if err := edges.Add(ctx, tenantID, edge); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_EDGE_ADD_FAILED", "failed to persist edge", err)
	}

	if edge.Kind == domain.EdgeKindDependsOn {
		dep, err := tasks.Get(ctx, tenantID, edge.ToTaskID)
		if err != nil {
			return apperrors.New(apperrors.KindInternal, "TASK_EDGE_DEP_LOOKUP_FAILED", "failed to load dependency task", err)
		}
		if dep.Status != domain.StatusDone && dep.Status != domain.StatusCancelled {
			if err := tasks.UpdateStatus(ctx, tenantID, edge.FromTaskID, domain.StatusBlocked); err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_EDGE_AUTO_BLOCK_FAILED", "failed to auto-block dependent task", err)
			}
		}
	}
	return nil
}
