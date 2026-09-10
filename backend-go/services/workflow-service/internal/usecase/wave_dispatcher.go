package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// executionContext bundles a domain.ExecutionContext with the mutex
// guarding concurrent writes to its Outputs map — steps within one wave
// dispatch concurrently (see dispatchWave). Deliberately NOT a waveDispatcher
// field: waveDispatcher is a single instance shared across every execution
// this process runs (constructed once in NewExecute/NewExecuteAdHocStep/
// NewRecoverExecutions), so per-execution state must be threaded explicitly
// through the dispatch call chain instead, or concurrent executions would
// corrupt each other's Inputs/Outputs.
type executionContext struct {
	mu  sync.Mutex
	ctx domain.ExecutionContext
}

// newExecutionContext builds a fresh executionContext for one Execute/
// RecoverExecutions run.
func newExecutionContext(execCtx domain.ExecutionContext) *executionContext {
	if execCtx.Outputs == nil {
		execCtx.Outputs = make(map[string]map[string]any)
	}
	return &executionContext{ctx: execCtx}
}

// snapshot returns a lock-protected copy of the current ExecutionContext —
// safe to pass to domain.Interpolate (which takes it by value) without
// holding execCtx's mutex for the duration of interpolation.
func (e *executionContext) snapshot() domain.ExecutionContext {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ctx
}

// recordOutput stores stepID's parsed output — called after a step
// completes, guarded since sibling steps in the same wave finish and write
// concurrently.
func (e *executionContext) recordOutput(stepID string, output map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx.Outputs[stepID] = output
}

// stepEventPayload is orca.workflow.step.completed/.failed's JSON payload
// shape (BE-SOL-003/TASK-FT-003-03). OriginTaskID is carried directly
// (empty, not absent, for a standalone workflow run — see dispatchStep) so
// api-gateway's task.activity channel (TASK-FT-003-04) can filter by it
// without a cross-service lookup.
type stepEventPayload struct {
	ExecutionID  string `json:"execution_id"`
	StepID       string `json:"step_id"`
	StepType     string `json:"step_type"`
	Status       string `json:"status"`
	OriginTaskID string `json:"origin_task_id"`
}

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
//
// originTaskID (BE-SOL-003/TASK-FT-003-03) is the owning execution's
// domain.WorkflowExecution.OriginTaskID — threaded down as a plain
// parameter (both callers already have the full exec object, so no extra
// DB round trip is needed) so dispatchStep's terminal outbox event can
// carry it without waveDispatcher needing its own ExecutionRepository
// dependency. Empty for a standalone workflow run.
func (d *waveDispatcher) dispatchWaves(ctx context.Context, executionID string, waves [][]domain.Step, execCtx *executionContext, originTaskID string) bool {
	return d.dispatchWavesFrom(ctx, executionID, waves, 0, nil, execCtx, originTaskID)
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
func (d *waveDispatcher) dispatchWavesFrom(ctx context.Context, executionID string, waves [][]domain.Step, startWave int, existingRows map[string]domain.StepExecution, execCtx *executionContext, originTaskID string) bool {
	succeeded := true
	for waveIdx := startWave; waveIdx < len(waves); waveIdx++ {
		if !succeeded {
			break
		}
		var existing map[string]domain.StepExecution
		if waveIdx == startWave {
			existing = existingRows
		}
		if !d.dispatchWave(ctx, executionID, waveIdx, waves[waveIdx], existing, execCtx, originTaskID) {
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
func (d *waveDispatcher) dispatchWave(ctx context.Context, executionID string, waveIdx int, wave []domain.Step, existing map[string]domain.StepExecution, execCtx *executionContext, originTaskID string) bool {
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
			results[dsp.resultIdx] = d.dispatchStep(ctx, dsp.step, dsp.row, execCtx, originTaskID)
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
//
// Before dispatch, step.Config is interpolated (domain.Interpolate)
// against execCtx's current snapshot — resolving {{...}} tokens referencing
// ExecuteRequest.inputs_json values or an earlier wave's step outputs
// (TASK-WF-02-06). ctx is also enriched with the triggering user/project
// (tenant.WithUserID/WithProjectID) so a StepExecutor needing that scope
// (e.g. AgentExecutor's ProviderResolver — see TASK-WF-02-05) can read it
// without its own signature changing. On success, the step's parsed
// OutputJSON is recorded into execCtx.Outputs for later waves to reference.
//
// originTaskID (BE-SOL-003/TASK-FT-003-03) is carried into the terminal
// transition's outbox event payload — empty for a standalone workflow run,
// non-empty when task-service's Engine 3 WorkflowExecutor dispatched the
// owning execution.
func (d *waveDispatcher) dispatchStep(ctx context.Context, step domain.Step, se domain.StepExecution, execCtx *executionContext, originTaskID string) bool {
	se.MarkRunning()
	if err := d.stepExecutions.UpdateStepExecution(ctx, se, domain.OutboxEvent{}); err != nil {
		// A persistence hiccup on the pending->running transition doesn't
		// block dispatch — the terminal update below is what the wave gate
		// and final execution status actually depend on. Never enqueues an
		// outbox event — only the terminal transition below does.
		slog.ErrorContext(ctx, "workflow: marking step execution running failed", slog.String("step_execution_id", se.ID), slog.Any("error", err))
	}

	snapshot := execCtx.snapshot()
	ctx = tenant.WithUserID(tenant.WithProjectID(ctx, snapshot.ProjectID), snapshot.UserID)

	interpolated, ierr := domain.Interpolate(string(step.Config), snapshot)
	if ierr != nil {
		// domain.Interpolate's own contract never actually returns a
		// non-nil error today (unresolvable tokens are left as literal
		// text, not failed) — handled defensively in case that contract
		// changes, using the same fail-closed shape runStep's own errors
		// use below.
		se.Fail(ierr.Error())
		// The outbox-events param was added by BE-SOL-003/TASK-FT-003-03
		// after this interpolation-failure branch was written; a zero-value
		// event here just skips the enqueue, same as the pending->running
		// transition above.
		if uerr := d.stepExecutions.UpdateStepExecution(ctx, se, domain.OutboxEvent{}); uerr != nil {
			slog.ErrorContext(ctx, "workflow: persisting terminal step execution failed", slog.String("step_execution_id", se.ID), slog.Any("error", uerr))
		}
		return false
	}
	step.Config = json.RawMessage(interpolated)

	result, err := d.runStep(ctx, step, &se)

	// Outbox event (BE-SOL-003/TASK-FT-003-03) — a marshal failure degrades
	// to "persist the terminal step status, skip the event" rather than
	// failing the step, same best-effort posture
	// orchestration-service.UpdateTaskStatusAndPromote's own marshal
	// failure already uses (TASK-FT-003-01).
	subject := "orca.workflow.step.completed"
	if se.Status == domain.StepExecutionStatusFailed {
		subject = "orca.workflow.step.failed"
	}
	var event domain.OutboxEvent
	if payload, merr := json.Marshal(stepEventPayload{
		ExecutionID: se.ExecutionID, StepID: se.StepID, StepType: string(step.Type),
		Status: string(se.Status), OriginTaskID: originTaskID,
	}); merr == nil {
		event = domain.OutboxEvent{ID: uuid.NewString(), Subject: subject, OccurredAt: time.Now().UTC(), PayloadJSON: payload}
	}

	if uerr := d.stepExecutions.UpdateStepExecution(ctx, se, event); uerr != nil {
		slog.ErrorContext(ctx, "workflow: persisting terminal step execution failed", slog.String("step_execution_id", se.ID), slog.Any("error", uerr))
	}

	if err != nil {
		return false
	}
	if result.Status == domain.ResultStatusCompleted {
		var parsed map[string]any
		_ = json.Unmarshal([]byte(result.OutputJSON), &parsed) // best-effort — see ExecutionContext.Outputs' doc comment
		execCtx.recordOutput(step.ID, parsed)
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
