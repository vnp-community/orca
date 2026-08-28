# TASK-TG-04-04: Real `ComplexExecutor` — add `StartCoordinatorRun` to `orchestration.proto`, dial `orchestration-service`

**From Solution:** SOL-TG-04
**Priority:** P1
**Service:** `task-service` (client) + `orchestration-service` (new RPC — **larger gap than the solution assumed**: `orchestration.proto` currently has NO run-creation RPC at all, not even a stub)
**File:** `backend-go/proto/orca/orchestration/v1/orchestration.proto`, `backend-go/services/task-service/internal/adapter/grpcclient/complex_executor.go`
**Depends on:** TASK-TG-01-06 (`GetSubtree` for `buildOrchestrationSpec`)
**Status:** [x] DONE — Added StartCoordinatorRun RPC + OrchestrationTaskSpec/StartCoordinatorRunRequest/Response to orchestration.proto (regenerated); real ComplexExecutor (grpcclient) dials orchestration-service and translates task-service's subtree into a temp-id-based DAG spec via buildOrchestrationSpec; ComplexExecutor port signature widened to take worktreeID; wired into main.go replacing StubComplexExecutor (kept as a fallback type). Flagged dependency: orchestration-service's own StartCoordinatorRun handler is out of scope here — calling it before that handler lands fails at dial/call time, not compile time. `go build ./proto/... ./services/task-service/...` and every other backend-go service clean; `go test ./services/task-service/internal/adapter/grpcclient/... -run TestComplexExecutor` passes (3/3: spec-building with correct temp-id deps, subtree-fetch-error-never-calls-RPC, RPC-error-propagates).

---

## Context

`StubComplexExecutor` returns a synthesized ref without calling anything.
SOL-TG-04 phrases the proto work as "if not already generated" — checking
the actual current `orchestration.proto`, it is NOT generated:
`OrchestrationService` today only has `CreateDispatchContext`, `CreateGate`,
`ResolveGate`, `UpdateTaskStatusAndPromote`, `GetDispatchContextForTask` —
no RPC creates a `coordinator_run` at all. This task adds
`StartCoordinatorRun` to `orchestration.proto` (server-side handler is
`orchestration-service`'s own scope — out of this task, flagged as a
dependency this client-side change assumes will land) and implements the
real `ComplexExecutor` client against it.

## Changes to make

Add to `backend-go/proto/orca/orchestration/v1/orchestration.proto`'s
`OrchestrationService` service block:

```protobuf
  // StartCoordinatorRun is task-service's entry point into the complex
  // execution path (task-service.md §3.1/§7): starts a coordinator_run for
  // a subtree of tasks and returns immediately — this call does NOT block
  // for the DAG to finish. orchestration-service calls back into
  // task-service (ReportTaskExecutionResult) to report the terminal
  // result; it never blocks task-service synchronously for it.
  rpc StartCoordinatorRun(StartCoordinatorRunRequest) returns (StartCoordinatorRunResponse);
```

```protobuf
message OrchestrationTaskSpec {
  string temp_id = 1;         // task-service's own Task.ID, carried as a reference — NOT the primary key here (distinct id spaces, §2.1)
  string title = 2;
  string prompt = 3;          // description or prompt_template, whichever the caller resolved
  repeated string deps = 4;   // temp_ids of sibling nodes in THIS SAME request this node depends on
}

message StartCoordinatorRunRequest {
  string tenant_id = 1;
  string origin_task_id = 2;  // task-service's root Task.ID for this dispatch
  string worktree_id = 3;
  repeated OrchestrationTaskSpec tasks = 4;
}
message StartCoordinatorRunResponse {
  string id = 1; // the new coordinator_run's id — task-service's "logical FK" (active_execution_id)
}
```

**Flagged dependency**: this task only adds the client-facing RPC shape and
the `task-service`-side caller. `orchestration-service`'s own handler
(persisting a `coordinator_runs` row, minting real `orchestration_tasks`
rows per `OrchestrationTaskSpec`, and starting its state-machine ticking)
is `orchestration-service`'s own implementation work, not covered here —
confirm it lands (or is scheduled) before wiring `ComplexExecutor` into
`task-service`'s production composition root, since calling an RPC with no
server implementation fails at dial/call time, not compile time.

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
```

Replace `backend-go/services/task-service/internal/adapter/grpcclient/complex_executor.go`:

```go
package grpcclient

import (
	"context"
	"fmt"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

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

// Execute's signature gains worktreeID (resolved by ExecuteTask's own
// worktree reuse-or-create step, TASK-TG-04-03, before dispatch) — update
// usecase.ComplexExecutor's port signature to
// Execute(ctx, tenantID, taskID, requestID, worktreeID string) (string, error)
// and ExecuteTask's call site to pass it through.
func (c *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (string, error) {
	spec, err := c.buildOrchestrationSpec(ctx, tenantID, taskID)
	if err != nil {
		return "", fmt.Errorf("complex_executor: build spec: %w", err)
	}
	resp, err := c.orch.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
		TenantId: tenantID, OriginTaskId: taskID, WorktreeId: worktreeID, Tasks: spec,
	})
	if err != nil {
		return "", fmt.Errorf("complex_executor: start coordinator run: %w", err)
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

Wire `ComplexExecutor` into
`backend-go/services/task-service/cmd/server/main.go`, replacing
`taskgrpcclient.NewStubComplexExecutor()`:

```go
orchConn, err := taskgrpcclient.Dial(cfg.OrchestrationServiceAddr)
if err != nil {
	return fmt.Errorf("dialing orchestration-service: %w", err)
}
defer func() { _ = orchConn.Close() }()
orchClient := orchestrationv1.NewOrchestrationServiceClient(orchConn)
complexExecutor := taskgrpcclient.NewComplexExecutor(orchClient, repo, repo)
```

Add `OrchestrationServiceAddr string` to task-service's config if not
already present.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/... ./services/task-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestComplexExecutor -v
```

Expected: `complex_executor_test.go` — fake `OrchestrationServiceClient`:
`buildOrchestrationSpec` produces one spec node per subtree task with
correctly-translated `deps` (temp-id based, not raw task-service IDs
re-used as orchestration primary keys); a subtree fetch error never reaches
`StartCoordinatorRun`.
