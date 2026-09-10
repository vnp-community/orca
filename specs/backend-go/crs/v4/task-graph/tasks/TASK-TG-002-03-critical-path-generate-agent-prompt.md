# TASK-TG-002-03: `CalculateCriticalPath` (pure domain function) + `GenerateAgentPrompt` RPC

**From Solution:** BE-SOL-002
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/critical_path.go` (new), `backend-go/services/task-service/internal/usecase/generate_agent_prompt.go` (new), `backend-go/proto/orca/task/v1/task.proto` (`GenerateAgentPrompt` RPC), `backend-go/services/task-service/internal/adapter/grpc/server.go` (handler)
**Depends on:** TASK-TG-001-02 (`Task.EstimatedHours`, `Task.PromptTemplate` fields)
**Status:** `[ ]` TODO

---

## Context

Both pieces are entirely new — confirmed, `grep -rn "CriticalPath\|GenerateAgentPrompt"
backend-go/services/task-service/` finds nothing today. BE-SOL-002 names
`workflow-service`'s `DAG.BuildWaves` (per `workflow-service.md` §4) as the
"same primitive style," reused conceptually only — this task does not
import anything from `workflow-service` (separate microservice, separate
Go module boundary; a shared package would need to live in `common/` if
ever de-duplicated, out of scope here).

`GenerateAgentPrompt` is a **separate flow from `AIDecompose`** — same
`AICompleter` relay primitive, different prompt template and output target
(`Task.PromptTemplate`, which TASK-TG-005-03's `SimpleExecutor.buildExecutePrompt`
reads).

## Changes to make

**1. `internal/domain/critical_path.go`** (new, pure function — zero
imports outside stdlib, per `domain/`'s package-boundary rule stated in
`task.go:1-4`'s own header comment):

```go
package domain

// CalculateCriticalPath returns the longest-duration chain of depends_on
// edges through tasks, by EstimatedHours — Kahn's-algorithm topological
// sort + longest-path DP. Tasks with a nil EstimatedHours are treated as
// zero-duration for this calculation (a task with no estimate contributes
// nothing to path length, but is not excluded from the graph — cutting it
// out entirely could silently break a real depends_on chain through it).
func CalculateCriticalPath(tasks []Task, edges []TaskEdge) []string {
	// 1. Build adjacency (depends_on only) + in-degree map.
	// 2. Kahn's algorithm: process zero-in-degree nodes, decrementing
	//    successors' in-degree as each is processed — standard cycle-free
	//    topological order (a real cycle should never reach this function,
	//    since AddEdge/AIApply both reject cycles at write time; if one
	//    somehow exists, return nil rather than looping forever — assert
	//    processed-count == len(tasks) before trusting the order).
	// 3. Longest-path DP over the topological order: dist[node] =
	//    max(dist[dep] + dep.EstimatedHours) across all incoming depends_on
	//    edges, 0 for a node with no dependencies.
	// 4. Walk back from the max-dist node to reconstruct the path.
}
```

(Implementation body intentionally left as pseudocode above — BE-SOL-002
doesn't specify tie-breaking for multiple equal-length paths; pick the
first-discovered by topological order for determinism, and cover it with a
test asserting the SAME path on repeated calls against the same input.)

**2. `internal/usecase/generate_agent_prompt.go`** (new):

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type GenerateAgentPromptInput struct {
	TaskID string
}

// GenerateAgentPrompt produces a Task.PromptTemplate string for one task —
// same ai.complete relay primitive as AIDecompose (AICompleter), a
// different prompt template (single-task instructions, not a subtask
// breakdown). Writes the result to Task.PromptTemplate, which
// SimpleExecutor.buildExecutePrompt (TASK-TG-005-03) later reads.
type GenerateAgentPrompt struct {
	tasks    TaskRepository
	resolver ProjectExecutionResolver
	relay    AICompleter
}

func NewGenerateAgentPrompt(tasks TaskRepository, resolver ProjectExecutionResolver, relay AICompleter) *GenerateAgentPrompt {
	return &GenerateAgentPrompt{tasks: tasks, resolver: resolver, relay: relay}
}

func (uc *GenerateAgentPrompt) Execute(ctx context.Context, in GenerateAgentPromptInput) (string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return "", apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	connectionID, _, connected, err := uc.resolver.ResolveConnection(ctx, tenantID, task.ProjectID)
	if err != nil || !connected {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "TASK_GENERATE_PROMPT_NO_CONNECTION", "task's project has no connected dev server for AI relay", err)
	}
	prompt := buildAgentPromptRequest(task) // distinct template from buildDecomposePrompt/buildExecutePrompt
	result, err := uc.relay.Complete(ctx, connectionID, prompt)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_FAILED", "failed to generate agent prompt via AI relay", err)
	}
	if err := uc.tasks.Update(ctx, tenantID, withPromptTemplate(task, result)); err != nil {
		return "", apperrors.New(apperrors.KindInternal, "TASK_GENERATE_PROMPT_SAVE_FAILED", "failed to persist generated prompt template", err)
	}
	return result, nil
}
```

(`withPromptTemplate` is a small local helper copying `task` with
`PromptTemplate` set — `TaskRepository.Update`'s exact partial-update
semantics need a direct check of `repository.go`'s `Update` method before
finalizing whether it overwrites the whole row or only specific fields;
match whatever `UpdateTask`'s usecase already does for a single-field
change, don't invent a second convention.)

**3. Proto + handler** — add
`rpc GenerateAgentPrompt(GenerateAgentPromptRequest) returns (GenerateAgentPromptResponse);`
(request: `task_id`; response: `prompt_template`) to `task.proto`, handler
in `server.go` following the `AIDecompose` pattern
(`internal/adapter/grpc/server.go:210-217`).

`CalculateCriticalPath` has no RPC of its own per BE-SOL-002's design — it's
a pure function; wire it into whichever read path FE-SOL-001 actually
consumes (check that solution before adding a dedicated RPC — BE-SOL-002
doesn't specify one, and this task should not invent wire surface beyond
what's cited).

## Test plan

- `CalculateCriticalPath` on a 5-node DAG with 2 branches — longest path by
  hours matches hand-computed expectation.
- `CalculateCriticalPath` is deterministic across repeated calls on
  identical input (tie-break test).
- `GenerateAgentPrompt` snapshot test once BE-SOL-001's fields are wired;
  asserts `Task.PromptTemplate` is persisted after a successful call.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/domain/... -run TestCalculateCriticalPath -v
go test ./services/task-service/internal/usecase/... -run TestGenerateAgentPrompt -v
```

Expected: clean build; critical-path test matches hand-computed expected
path and total hours; `GenerateAgentPrompt` persists the AI-generated
template and returns it in the same call.
