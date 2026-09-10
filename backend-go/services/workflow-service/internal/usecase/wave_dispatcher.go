package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// defaultMaxConcurrentSteps bounds in-flight step dispatch per execution —
// workflow-service.md §8: a bounded worker pool, not one unbounded
// goroutine per step, which would let a pathological fan-out wave exhaust
// the outbound connection budget to infra-fleet-service.
const defaultMaxConcurrentSteps = 10

// waveDispatcher runs a DAG's waves against a StepExecutorRegistry,
// persisting one step_executions row per step and gating wave N+1 on every
// step in wave N reaching a terminal status (domain.StepExecution.Terminal)
// — the mechanism behind both usecase.Execute's real template-driven runs
// and usecase.ExecuteAdHocStep's single synthetic wave-0 run, so both paths
// get identical persistence/observability semantics instead of a
// bespoke one-off for ad hoc steps. See execute.go's doc comment for why
// Execute dispatches this off the RPC path.
//
// Failure semantics (a deliberate choice workflow-service.md §7's diagram
// doesn't spell out): ANY step failing — a business-level "failed"
// StepResult or a hard StepExecutor error — fails the whole execution.
// Waves already in flight when a failure is observed are not cancelled
// (their steps still run to completion and get their own terminal status
// persisted), but no further wave is dispatched once the current wave
// finishes. This is the simplest correct behavior given
// domain.Status has no partial-success state to report — see README.
type waveDispatcher struct {
	stepExecutions StepExecutionRepository
	registry       StepExecutorRegistry
	concurrency    int
}

func newWaveDispatcher(stepExecutions StepExecutionRepository, registry StepExecutorRegistry, concurrency int) *waveDispatcher {
	if concurrency <= 0 {
		concurrency = defaultMaxConcurrentSteps
	}
	return &waveDispatcher{stepExecutions: stepExecutions, registry: registry, concurrency: concurrency}
}

// dispatchWaves runs every wave of tenantID/executionID's DAG in order and
// reports whether the whole run succeeded (every step in every wave
// completed) — callers persist the owning execution's final status
// themselves, since ExecuteAdHocStep (one synthetic wave) and Execute (a
// real multi-wave run) manage that execution row slightly differently.
//
// inputs feeds usecase.Interpolate's {{path}} substitution pass
// (TASK-WF-003-01) — deliberately a call parameter, NOT a waveDispatcher
// field: this type is constructed once per usecase (Execute.dispatcher,
// ExecuteAdHocStep.dispatcher) and shared across every call that usecase
// ever makes, so per-execution data like inputs must never be stored on it
// (concurrent executions dispatched through the same shared *waveDispatcher
// would otherwise clobber each other's inputs — a real concurrency bug
// BE-SOL-003's own sketch, which put inputs on waveDispatcher itself,
// would have introduced).
//
// ctx's lifetime must outlive the RPC that triggered dispatch when called
// from Execute's background goroutine — see that type's doc comment; the
// context passed here is NOT the inbound RPC's context in that path.
func (d *waveDispatcher) dispatchWaves(ctx context.Context, executionID string, waves [][]domain.Step, inputs map[string]any) bool {
	return d.dispatchWavesFrom(ctx, executionID, waves, 0, nil, inputs)
}

// dispatchWavesFrom is dispatchWaves' resume variant, used by
// RecoverExecutions' boot-time scan (see that usecase's doc comment for
// the "first non-terminal-success wave" algorithm that computes startWave
// and existingRows). It starts dispatch at startWave instead of wave 0 —
// every earlier wave is assumed already fully, successfully terminal and
// is not touched again. For startWave only, existingRows lets a step that
// already has a persisted step_executions row (dispatched before a crash,
// now in an unknown pending/running/failed state) be re-dispatched onto
// that SAME row via UpdateStepExecution, instead of calling
// CreateStepExecution again — which would otherwise violate
// step_executions' (execution_id, step_id) UNIQUE constraint. Waves after
// startWave have no pre-existing rows and dispatch fresh, identical to
// dispatchWaves.
func (d *waveDispatcher) dispatchWavesFrom(ctx context.Context, executionID string, waves [][]domain.Step, startWave int, existingRows map[string]domain.StepExecution, inputs map[string]any) bool {
	// completedOutputs accumulates across every wave (not reset per wave)
	// since a later wave's {{outputs.<stepId>.*}} reference may name a
	// step from an earlier wave — see interpolateStepConfig. Seeded from
	// existingRows for the RESUME case: a step already recorded as
	// completed before a crash still needs its output available to a
	// step dispatched after recovery.
	completedOutputs := make(map[string]domain.StepResult)
	for stepID, row := range existingRows {
		if row.Status == domain.StepExecutionStatusCompleted {
			completedOutputs[stepID] = domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: row.OutputJSON}
		}
	}

	succeeded := true
	for waveIdx := startWave; waveIdx < len(waves); waveIdx++ {
		if !succeeded {
			break
		}
		var existing map[string]domain.StepExecution
		if waveIdx == startWave {
			existing = existingRows
		}
		if !d.dispatchWave(ctx, executionID, waveIdx, waves[waveIdx], existing, inputs, completedOutputs) {
			succeeded = false
		}
	}
	return succeeded
}

// dispatchWave persists a pending step_executions row for every step in
// the wave that doesn't already have one in existing, then runs every step
// in the wave concurrently through a bounded worker pool, returning once
// every step has reached a terminal status — this is the wave gate:
// dispatchWaves/dispatchWavesFrom do not start the next wave until this
// call returns, so a step in wave N+1 can never be dispatched before every
// step in wave N is terminal.
//
// existing (nil in the normal, non-recovery path — see dispatchWaves) maps
// step id to a step_executions row already persisted for this wave from
// before a crash. A step whose existing row is already
// StepExecutionStatusCompleted is treated as a known-good terminal outcome
// and is not re-run (re-dispatching a step already recorded as succeeded
// would duplicate its side effects for no benefit); every other step —
// missing, pending, running, or failed — is (re)dispatched, reusing the
// existing row via UpdateStepExecution when present instead of
// CreateStepExecution. See RecoverExecutions' doc comment for why
// "anything short of a recorded success" is re-dispatched rather than
// left alone: a running row's real-world outcome is unknown after a
// crash, and treating a failed row as equally uncertain keeps this rule
// uniform rather than adding a second special case.
// completedOutputs is mutated in place with every step in this wave that
// completes successfully (existing-and-already-completed steps included) —
// callers (dispatchWavesFrom) rely on this to make an earlier wave's output
// available to a later wave's {{outputs.<stepId>.*}} reference.
func (d *waveDispatcher) dispatchWave(ctx context.Context, executionID string, waveIdx int, wave []domain.Step, existing map[string]domain.StepExecution, inputs map[string]any, completedOutputs map[string]domain.StepResult) bool {
	type dispatchable struct {
		resultIdx int
		step      domain.Step
		row       domain.StepExecution
	}

	results := make([]bool, len(wave))
	// stepResults holds each dispatched step's StepResult at the same
	// index as results — written only by that step's own goroutine (one
	// writer per index, matching results' existing concurrency-safety
	// pattern), merged into completedOutputs single-threaded after
	// wg.Wait() below.
	stepResults := make([]domain.StepResult, len(wave))
	toDispatch := make([]dispatchable, 0, len(wave))
	for i, step := range wave {
		row, hasExisting := existing[step.ID]
		if hasExisting && row.Status == domain.StepExecutionStatusCompleted {
			// Already recorded as succeeded before the crash — re-running
			// it would duplicate its side effects for no benefit.
			results[i] = true
			stepResults[i] = domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: row.OutputJSON}
			continue
		}
		if !hasExisting {
			se, err := domain.NewStepExecution(uuid.NewString(), executionID, step.ID, uuid.NewString(), waveIdx)
			if err != nil {
				// Can't happen in practice: executionID is always non-empty
				// by the time dispatch starts, and step.ID was already
				// validated by DAGDefinition.Validate — fail closed rather
				// than panic.
				slog.ErrorContext(ctx, "workflow: building step execution failed", slog.String("execution_id", executionID), slog.String("step_id", step.ID), slog.Any("error", err))
				return false
			}
			if err := d.stepExecutions.CreateStepExecution(ctx, se); err != nil {
				slog.ErrorContext(ctx, "workflow: persisting pending step execution failed", slog.String("execution_id", executionID), slog.String("step_id", step.ID), slog.Any("error", err))
				return false
			}
			row = se
		}
		toDispatch = append(toDispatch, dispatchable{resultIdx: i, step: step, row: row})
	}

	// completedOutputs as dispatchStep/runStep see it must reflect every
	// step already known-good above (the existing-and-completed branch)
	// PLUS every other step already completed in an earlier wave — take a
	// read-only snapshot before dispatch starts so concurrent steps within
	// THIS wave don't race on the shared map (they never depend on a
	// same-wave sibling's output — BuildWaves' own wave-gate invariant —
	// so a stale-for-this-wave snapshot is correct, not just safe).
	outputsSnapshot := make(map[string]domain.StepResult, len(completedOutputs)+len(wave))
	for k, v := range completedOutputs {
		outputsSnapshot[k] = v
	}
	for i, step := range wave {
		if results[i] {
			outputsSnapshot[step.ID] = stepResults[i]
		}
	}

	sem := make(chan struct{}, d.concurrency)
	var wg sync.WaitGroup
	for _, dsp := range toDispatch {
		wg.Add(1)
		sem <- struct{}{}
		go func(dsp dispatchable) {
			defer wg.Done()
			defer func() { <-sem }()
			ok, result := d.dispatchStep(ctx, dsp.step, dsp.row, inputs, outputsSnapshot)
			results[dsp.resultIdx] = ok
			stepResults[dsp.resultIdx] = result
		}(dsp)
	}
	wg.Wait()

	allOK := true
	for i, ok := range results {
		if ok {
			completedOutputs[wave[i].ID] = stepResults[i]
		} else {
			allOK = false
		}
	}
	return allOK
}

// dispatchStep resolves step's StepExecutor and runs it, persisting se's
// running->terminal transitions, and reports whether the step succeeded
// plus its StepResult (for completedOutputs — see dispatchWave).
func (d *waveDispatcher) dispatchStep(ctx context.Context, step domain.Step, se domain.StepExecution, inputs map[string]any, outputs map[string]domain.StepResult) (bool, domain.StepResult) {
	se.MarkRunning()
	if err := d.stepExecutions.UpdateStepExecution(ctx, se); err != nil {
		// A persistence hiccup on the pending->running transition doesn't
		// block dispatch — the terminal update below is what the wave gate
		// and final execution status actually depend on.
		slog.ErrorContext(ctx, "workflow: marking step execution running failed", slog.String("step_execution_id", se.ID), slog.Any("error", err))
	}

	result, err := d.runStep(ctx, step, &se, inputs, outputs)

	if uerr := d.stepExecutions.UpdateStepExecution(ctx, se); uerr != nil {
		slog.ErrorContext(ctx, "workflow: persisting terminal step execution failed", slog.String("step_execution_id", se.ID), slog.Any("error", uerr))
	}

	if err != nil {
		return false, result
	}
	return result.Status == domain.ResultStatusCompleted, result
}

// runStep resolves step's StepExecutor from the registry and calls it,
// mutating se in place (MarkRunning is the caller's responsibility; this
// only sets the terminal state via se.Fail/se.FromResult) but does NOT
// persist se — callers own the CreateStepExecution/UpdateStepExecution
// calls, since the concurrent wave-dispatch path and ExecuteAdHocStep's
// single synchronous path persist at different points.
//
// inputs/outputs feed usecase.Interpolate's {{path}} substitution pass
// (TASK-WF-003-01), applied to step.Config immediately before the
// executor runs — see interpolateStepConfig. ExecuteAdHocStep's
// single-step call site passes empty maps (no ExecuteRequest.inputs and
// no prior steps in an ad hoc run — {{input.*}}/{{outputs.*}} references
// there always fail closed with a clear error, never silently no-op).
func (d *waveDispatcher) runStep(ctx context.Context, step domain.Step, se *domain.StepExecution, inputs map[string]any, outputs map[string]domain.StepResult) (domain.StepResult, error) {
	// parallel (TASK-WF-003-03) is different in kind from every other step
	// type: it doesn't relay to an external system, it fans out to other
	// steps already expressed in this same DAG's Step shape and recurses
	// back into runStep itself — routing it through the ordinary
	// StepExecutorRegistry.Resolve path doesn't fit, since a "sub-step
	// fan-out" mechanism needs to call runStep recursively, not just relay
	// JSON to a StepExecutor.Execute(ctx, string) contract.
	if step.Type == domain.StepTypeParallel {
		result, err := d.runParallelStep(ctx, step, se, inputs, outputs)
		// Always record whatever aggregated sub-step output was produced,
		// even on an overall failure — unlike a hard executor error (no
		// StepResult at all, se.Fail's case), a failed parallel step still
		// ran and produced real, inspectable per-sub-step output.
		se.FromResult(result)
		if err != nil {
			se.Error = err.Error()
			return result, err
		}
		return result, nil
	}

	executor, err := d.registry.Resolve(step.Type)
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	interpolatedConfig, err := interpolateStepConfig(string(step.Config), inputs, outputs)
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	result, err := executor.Execute(ctx, interpolatedConfig)
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	se.FromResult(result)
	return result, nil
}

// runParallelStep fans out cfg.Steps concurrently, each through the SAME
// runStep this method is itself called from — a parallel step's sub-steps
// may themselves reference {{outputs.*}} from steps outside the parallel
// block (the outputs snapshot this method was handed), but NOT from
// sibling sub-steps within the same parallel block — there is no defined
// execution order among them to interpolate against.
// AllowPartialFailure controls whether one sub-step's failure fails the
// whole parallel step.
//
// Each sub-step gets its own persisted step_executions row (same wave
// index as the parent parallel step, executionID read off parentSE) —
// chosen over aggregating sub-steps into the parent's single row for
// observability parity with top-level steps: a sub-step's own
// running->terminal transition, error message, and output are each
// independently visible via ListStepExecutions, matching what a top-level
// step of the same type would record.
func (d *waveDispatcher) runParallelStep(ctx context.Context, step domain.Step, parentSE *domain.StepExecution, inputs map[string]any, outputs map[string]domain.StepResult) (domain.StepResult, error) {
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
			results[i], errs[i] = d.runParallelSubStep(ctx, s, parentSE.ExecutionID, parentSE.Wave, inputs, outputs)
		}(i, sub)
	}
	wg.Wait()

	anyFailed := false
	for i, r := range results {
		if errs[i] != nil || r.Status == domain.ResultStatusFailed {
			anyFailed = true
		}
	}
	outputJSON := aggregateParallelOutputs(cfg.Steps, results)
	if anyFailed && !cfg.AllowPartialFailure {
		return domain.StepResult{Status: domain.ResultStatusFailed, OutputJSON: outputJSON}, domain.ErrParallelStepFailed
	}
	return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: outputJSON}, nil
}

// runParallelSubStep persists sub's own step_executions row (pending ->
// running -> terminal, same lifecycle dispatchStep gives a top-level step)
// and runs it through runStep. A row-persistence failure is reported as
// this sub-step's own error rather than aborting its siblings — matching
// dispatchWave's "a persistence hiccup on pending->running doesn't block
// dispatch" tolerance for the intermediate transition, while a failure to
// even CREATE the row is a hard error for this one sub-step (nothing to
// update afterwards).
func (d *waveDispatcher) runParallelSubStep(ctx context.Context, sub domain.Step, executionID string, wave int, inputs map[string]any, outputs map[string]domain.StepResult) (domain.StepResult, error) {
	se, err := domain.NewStepExecution(uuid.NewString(), executionID, sub.ID, uuid.NewString(), wave)
	if err != nil {
		return domain.StepResult{}, err
	}
	if err := d.stepExecutions.CreateStepExecution(ctx, se); err != nil {
		return domain.StepResult{}, fmt.Errorf("wave_dispatcher: parallel: persisting sub-step %q: %w", sub.ID, err)
	}

	se.MarkRunning()
	if err := d.stepExecutions.UpdateStepExecution(ctx, se); err != nil {
		slog.ErrorContext(ctx, "workflow: marking parallel sub-step running failed", slog.String("step_execution_id", se.ID), slog.Any("error", err))
	}

	result, runErr := d.runStep(ctx, sub, &se, inputs, outputs)

	if err := d.stepExecutions.UpdateStepExecution(ctx, se); err != nil {
		slog.ErrorContext(ctx, "workflow: persisting terminal parallel sub-step failed", slog.String("step_execution_id", se.ID), slog.Any("error", err))
	}
	return result, runErr
}

// aggregateParallelOutputs is the stable shape {{outputs.<parallelStepId>.*}}
// references from later steps depend on:
// {"subSteps": {"<stepId>": {"status": "completed"|"failed", "output": <parsed sub-step OutputJSON, or null>}}}.
// Each sub-step's OutputJSON is parsed back into a real JSON value (not
// left as an escaped string) so a reference like
// {{outputs.parallelStepId.subSteps.stepA.output.branch}} resolves through
// interpolate.go's ordinary nested-map walk. A sub-step whose OutputJSON
// is empty or fails to parse contributes a null output rather than
// aborting the whole aggregation over one sub-step's malformed JSON.
func aggregateParallelOutputs(steps []domain.Step, results []domain.StepResult) string {
	subSteps := make(map[string]map[string]any, len(steps))
	for i, s := range steps {
		var output any
		if i < len(results) && results[i].OutputJSON != "" {
			_ = json.Unmarshal([]byte(results[i].OutputJSON), &output)
		}
		status := ""
		if i < len(results) {
			status = string(results[i].Status)
		}
		subSteps[s.ID] = map[string]any{"status": status, "output": output}
	}
	out, err := json.Marshal(map[string]any{"subSteps": subSteps})
	if err != nil {
		// Can't happen in practice: every value here is a plain map/string/
		// json.Unmarshal-produced any — fail closed with an empty object
		// rather than propagating a marshal error from an aggregation step.
		return `{"subSteps":{}}`
	}
	return string(out)
}
