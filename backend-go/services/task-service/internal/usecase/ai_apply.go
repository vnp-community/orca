package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// AIApplyInput mirrors the AIApply RPC request — TenantID isn't a field
// here; CreateTask/AddEdge (the two usecases this delegates to) each pull
// it from context themselves.
type AIApplyInput struct {
	TaskID    string
	Proposals []domain.SubtaskProposal
}

// AIApply commits a (possibly user-edited) proposal set from a prior
// AIDecompose call: for each proposal, creates a subtask (CreateTask) and
// links it to TaskID with a parent_child edge (AddEdge) — the two-step
// review-before-commit shape TASK-224 specifies.
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
// edge add for this call now runs inside ONE Postgres transaction opened by
// TxRunner.RunInTx (internal/adapter/postgres.Repository.RunInTx, via
// pgx.BeginFunc): if any proposal's CreateTask or AddEdge call fails, the
// whole transaction rolls back and NO subtask from this call is left
// behind — see ai_apply_test.go's
// TestAIApply_MidLoopFailure_RollsBackEntireSubtree for the regression proof
// (previously ai_apply_test.go's TestAIApply_MidLoopFailure_* test name
// documented the opposite: a surfaced error but a real partial subtree left
// behind — that gap is what this change removes).
type AIApply struct {
	txRunner TxRunner
}

func NewAIApply(txRunner TxRunner) *AIApply {
	return &AIApply{txRunner: txRunner}
}

func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
	created := make([]domain.Task, 0, len(in.Proposals))
	err := uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		// Scoped to the open transaction's repos (tasks/edges), not the
		// outer-scope repos NewAIApply might otherwise have captured — see
		// TxRunner's doc comment for why this reuses CreateTask/AddEdge
		// unchanged rather than duplicating their logic.
		// nil GrantRepository: AI-generated subtasks have no human
		// "creator" to owner-grant (see NewCreateTask's doc comment,
		// TASK-TG-003-02's option (b)) — this loop never sets CreatorID
		// either, so CreateTask.Execute's grant step is simply never
		// reached from this call site.
		createTask := NewCreateTask(tasks, nil)
		addEdge := NewAddEdge(tasks, edges) // TASK-TG-001-04's 2-arg constructor
		createdIDs := make([]string, len(in.Proposals))
		for i, p := range in.Proposals {
			task, err := createTask.Execute(ctx, CreateTaskInput{
				Title: p.Title, ParentID: in.TaskID,
				Description: p.Description, Type: p.Type,
				EstimatedHours: p.EstimatedHours, PromptTemplate: p.PromptTemplate,
			})
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create subtask from AI proposal", err)
			}
			createdIDs[i] = task.ID
			if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: in.TaskID, ToTaskID: task.ID, Kind: domain.EdgeKindParentChild}); err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link subtask to parent", err)
			}
			created = append(created, task)
		}
		// depends_on edges are added in a SECOND pass, after every subtask
		// in the batch has a real ID — DependsOnIndex references index
		// positions in the SAME batch (see domain.SubtaskProposal's doc
		// comment), which don't resolve to anything until the loop above
		// has run to completion. Reuses AddEdge.Execute unchanged, so the
		// cycle check AND the auto-block write both run per edge
		// automatically (TASK-TG-001-04) — a same-batch DependsOnIndex
		// cycle is rejected exactly like a manual AddEdge call would
		// reject it, and the whole RunInTx closure rolling back means zero
		// tasks/edges persist from a rejected batch.
		for i, p := range in.Proposals {
			for _, depIdx := range p.DependsOnIndex {
				if depIdx < 0 || depIdx >= len(createdIDs) {
					return apperrors.New(apperrors.KindInvalidArgument, "TASK_AI_APPLY_INVALID_DEPENDENCY_INDEX", "depends_on_index out of range for this batch", nil)
				}
				if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: createdIDs[depIdx], ToTaskID: createdIDs[i], Kind: domain.EdgeKindDependsOn}); err != nil {
					return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link AI-proposed dependency edge", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}
