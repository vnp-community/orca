# TASK-TG-02-05: `AIApply` — dependency edges from proposal indices + `ai_plan_json` persistence

**From Solution:** SOL-TG-02
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/ai_apply.go`
**Depends on:** TASK-TG-02-04, TASK-TG-01-07 (transaction-scoped `AddEdge` auto-block), TASK-TG-01-04 (`UpdateAIPlanJSON`)
**Status:** `[ ]` TODO

---

## Context

`AIApply` currently only creates one subtask + `parent_child` edge per
proposal. It needs a second pass — after every proposal in the call has
been created (so `DependsOnIndices` can resolve to real `Task.ID`s) — to add
`depends_on` edges, plus persisting the raw AI response into `ai_plan_json`.
Both stay inside the same `TxRunner.RunInTx` transaction `AIApply` already
uses, so the existing rollback guarantee
(`TestAIApply_MidLoopFailure_RollsBackEntireSubtree`) extends to the new
edges without a new test concept.

## Changes to make

Rewrite `backend-go/services/task-service/internal/usecase/ai_apply.go`'s
`AIApplyInput` and `Execute`:

```go
type AIApplyInput struct {
	TaskID        string
	Proposals     []domain.SubtaskProposal
	RawAIResponse string // echoed from AIDecomposeResult.RawResponse, see TASK-TG-02-01's proto field
}

func (uc *AIApply) Execute(ctx context.Context, in AIApplyInput) ([]domain.Task, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	created := make([]domain.Task, 0, len(in.Proposals))
	err = uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error {
		createTask := NewCreateTask(tasks)
		addEdge := NewAddEdge(&passthroughTxRunner{tasks: tasks, edges: edges}) // reuses AddEdge's auto-block logic within the SAME already-open tx
		for _, p := range in.Proposals {
			task, err := createTask.Execute(ctx, CreateTaskInput{
				Title: p.Title, Description: p.Description, ParentID: in.TaskID,
				Type: p.Type, EstimatedHours: p.EstimatedHours, PromptTemplate: p.PromptTemplate,
			})
			if err != nil {
				return apperrors.New(apperrors.KindInternal, "TASK_AI_APPLY_FAILED", "failed to create subtask from AI proposal", err)
			}
			if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: in.TaskID, ToTaskID: task.ID, Kind: domain.EdgeKindParentChild}); err != nil {
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
				if _, err := addEdge.Execute(ctx, AddEdgeInput{FromTaskID: created[i].ID, ToTaskID: created[depIdx].ID, Kind: domain.EdgeKindDependsOn}); err != nil {
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
```

`AddEdge`'s constructor (per `TASK-TG-01-07`) now takes a `TxRunner`, not a
bare `EdgeRepository` — inside `AIApply`'s own already-open transaction,
wrap the transaction-scoped `tasks`/`edges` repos in a trivial
`passthroughTxRunner` so `AddEdge.Execute`'s internal `RunInTx` call reuses
the SAME transaction rather than opening a nested one:

```go
// passthroughTxRunner adapts an already-open transaction's TaskRepository/
// EdgeRepository pair into the TxRunner shape AddEdge expects, so
// AIApply's own transaction is reused rather than nested — pgx does not
// support nested BEGIN.
type passthroughTxRunner struct {
	tasks TaskRepository
	edges EdgeRepository
}

func (p *passthroughTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error) error {
	return fn(ctx, p.tasks, p.edges)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestAIApply -v
```

Expected: `TestAIApply_DependsOnIndices_CreatesDependsOnEdgesAfterAllSiblingsExist`
(new) asserts edge `FromTaskID`/`ToTaskID` resolve to the right created IDs;
an out-of-range index is silently skipped, not a hard failure; a mid-loop
failure during the dependency pass still rolls back the whole transaction
(extends `TestAIApply_MidLoopFailure_RollsBackEntireSubtree`); existing
create+parent-edge tests still pass unchanged.
