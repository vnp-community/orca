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

// AddEdge is task-service's edge-mutation usecase. Runs the cycle check
// (depends_on only) and the write in ONE transaction via TxRunner, closing
// README.md's "known gap": a SELECT ... FOR UPDATE over the depends_on edge
// set (EdgeRepository.ListByKindForUpdate) closes the race the prior
// two-call (ListByKind then Add) shape allowed. Also implements auto-block:
// adding "from depends_on to" means "from must wait for to" — if `to` isn't
// Done/Cancelled, `from` transitions to StatusBlocked.
type AddEdge struct {
	txRunner TxRunner
}

func NewAddEdge(txRunner TxRunner) *AddEdge {
	return &AddEdge{txRunner: txRunner}
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

	err = uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		return addEdgeWithinTx(ctx, tenantID, tasks, edges, edge)
	})
	if err != nil {
		return domain.TaskEdge{}, err
	}
	return edge, nil
}

// addEdgeWithinTx is the cycle-check + write + auto-block core, factored
// out so it can run against an ALREADY-open transaction's TaskRepository/
// EdgeRepository pair — AddEdge.Execute calls it via TxRunner.RunInTx;
// AIApply's own RunInTx-scoped subtask loop (ai_apply.go) calls it
// directly, since Repository.RunInTx always begins a fresh transaction
// against the pool (not the currently-open pgx.Tx) — nesting a second
// RunInTx call there would silently open an unrelated transaction and
// break AIApply's all-or-nothing guarantee.
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
