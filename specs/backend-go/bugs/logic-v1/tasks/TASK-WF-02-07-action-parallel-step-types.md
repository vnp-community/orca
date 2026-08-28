# TASK-WF-02-07: Implement `action` and `parallel` step types

**From Solution:** SOL-WF-02
**Priority:** P2
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/step.go`
**Depends on:** TASK-WF-02-01, TASK-WF-02-02
**Status:** `[x]` DONE — `StepTypeAction`/`StepTypeParallel` + `ActionStepConfig`/`ParallelStepConfig` added to `step.go`, `StepType.Valid()` extended to seven types. New `stepexecutors.ActionExecutor` (map[string]ActionHandler, empty at construction, returns `usecase.ErrNoActionHandlerRegistered` for any name) and `stepexecutors.ParallelExecutor` (allSettled fan-out via the injected `StepExecutorRegistry`, two-phase `SetRegistry` init since it's one of the registry's own entries). Aggregate output uses a small `subStepOutcome` struct (status+output+error per sub-step) rather than the pseudocode's raw `StepResult` — carries the failure reason too, still satisfies "failed sub-step's outcome still present." Wired into `cmd/server/main.go`'s registry and `grpc.toDomainStepType`. New tests: `action_test.go` (unregistered->typed error, registered handler runs, invalid config JSON) + `parallel_test.go` (all-succeed, one-fails-no-partial->aggregate fails but every sub-step still ran assert via call-count, one-fails-allow-partial->aggregate completes with the failure visible, hard executor error counts as failure, unresolved sub-step type counts as failure) — 8/8 pass under `-race`. `go build/vet/test ./... -race` green for workflow-service; api-gateway still builds.

---

## Context

BUG-WF-02 finds only 5 of 6 `StepType`s implemented — `action` and
`parallel` are absent entirely. Neither is described in detail by
`workflow-service.md` §4, so this wires the minimal, extensible type
system rather than inventing a concrete action catalog.

## Changes to make

In `step.go`, add:

```go
const (
    StepTypeAction   StepType = "action"
    StepTypeParallel StepType = "parallel"
)

// ActionStepConfig dispatches to a named, in-process action handler. No
// handlers are registered by this task itself — this wires the type
// system so `action` steps are recognized and fail with a clear, typed
// error rather than silently no-op-ing.
type ActionStepConfig struct {
    ActionName string          `json:"actionName"`
    Params     json.RawMessage `json:"params,omitempty"`
}

// ParallelStepConfig fans SubSteps out concurrently and aggregates their
// results (Promise.allSettled + allowPartialFailure semantics). SubSteps'
// own DependsOn is ignored — sub-steps always run together in one
// fan-out, not wave-computed among themselves.
type ParallelStepConfig struct {
    SubSteps            []Step `json:"subSteps"`
    AllowPartialFailure  bool   `json:"allowPartialFailure,omitempty"`
}
```

Update `StepType.Valid()` (`step.go:26-33`) to accept both new values.

Create `backend-go/services/workflow-service/internal/adapter/stepexecutors/parallel.go`:

```go
package stepexecutors

// ParallelExecutor needs the SAME StepExecutorRegistry the wave
// dispatcher uses, to recursively resolve each sub-step's own executor —
// injected at construction (main.go wires it after the registry itself
// is built, a two-phase init since ParallelExecutor IS one of the
// registry's own entries).
type ParallelExecutor struct {
    registry usecase.StepExecutorRegistry
}

func (e *ParallelExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
    var cfg domain.ParallelStepConfig
    json.Unmarshal([]byte(stepConfigJSON), &cfg)

    results := make([]domain.StepResult, len(cfg.SubSteps))
    errs := make([]error, len(cfg.SubSteps))
    var wg sync.WaitGroup
    for i, sub := range cfg.SubSteps {
        wg.Add(1)
        go func(i int, sub domain.Step) {
            defer wg.Done()
            executor, err := e.registry.Resolve(sub.Type)
            if err != nil {
                errs[i] = err
                return
            }
            results[i], errs[i] = executor.Execute(ctx, string(sub.Config)) // allSettled
        }(i, sub)
    }
    wg.Wait()

    anyFailed := false
    agg := make(map[string]any, len(cfg.SubSteps))
    for i, sub := range cfg.SubSteps {
        if errs[i] != nil || results[i].Status == domain.ResultStatusFailed {
            anyFailed = true
        }
        agg[sub.ID] = results[i]
    }
    if anyFailed && !cfg.AllowPartialFailure {
        return domain.StepResult{Status: domain.ResultStatusFailed}, fmt.Errorf("stepexecutors: parallel: one or more sub-steps failed")
    }
    outputJSON, _ := json.Marshal(agg)
    return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: string(outputJSON)}, nil
}
```

Create `backend-go/services/workflow-service/internal/adapter/stepexecutors/action.go`
holding a `map[string]ActionHandler` (empty at construction) and
returning a new `usecase.ErrNoActionHandlerRegistered` sentinel for any
unregistered `ActionName`.

Register both executors in `cmd/server/main.go`'s `StepExecutorRegistry`
construction, and add `STEP_TYPE_ACTION`/`STEP_TYPE_PARALLEL` handling
wherever the proto `StepType` enum is mapped to `domain.StepType`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/adapter/stepexecutors/... -race
```

Expected: all sub-steps succeed → aggregate completed; one sub-step fails
+ `AllowPartialFailure=false` → aggregate failed but EVERY sub-step still
ran (assert via a fake executor's call count); one sub-step fails +
`AllowPartialFailure=true` → aggregate completed, failed sub-step's
outcome still present. Unregistered `ActionName` returns
`ErrNoActionHandlerRegistered`, never a panic.
