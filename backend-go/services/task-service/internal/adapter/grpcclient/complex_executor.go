package grpcclient

import (
	"context"
	"fmt"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// StubComplexExecutor implements usecase.ComplexExecutor as a stub that
// returns a synthesized execution reference without calling
// orchestration-service. Superseded in production wiring by ComplexExecutor
// below (TASK-TG-04-04) — kept for now as a fallback for any environment
// where orchestration-service's StartCoordinatorRun handler (its own scope,
// not covered by TASK-TG-04-04) hasn't landed yet.
type StubComplexExecutor struct{}

func NewStubComplexExecutor() *StubComplexExecutor {
	return &StubComplexExecutor{}
}

func (s *StubComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
	return fmt.Sprintf("stub-orchestration-exec:%s:%s", taskID, requestID), nil
}

// ComplexExecutor implements usecase.ComplexExecutor for real, replacing
// StubComplexExecutor — dispatches to orchestration-service's coordinator.
// orchestration-service calls back into task-service
// (ReportTaskExecutionResult, TASK-TG-04-05) to report the terminal
// result; this call itself returns immediately (StartCoordinatorRun does
// not block for the DAG to finish).
type ComplexExecutor struct {
	orch  orchestrationv1.OrchestrationServiceClient
	tasks usecase.TaskRepository
	edges usecase.EdgeRepository
}

func NewComplexExecutor(orch orchestrationv1.OrchestrationServiceClient, tasks usecase.TaskRepository, edges usecase.EdgeRepository) *ComplexExecutor {
	return &ComplexExecutor{orch: orch, tasks: tasks, edges: edges}
}

// Execute translates task-service's subtree into orchestration-service's
// DAG shape and starts a coordinator_run. worktreeID is resolved by
// ExecuteTask's own worktree reuse-or-create step (TASK-TG-04-03) before
// this is ever called.
func (c *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
	spec, err := c.buildOrchestrationSpec(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("complex_executor: build spec: %w", err)
	}
	ctx, err = withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := c.orch.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
		TenantId: tenantID, OriginTaskId: taskID, WorktreeId: worktreeID, Tasks: spec,
	})
	if err != nil {
		return "", fmt.Errorf("complex_executor: start coordinator run: %w", err)
	}
	// Record the new run's id as this task's active_execution_id — the
	// staleness check ReportTaskExecutionResult (TASK-TG-04-05) needs to
	// reject a callback for a run this task was re-dispatched away from.
	if err := c.tasks.UpdateActiveExecutionID(ctx, tenantID, taskID, resp.GetId()); err != nil {
		return "", fmt.Errorf("complex_executor: persist active_execution_id: %w", err)
	}
	return resp.GetId(), nil
}

// buildOrchestrationSpec translates task-service's own subtree
// (parent_child + depends_on edges, this task's own id space) into
// orchestration-service's OrchestrationTaskSpec DAG shape — the two
// services deliberately use distinct id spaces (orchestration-service.md
// §2.1), so this function's whole job is producing a closed temp-id set
// from a task-service subtree. Uses TASK-TG-01-05's GetSubtree
// (usecase.TaskRepository.GetSubtree(ctx, tenantID, rootID, maxDepth)
// ([]domain.Task, []domain.TaskEdge, error)) directly.
func (c *ComplexExecutor) buildOrchestrationSpec(ctx context.Context, tenantID, rootTaskID string) ([]*orchestrationv1.OrchestrationTaskSpec, error) {
	subtree, edges, err := c.tasks.GetSubtree(ctx, tenantID, rootTaskID, 0)
	if err != nil {
		return nil, fmt.Errorf("complex_executor: fetch subtree: %w", err)
	}

	depsByFrom := map[string][]string{}
	for _, e := range edges { // edges is already depends_on-only, per GetSubtree's contract
		depsByFrom[e.FromTaskID] = append(depsByFrom[e.FromTaskID], e.ToTaskID)
	}

	out := make([]*orchestrationv1.OrchestrationTaskSpec, 0, len(subtree))
	for _, t := range subtree {
		prompt := t.PromptTemplate
		if prompt == "" {
			prompt = t.Description
		}
		out = append(out, &orchestrationv1.OrchestrationTaskSpec{
			TempId: t.ID, Title: t.Title, Prompt: prompt, Deps: depsByFrom[t.ID],
		})
	}
	return out, nil
}
