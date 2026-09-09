# TASK-WF-003-03: `parallel` step type

**From Solution:** BE-SOL-003
**Priority:** P1
**Service:** `workflow-service`
**File:** `backend-go/proto/orca/workflow/v1/workflow.proto` (`StepType` enum), `backend-go/services/workflow-service/internal/domain/step.go` (`StepTypeParallel`, `ParallelStepConfig`), `backend-go/services/workflow-service/internal/usecase/wave_dispatcher.go`, `backend-go/services/workflow-service/internal/adapter/grpc/server.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_automation_task.go`
**Depends on:** TASK-WF-003-02 (shares the same `workflow.proto` `StepType` enum edit — land together or in quick succession to avoid enum-value churn)
**Status:** `[ ]` TODO

---

## Context — one naming correction versus BE-SOL-003's sketch

BE-SOL-003 sketches the dispatch as `a new case in executeStep()`. Re-read
`wave_dispatcher.go` directly: **there is no function named
`executeStep`**. The real per-step dispatch function is `runStep`
(`internal/usecase/wave_dispatcher.go:199-214`):

```go
func (d *waveDispatcher) runStep(ctx context.Context, step domain.Step, se *domain.StepExecution) (domain.StepResult, error) {
	executor, err := d.registry.Resolve(step.Type)
	...
	result, err := executor.Execute(ctx, string(step.Config))
	...
}
```

`runStep` currently always resolves `step.Type` through
`d.registry` (a `StepExecutorRegistry`, `ports.go:98`) and calls a single
`StepExecutor.Execute`. `parallel` is different in kind from the other
step types: it doesn't relay to an external system, it fans out to
*other steps already expressed in this same DAG's `Step` shape*
(`ParallelStepConfig.Steps []Step`) and recurses back into the same
dispatch mechanism. This task adds a `parallel`-specific branch in
`runStep` (or a thin wrapper `runStep` delegates to) rather than routing
`parallel` through the ordinary `StepExecutorRegistry.Resolve` path,
since a "sub-step fan-out" executor doesn't have the shape
`StepExecutor.Execute(ctx, stepConfigJSON string) (StepResult, error)`
naturally supports without reaching back into `waveDispatcher` itself
(it needs to call `runStep` recursively, not just relay JSON somewhere).

This is distinct from the wave-level parallelism `wave_dispatcher.go`
already does across independent DAG steps
(`dispatchWave`'s per-wave goroutine fan-out, `:150-160`, driven by
`domain.DAGDefinition.BuildWaves`, `dag.go:114`) — `parallel` is a single
step's own nested fan-out, a different mechanism operating one level
down, confirmed unchanged by this task.

## Changes to make

**1. `workflow.proto`** — add the enum value (coordinate the exact next
number with TASK-WF-003-02, whichever of the two lands first claims the
lower number):

```protobuf
enum StepType {
  ...
  STEP_TYPE_ACTION = 6;
  STEP_TYPE_PARALLEL = 7; // NEW
}
```

**2. `internal/domain/step.go`**:

```go
const (
	...
	StepTypeParallel StepType = "parallel" // NEW
)

func (t StepType) Valid() bool {
	switch t {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition, StepTypeAction, StepTypeParallel:
		return true
	default:
		return false
	}
}

// ParallelStepConfig is the Parallel step type's config shape — a single
// step's own nested fan-out to sub-steps, dispatched directly by
// waveDispatcher.runStep (not through StepExecutorRegistry, since it
// needs to recurse back into runStep itself for each sub-step).
type ParallelStepConfig struct {
	Steps               []Step `json:"steps"`
	AllowPartialFailure bool   `json:"allowPartialFailure"`
}
```

**3. `internal/adapter/grpc/server.go`**'s `toDomainStepType`
(`:184-198`) — add `case workflowv1.StepType_STEP_TYPE_PARALLEL: return
domain.StepTypeParallel`. `api-gateway`'s
`channels_automation_task.go:49` `parseStepType` — add the `"parallel"`
case, same as TASK-WF-003-02's `"action"` addition.

**4. `wave_dispatcher.go`'s `runStep`** — branch before the generic
registry path:

```go
func (d *waveDispatcher) runStep(ctx context.Context, step domain.Step, se *domain.StepExecution) (domain.StepResult, error) {
	if step.Type == domain.StepTypeParallel {
		return d.runParallelStep(ctx, step)
	}

	executor, err := d.registry.Resolve(step.Type)
	...
}

// runParallelStep fans out cfg.Steps concurrently, each through the SAME
// runStep this method is itself called from — a parallel step's
// sub-steps may themselves reference {{outputs.*}} from steps outside the
// parallel block, but NOT from sibling sub-steps within the same
// parallel block (no defined execution order among them to interpolate
// against). AllowPartialFailure controls whether one sub-step's failure
// fails the whole parallel step.
func (d *waveDispatcher) runParallelStep(ctx context.Context, step domain.Step) (domain.StepResult, error) {
	var cfg domain.ParallelStepConfig
	if err := json.Unmarshal(step.Config, &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("wave_dispatcher: parallel: invalid step config JSON: %w", err)
	}

	results := make([]domain.StepResult, len(cfg.Steps))
	errs := make([]error, len(cfg.Steps))
	var wg sync.WaitGroup
	for i, sub := range cfg.Steps {
		wg.Add(1)
		go func(i int, s domain.Step) {
			defer wg.Done()
			// domain.NewStepExecution(id, executionID, stepID, dispatchToken string, wave int) (StepExecution, error)
			// — confirmed signature at internal/domain/step_execution.go:55.
			se, err := domain.NewStepExecution(uuid.NewString(), executionID, s.ID, dispatchToken, wave)
			if err != nil {
				errs[i] = err
				return
			}
			results[i], errs[i] = d.runStep(ctx, s, &se)
		}(i, sub)
	}
	wg.Wait()

	anyFailed := false
	for i, r := range results {
		if errs[i] != nil || r.Status == domain.ResultStatusFailed {
			anyFailed = true
		}
	}
	if anyFailed && !cfg.AllowPartialFailure {
		return domain.StepResult{Status: domain.ResultStatusFailed}, ErrParallelStepFailed
	}
	// Aggregate sub-results into the parallel step's own OutputJSON — exact
	// shape (array of sub-results, keyed by sub-step id, etc.) is an
	// implementation choice not fixed by BE-SOL-003; pick one and document
	// it since {{outputs.<parallelStepId>.*}} references from later steps
	// depend on this shape being stable.
	return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: aggregateOutputs(results)}, nil
}
```

Add `ErrParallelStepFailed` as a new sentinel in `internal/domain` (or
`internal/usecase`, following whatever this codebase's existing
per-package error-sentinel convention is — see `domain.ErrTemplateNotFound`
et al. in `template.go:28-56` for the pattern).

Confirm `domain.StepExecution`'s real constructor/field shape (this
sketch's `domain.NewStepExecution(...)` is a placeholder — check the real
type, likely alongside `domain.StepExecution.MarkRunning`/`Fail`/
`FromResult` used elsewhere in `wave_dispatcher.go`) before implementing
persistence for sub-steps; whether each `parallel` sub-step gets its own
persisted `step_executions` row (recommended, for observability parity
with top-level steps) or is aggregated into the parent's single row is a
decision this task must make explicit in the PR description.

## Not in scope

- Nested `parallel`-within-`parallel` depth limits — not enforced by this
  task; flag in review if the team wants a hard cap (per BE-SOL-003's own
  "Not in scope" note).
- UI for `parallel` steps.

## Test plan

- `parallel` step: one sub-step fails, `allowPartialFailure=true` → step
  succeeds with partial results recorded.
- `allowPartialFailure=false` → step fails, and the overall execution
  fails per `waveDispatcher`'s existing "any step failing fails the whole
  execution" semantics (`wave_dispatcher.go`'s doc comment, `:28-35`).
- All sub-steps succeed → step succeeds, aggregated output shape is
  stable/documented.
- A `{{outputs.<parallelStepId>.*}}` reference from a later step resolves
  correctly against the aggregated output.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/usecase/... -run "TestWaveDispatcher_Parallel|TestRunParallelStep" -v
```

Expected: clean build; partial-failure and full-failure/full-success
cases pass; concurrent sub-step dispatch doesn't race on `results`/`errs`
(run with `-race`).
