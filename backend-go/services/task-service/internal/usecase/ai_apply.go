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
// KNOWN GAP, not solved by this usecase: this loop is NOT wrapped in a
// single database transaction. task-service.md §8 calls for
// check-then-write atomicity on edge mutations (the same gap AddEdge's own
// doc comment already names for its cycle-check-then-insert), and by the
// same logic, one failed subtask insert partway through this loop can
// leave a partial subtree — some proposals committed as real tasks +
// edges, the rest silently absent, with no rollback. ports.go currently
// exposes no WithTx/UnitOfWork primitive (checked before writing this;
// none exists anywhere in this repo yet), and adding one is a real
// architectural change — threading a transaction-scoped repository
// variant through CreateTask/AddEdge's constructors — well beyond this
// usecase's own scope. See TASK-224's report for this gap; close it by
// adding a transaction-scoped repository (or a shared UnitOfWork port) as
// its own follow-up task, not by hand-rolling one here under this task's
// time budget.
type AIApply struct {
	createTask *CreateTask
	addEdge    *AddEdge
}

func NewAIApply(createTask *CreateTask, addEdge *AddEdge) *AIApply {
	return &AIApply{createTask: createTask, addEdge: addEdge}
}

func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
	created := make([]domain.Task, 0, len(in.Proposals))
	for _, p := range in.Proposals {
		task, err := uc.createTask.Execute(ctx, CreateTaskInput{Title: p.Title, ParentID: in.TaskID})
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create subtask from AI proposal", err)
		}
		if _, err := uc.addEdge.Execute(ctx, AddEdgeInput{FromTaskID: in.TaskID, ToTaskID: task.ID, Kind: domain.EdgeKindParentChild}); err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to link subtask to parent", err)
		}
		created = append(created, task)
	}
	return created, nil
}
