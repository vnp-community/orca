package usecase

import (
	"context"
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
// ctx's lifetime must outlive the RPC that triggered dispatch when called
// from Execute's background goroutine — see that type's doc comment; the
// context passed here is NOT the inbound RPC's context in that path.
func (d *waveDispatcher) dispatchWaves(ctx context.Context, executionID string, waves [][]domain.Step) bool {
	return d.dispatchWavesFrom(ctx, executionID, waves, 0, nil)
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
func (d *waveDispatcher) dispatchWavesFrom(ctx context.Context, executionID string, waves [][]domain.Step, startWave int, existingRows map[string]domain.StepExecution) bool {
	succeeded := true
	for waveIdx := startWave; waveIdx < len(waves); waveIdx++ {
		if !succeeded {
			break
		}
		var existing map[string]domain.StepExecution
		if waveIdx == startWave {
			existing = existingRows
		}
		if !d.dispatchWave(ctx, executionID, waveIdx, waves[waveIdx], existing) {
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
func (d *waveDispatcher) dispatchWave(ctx context.Context, executionID string, waveIdx int, wave []domain.Step, existing map[string]domain.StepExecution) bool {
	type dispatchable struct {
		resultIdx int
		step      domain.Step
		row       domain.StepExecution
	}

	results := make([]bool, len(wave))
	toDispatch := make([]dispatchable, 0, len(wave))
	for i, step := range wave {
		row, hasExisting := existing[step.ID]
		if hasExisting && row.Status == domain.StepExecutionStatusCompleted {
			// Already recorded as succeeded before the crash — re-running
			// it would duplicate its side effects for no benefit.
			results[i] = true
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

	sem := make(chan struct{}, d.concurrency)
	var wg sync.WaitGroup
	for _, dsp := range toDispatch {
		wg.Add(1)
		sem <- struct{}{}
		go func(dsp dispatchable) {
			defer wg.Done()
			defer func() { <-sem }()
			results[dsp.resultIdx] = d.dispatchStep(ctx, dsp.step, dsp.row)
		}(dsp)
	}
	wg.Wait()

	for _, ok := range results {
		if !ok {
			return false
		}
	}
	return true
}

// dispatchStep resolves step's StepExecutor and runs it, persisting se's
// running->terminal transitions, and reports whether the step succeeded.
func (d *waveDispatcher) dispatchStep(ctx context.Context, step domain.Step, se domain.StepExecution) bool {
	se.MarkRunning()
	if err := d.stepExecutions.UpdateStepExecution(ctx, se); err != nil {
		// A persistence hiccup on the pending->running transition doesn't
		// block dispatch — the terminal update below is what the wave gate
		// and final execution status actually depend on.
		slog.ErrorContext(ctx, "workflow: marking step execution running failed", slog.String("step_execution_id", se.ID), slog.Any("error", err))
	}

	result, err := d.runStep(ctx, step, &se)

	if uerr := d.stepExecutions.UpdateStepExecution(ctx, se); uerr != nil {
		slog.ErrorContext(ctx, "workflow: persisting terminal step execution failed", slog.String("step_execution_id", se.ID), slog.Any("error", uerr))
	}

	if err != nil {
		return false
	}
	return result.Status == domain.ResultStatusCompleted
}

// runStep resolves step's StepExecutor from the registry and calls it,
// mutating se in place (MarkRunning is the caller's responsibility; this
// only sets the terminal state via se.Fail/se.FromResult) but does NOT
// persist se — callers own the CreateStepExecution/UpdateStepExecution
// calls, since the concurrent wave-dispatch path and ExecuteAdHocStep's
// single synchronous path persist at different points.
func (d *waveDispatcher) runStep(ctx context.Context, step domain.Step, se *domain.StepExecution) (domain.StepResult, error) {
	executor, err := d.registry.Resolve(step.Type)
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	result, err := executor.Execute(ctx, string(step.Config))
	if err != nil {
		se.Fail(err.Error())
		return domain.StepResult{}, err
	}

	se.FromResult(result)
	return result, nil
}
