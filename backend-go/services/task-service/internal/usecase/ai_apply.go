package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// AIApplyInput mirrors the AIApply RPC request — TenantID isn't a field
// here; CreateTask/AddEdge (the two usecases this delegates to) each pull
// it from context themselves.
type AIApplyInput struct {
	TaskID        string
	Proposals     []domain.SubtaskProposal
	RawAIResponse string // echoed from AIDecomposeResult.RawResponse, see TASK-TG-02-01's proto field
}

// AIApply commits a (possibly user-edited) proposal set from a prior
// AIDecompose call: for each proposal, creates a subtask (CreateTask) and
// links it to TaskID with a parent_child edge, then — once every proposal
// in the call has a real Task.ID — a second pass adds depends_on edges from
// each proposal's DependsOnIndices, and RawAIResponse (if any) is persisted
// to Task.AIPlanJSON. The two-step review-before-commit shape TASK-224
// specifies, widened by TASK-TG-02-05 for dependency edges + ai_plan_json.
//
// TASK-224 Gap 2, closed: this loop previously ran outside any transaction,
// so one failed subtask insert partway through could leave a partial
// subtree (some proposals committed as real tasks + edges, the rest
// silently absent). Re-checked against the WHOLE backend-go tree (not just
// task-service) before writing this — a real pgx.Tx precedent now exists in
// several services (project-service, issue-tracking-service, usage-service,
// orchestration-service, automation-service all call pool.Begin(ctx)
// directly; credential-broker-service goes further with a named
// TxRunner/RunInTx port + dbtx abstraction in its postgres adapter). This
// usecase adopts credential-broker-service's TxRunner shape exactly (see
// usecase.TxRunner's doc comment) rather than the bare pool.Begin calls,
// since it already matches this loop's "hand fn a repo pair, run several
// port calls, commit once" need with no adaptation. Every subtask create +
// edge add for this call (both passes, plus the ai_plan_json write) now
// runs inside ONE Postgres transaction opened by TxRunner.RunInTx
// (internal/adapter/postgres.Repository.RunInTx, via pgx.BeginFunc): if any
// step fails, the whole transaction rolls back and NO subtask from this
// call is left behind — see ai_apply_test.go's
// TestAIApply_MidLoopFailure_RollsBackEntireSubtree for the regression
// proof.
type AIApply struct {
	txRunner TxRunner
}

func NewAIApply(txRunner TxRunner) *AIApply {
	return &AIApply{txRunner: txRunner}
}

func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	created := make([]domain.Task, 0, len(in.Proposals))
	err = uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		// Scoped to the open transaction's repos (tasks/edges), not the
		// outer-scope repos NewAIApply might otherwise have captured — see
		// TxRunner's doc comment for why this reuses CreateTask unchanged.
		// Edge adds call addEdgeWithinTx directly (NOT NewAddEdge(...).Execute)
		// — AddEdge now opens its OWN transaction via TxRunner, and nesting
		// that here would start an unrelated second transaction against the
		// pool rather than participating in this one, breaking AIApply's
		// all-or-nothing guarantee. See addEdgeWithinTx's doc comment
		// (add_edge.go).
		createTask := NewCreateTask(tasks)
		for _, p := range in.Proposals {
			task, err := createTask.Execute(ctx, CreateTaskInput{
				Title: p.Title, Description: p.Description, ParentID: in.TaskID,
				Type: p.Type, EstimatedHours: p.EstimatedHours, PromptTemplate: p.PromptTemplate,
			})
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create subtask from AI proposal", err)
			}
			edge, err := domain.NewTaskEdge(in.TaskID, task.ID, domain.EdgeKindParentChild)
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to build parent-child edge", err)
			}
			if err := addEdgeWithinTx(ctx, tenantID, tasks, edges, edge); err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link subtask to parent", err)
			}
			created = append(created, task)
		}

		// Second pass: proposal indices only resolve to real Task.IDs once
		// every proposal in this call has been created above.
		for i, p := range in.Proposals {
			for _, depIdx := range p.DependsOnIndices {
				if depIdx < 0 || depIdx >= len(created) {
					continue // defensive: AI hallucinated an out-of-range index
				}
				depEdge, err := domain.NewTaskEdge(created[i].ID, created[depIdx].ID, domain.EdgeKindDependsOn)
				if err != nil {
					continue // defensive: e.g. a self-dependency (i == depIdx) — skip, don't fail the whole apply
				}
				if err := addEdgeWithinTx(ctx, tenantID, tasks, edges, depEdge); err != nil {
					return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create dependency edge from AI proposal", err)
				}
			}
		}

		if in.RawAIResponse != "" {
			if err := tasks.UpdateAIPlanJSON(ctx, tenantID, in.TaskID, in.RawAIResponse); err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to persist ai_plan_json", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}
