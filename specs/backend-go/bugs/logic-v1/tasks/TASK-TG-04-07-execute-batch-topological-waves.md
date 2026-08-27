# TASK-TG-04-07: `ExecuteBatch` — topological-wave batch execution

**From Solution:** SOL-TG-04
**Priority:** P2
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/topological_waves.go` (new), `backend-go/services/task-service/internal/usecase/execute_batch.go` (new)
**Depends on:** TASK-TG-04-03 (`ExecuteTask` real dispatch), TASK-TG-02-02 (`buildDependsOnGraph` reuse)
**Status:** `[ ]` TODO

---

## Context

Spec calls for batch execution with "limited parallelism" over a set of
tasks, grouped into waves of mutually-independent tasks (siblings within a
wave run concurrently; a wave only starts once every task in the previous
wave has finished). `{{outputs.<taskId>.*}}` prompt interpolation needs
each task's last stdout retained somewhere readable by a later wave's
prompt build — this solution proposes a bounded `last_execution_output
TEXT` column, flagged as a product-tradeoff decision (full output vs.
truncated) worth confirming before implementation.

## Changes to make

Add a small migration,
`backend-go/services/task-service/migrations/0006_last_execution_output.up.sql`:

```sql
-- Bounded to 8KB via application-layer truncation (not a DB CHECK
-- constraint, since the truncation point is a product tradeoff — see this
-- task's Context note) before writing.
ALTER TABLE task.tasks ADD COLUMN last_execution_output TEXT;
```

`.down.sql`:

```sql
ALTER TABLE task.tasks DROP COLUMN last_execution_output;
```

Create `backend-go/services/task-service/internal/domain/topological_waves.go`:

```go
package domain

// TopologicalWaves groups taskIDs into waves of mutually-independent tasks
// over the depends_on DAG, scoped to ONLY the given taskIDs (edges pointing
// outside that set are ignored) — reuses buildDependsOnGraph's adjacency
// build from critical_path.go rather than a second graph-construction
// implementation. A task with no dependency among the batch set is wave 0;
// each subsequent wave contains every task whose depends_on targets are
// all in an earlier wave.
func TopologicalWaves(edges []TaskEdge, taskIDs []string) [][]string {
	inSet := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		inSet[id] = true
	}
	scoped := make([]TaskEdge, 0, len(edges))
	for _, e := range edges {
		if e.Kind == EdgeKindDependsOn && inSet[e.FromTaskID] && inSet[e.ToTaskID] {
			scoped = append(scoped, e)
		}
	}

	remainingDeps := make(map[string]map[string]bool, len(taskIDs)) // taskID -> set of not-yet-satisfied dependency IDs
	for _, id := range taskIDs {
		remainingDeps[id] = map[string]bool{}
	}
	for _, e := range scoped {
		remainingDeps[e.FromTaskID][e.ToTaskID] = true
	}

	var waves [][]string
	placed := map[string]bool{}
	for len(placed) < len(taskIDs) {
		var wave []string
		for _, id := range taskIDs {
			if placed[id] || len(remainingDeps[id]) > 0 {
				continue
			}
			wave = append(wave, id)
		}
		if len(wave) == 0 {
			break // defensive: a cycle slipped through (DetectCycle is the real enforcement point)
		}
		for _, id := range wave {
			placed[id] = true
		}
		for _, deps := range remainingDeps {
			for _, id := range wave {
				delete(deps, id)
			}
		}
		waves = append(waves, wave)
	}
	return waves
}
```

Create `backend-go/services/task-service/internal/usecase/execute_batch.go`:

```go
package usecase

import (
	"context"
	"sync"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ExecuteBatchInput struct {
	TaskIDs       []string
	MaxConcurrency int
	StopOnFailure  bool
	RequestID      string
}

type ExecuteBatchResult struct {
	Completed []string
	Failed    map[string]error
}

// ExecuteBatch dispatches a set of tasks in dependency-respecting waves —
// siblings within a wave run with bounded concurrency; a wave only starts
// once every task in the previous wave has finished.
type ExecuteBatch struct {
	edges       EdgeRepository
	executeTask *ExecuteTask
}

func NewExecuteBatch(edges EdgeRepository, executeTask *ExecuteTask) *ExecuteBatch {
	return &ExecuteBatch{edges: edges, executeTask: executeTask}
}

func (uc *ExecuteBatch) Execute(ctx context.Context, in ExecuteBatchInput) (ExecuteBatchResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ExecuteBatchResult{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	allDeps, err := uc.edges.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn)
	if err != nil {
		return ExecuteBatchResult{}, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_BATCH_EDGE_LOOKUP_FAILED", "failed to list dependency edges", err)
	}
	waves := domain.TopologicalWaves(allDeps, in.TaskIDs)

	maxConcurrency := in.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // config value in production wiring, not hardcoded — see main.go
	}

	result := ExecuteBatchResult{Failed: map[string]error{}}
	for _, wave := range waves {
		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, taskID := range wave {
			wg.Add(1)
			sem <- struct{}{}
			go func(taskID string) {
				defer wg.Done()
				defer func() { <-sem }()
				_, err := uc.executeTask.Execute(ctx, ExecuteTaskInput{TaskID: taskID, RequestID: in.RequestID})
				mu.Lock()
				if err != nil {
					result.Failed[taskID] = err
				} else {
					result.Completed = append(result.Completed, taskID)
				}
				mu.Unlock()
			}(taskID)
		}
		wg.Wait()
		if in.StopOnFailure && len(result.Failed) > 0 {
			return result, apperrors.New(apperrors.KindInternal, "TASK_EXECUTE_BATCH_STOPPED", "batch execution stopped after a wave failure", nil)
		}
	}
	return result, nil
}
```

`{{outputs.<taskId>.*}}` interpolation itself (reading a prior wave's
`last_execution_output` into a later wave's prompt) is a `buildExecutePrompt`
extension — add a `PriorOutputs map[string]string` parameter there (from
`TASK-TG-04-06`) once this task's `last_execution_output` column exists, and
have `ExecuteTask`'s simple-path completion write persist truncated
(8KB) stdout to it via `CompleteExecution`'s widened signature. Scoping
that wiring precisely is left to implementation time since it touches
`TASK-TG-04-06`'s already-landed `buildExecutePrompt` shape — note the
dependency, don't silently skip it.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/domain/... -run TestTopologicalWaves -v
go test ./services/task-service/internal/usecase/... -run TestExecuteBatch -v
```

Expected: `topological_waves_test.go` — a diamond dependency graph produces
the correct wave grouping (independent siblings in one wave, not
serialized); a task with no dependency among the batch set is wave 0.
`execute_batch_test.go` — a wave's tasks dispatch concurrently (bounded by
`MaxConcurrency`); `StopOnFailure=true` halts before the next wave once any
task in the current wave fails.
