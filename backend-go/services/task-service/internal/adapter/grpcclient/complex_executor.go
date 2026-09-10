package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
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

// ComplexExecutor implements usecase.ComplexExecutor for real (the
// BE-SOL-002 integration addendum), replacing the prior
// StubComplexExecutor — dispatches Execute's complex path to
// orchestration-service's real StartCoordinatorRun RPC
// (TASK-TASKV1-005-05), which itself starts the autonomous tick-loop
// coordinator (TASK-TASKV1-005-10) that later reports back via
// ReportTaskExecutionResult (TASK-FT-002-04).
//
// tasks/edges are task-service's OWN in-process TaskRepository/
// EdgeRepository — not a second gRPC hop back into this same service — the
// same pattern SimpleExecutor already uses for TaskRepository (see that
// type's doc comment).
//
// Wire shape note: orchestration.proto's StartCoordinatorRunRequest carries
// spec_json (a string), not a structured task list — this adapter's job is
// producing that JSON blob, not a proto message list. worktreeID is
// ExecuteTask's own worktree reuse-or-create result (TASK-TG-04-02/03),
// threaded straight through rather than re-read off task.WorktreeID here,
// since the caller already resolved the authoritative value for this
// dispatch.
//
// Spec-building simplification (flagged, not silently assumed): task.proto
// has no GetSubtree RPC and GetDependencies only returns one level of
// depends_on edges FROM a task — reusing either doesn't give a full
// recursive subtree cheaply. Rather than inventing recursive expansion
// here, buildSpec below covers ONE level of parent_child children only
// (matching selectEngine's own one-level ListFrom check that routed this
// task to Engine 2 in the first place): the root task-1 node plus one
// SpecNode per direct child, with the root's Deps listing every child
// tempID — i.e. orchestration-service considers task-1 "done" once every
// direct child completes. depends_on edges (the other trigger for Engine
// 2) are NOT expanded into extra DAG nodes: a depends_on target is a
// pre-existing, independently-tracked task-service Task, not new work this
// dispatch should re-run, so it's used only for selectEngine's routing
// decision, never materialized into spec_json. Grandchild subtasks (a
// child that itself has children) are also not recursively expanded — the
// same one-level limit. Deeper/recursive decomposition is an honest,
// documented gap for follow-up work, not a silent guess.
type ComplexExecutor struct {
	tasks         usecase.TaskRepository
	edges         usecase.EdgeRepository
	orchestration orchestrationv1.OrchestrationServiceClient
}

func NewComplexExecutor(tasks usecase.TaskRepository, edges usecase.EdgeRepository, orchestration orchestrationv1.OrchestrationServiceClient) *ComplexExecutor {
	return &ComplexExecutor{tasks: tasks, edges: edges, orchestration: orchestration}
}

// specNode mirrors orchestration-service's domain.SpecNode — the CLOSED
// wire contract between task-service's spec-building and that service's
// ExpandSpec (see orchestration-service/internal/domain/spec.go's doc
// comment). Kept as a private mirror rather than importing that service's
// internal/domain package, per Clean Architecture's service-boundary rule
// (task-service and orchestration-service share nothing but their proto).
type specNode struct {
	TempID string          `json:"tempId"`
	Title  string          `json:"title"`
	Spec   json.RawMessage `json:"spec"`
	Deps   []string        `json:"deps,omitempty"`
}

// Execute translates task-service's subtree into orchestration-service's
// spec_json DAG shape and starts a coordinator_run. worktreeID is resolved
// by ExecuteTask's own worktree reuse-or-create step (TASK-TG-04-02/03)
// before this is ever called.
func (c *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
	task, err := c.tasks.Get(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("complex_executor: load task: %w", err)
	}

	specJSON, err := c.buildSpec(ctx, tenantID, task)
	if err != nil {
		return "", fmt.Errorf("complex_executor: build spec: %w", err)
	}

	ctx, err = withTenantMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := c.orchestration.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
		OriginTaskId: task.ID,
		SpecJson:     string(specJSON),
		WorktreeId:   worktreeID,
	})
	if err != nil {
		return "", fmt.Errorf("complex_executor: start coordinator run: %w", err)
	}
	// Record the new run's id as this task's active_execution_id — an
	// informational mirror of the dispatch (the staleness/idempotence check
	// itself now runs through ReportTaskExecutionResult's execution_links
	// comparison, TASK-FT-002-04, not this field).
	if err := c.tasks.UpdateActiveExecutionID(ctx, tenantID, taskID, resp.GetId()); err != nil {
		return "", fmt.Errorf("complex_executor: persist active_execution_id: %w", err)
	}
	return resp.GetId(), nil
}

// buildSpec assembles the [{tempId,title,spec,deps}] array
// domain.ExpandSpec (orchestration-service) requires, root node first
// (index 0) per that function's own convention — see this type's doc
// comment for the one-level-only simplification.
func (c *ComplexExecutor) buildSpec(ctx context.Context, tenantID string, task domain.Task) ([]byte, error) {
	children, err := c.edges.ListFrom(ctx, tenantID, task.ID, domain.EdgeKindParentChild)
	if err != nil {
		return nil, fmt.Errorf("list child edges: %w", err)
	}

	childTempIDs := make([]string, 0, len(children))
	nodes := make([]specNode, 0, len(children)+1)
	for _, edge := range children {
		childTask, err := c.tasks.Get(ctx, tenantID, edge.ToTaskID)
		if err != nil {
			return nil, fmt.Errorf("load child task %s: %w", edge.ToTaskID, err)
		}
		nodes = append(nodes, specNode{TempID: childTask.ID, Title: childTask.Title, Spec: json.RawMessage(`{}`)})
		childTempIDs = append(childTempIDs, childTask.ID)
	}

	root := specNode{TempID: task.ID, Title: task.Title, Spec: json.RawMessage(`{}`), Deps: childTempIDs}
	nodes = append([]specNode{root}, nodes...)

	out, err := json.Marshal(nodes)
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}
	return out, nil
}
