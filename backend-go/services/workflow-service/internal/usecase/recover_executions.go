package usecase

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// RecoverExecutions is the boot-time recovery scan workflow-service.md §8
// requires ("resumability after restart is a hard requirement, not
// best-effort"): on startup, before this instance accepts new Execute
// calls, it re-attaches to every execution left in status=running by a
// previous process instance (a crash, deploy, or restart) and resumes wave
// dispatch where it left off. This closes the gap execute.go's doc comment
// names explicitly ("a boot-time recovery scan re-attaching to
// root_trace_id, not implemented in this pass").
//
// Deliberately out of scope, matching workflow-service.md §8 and
// domain.WorkflowExecution.Resume's doc comment: status=paused executions
// are never touched here — a paused execution was a deliberate user/system
// action, and a restart must not silently resume it. PauseExecution/
// ResumeExecution/CancelExecution remain the only paths that transition a
// paused execution.
type RecoverExecutions struct {
	templates      TemplateRepository
	executions     ExecutionRepository
	stepExecutions StepExecutionRepository
	dispatcher     *waveDispatcher
}

func NewRecoverExecutions(templates TemplateRepository, executions ExecutionRepository, stepExecutions StepExecutionRepository, registry StepExecutorRegistry) *RecoverExecutions {
	return &RecoverExecutions{
		templates:      templates,
		executions:     executions,
		stepExecutions: stepExecutions,
		dispatcher:     newWaveDispatcher(stepExecutions, registry, defaultMaxConcurrentSteps),
	}
}

// Execute runs the scan once. Unlike every other usecase in this codebase,
// it takes no tenant from ctx (there is no inbound request to take one
// from) and is not called from an RPC handler — see
// ExecutionRepository.ListRunning's doc comment for why this usecase alone
// operates process-wide, across every tenant this instance's database
// holds. Callers (cmd/server/main.go) should call this once at startup,
// before the gRPC server starts accepting Execute calls, per §8's ordering
// requirement.
//
// Execute itself only blocks long enough to list running executions and
// reconstruct each one's DAG/dispatch state (fast, bounded work) — each
// recovered execution's actual wave dispatch runs on its own detached
// background goroutine (the same pattern usecase.Execute's own RPC path
// establishes), so a slow or long-running recovered step never delays
// server startup.
func (uc *RecoverExecutions) Execute(ctx context.Context) error {
	running, err := uc.executions.ListRunning(ctx)
	if err != nil {
		return err
	}

	for _, exec := range running {
		uc.recoverOne(ctx, exec)
	}
	return nil
}

// recoverOne reconstructs exec's DAG and existing step_executions rows,
// determines where dispatch left off, and either resumes dispatch (on a
// detached background goroutine) or, if the run had actually already
// finished before the crash (every wave already terminal-success), simply
// persists the final status that runToCompletion never got to write.
func (uc *RecoverExecutions) recoverOne(ctx context.Context, exec domain.WorkflowExecution) {
	if exec.TemplateID == "" {
		// An ad hoc execution (usecase.ExecuteAdHocStep) has no backing
		// WorkflowTemplate to reconstruct a DAG from, and — unlike a
		// template-driven step — its single step's config JSON is never
		// persisted anywhere (step_executions has no config column, only
		// output/error): ExecuteAdHocStep runs synchronously and holds the
		// config only in memory for the duration of the call. If a crash
		// left an ad hoc execution stuck at status=running, this scan has
		// no way to know what to re-run. Documented as a known gap (see
		// README) rather than silently guessed at.
		slog.WarnContext(ctx, "workflow: recovery scan: leaving ad hoc execution stuck in running, no persisted DAG/step config to resume from", slog.String("execution_id", exec.ID), slog.String("tenant_id", exec.TenantID))
		return
	}

	tmpl, err := uc.templates.GetTemplate(ctx, exec.TenantID, exec.TemplateID)
	if err != nil {
		slog.ErrorContext(ctx, "workflow: recovery scan: fetching template failed, leaving execution as-is", slog.String("execution_id", exec.ID), slog.String("template_id", exec.TemplateID), slog.Any("error", err))
		return
	}

	dag, err := domain.ParseDAG(tmpl.DAGJSON)
	if err != nil {
		slog.ErrorContext(ctx, "workflow: recovery scan: parsing dag failed, leaving execution as-is", slog.String("execution_id", exec.ID), slog.Any("error", err))
		return
	}
	if err := dag.Validate(); err != nil {
		slog.ErrorContext(ctx, "workflow: recovery scan: dag validation failed, leaving execution as-is", slog.String("execution_id", exec.ID), slog.Any("error", err))
		return
	}
	waves, err := dag.BuildWaves()
	if err != nil {
		slog.ErrorContext(ctx, "workflow: recovery scan: building waves failed, leaving execution as-is", slog.String("execution_id", exec.ID), slog.Any("error", err))
		return
	}

	if len(waves) == 0 {
		// A zero-step DAG completes trivially (see Execute's own handling)
		// — the crash must have landed after CreateExecution but before
		// runToCompletion's UpdateExecution, since dispatchWaves(nil)
		// always succeeds immediately. Nothing to redispatch; just finish
		// it.
		uc.finish(ctx, exec, true)
		return
	}

	existingRows, err := uc.stepExecutions.ListStepExecutions(ctx, exec.TenantID, exec.ID)
	if err != nil {
		slog.ErrorContext(ctx, "workflow: recovery scan: listing step executions failed, leaving execution as-is", slog.String("execution_id", exec.ID), slog.Any("error", err))
		return
	}
	byStepID := make(map[string]domain.StepExecution, len(existingRows))
	for _, se := range existingRows {
		byStepID[se.StepID] = se
	}

	// The resume point is the first wave that is NOT fully terminal-
	// success: every step in an earlier wave must have a persisted row
	// with status=completed, or that earlier wave isn't actually done and
	// dispatch couldn't have moved past it (waveDispatcher's wave gate —
	// see dispatchWave — never starts wave N+1 before wave N is fully
	// terminal). A step with no row at all (never dispatched), or a row
	// that's pending/running (crashed mid-flight, real-world outcome
	// unknown) or failed, all count as "not a terminal success" and land
	// dispatch back on this wave.
	resumeWave := len(waves)
	for waveIdx, wave := range waves {
		waveFullySucceeded := true
		for _, step := range wave {
			se, ok := byStepID[step.ID]
			if !ok || se.Status != domain.StepExecutionStatusCompleted {
				waveFullySucceeded = false
				break
			}
		}
		if !waveFullySucceeded {
			resumeWave = waveIdx
			break
		}
	}

	if resumeWave >= len(waves) {
		// Every wave already fully succeeded before the crash — only the
		// final UpdateExecution(status=completed) never landed.
		uc.finish(ctx, exec, true)
		return
	}

	// Detach from ctx and carry the recovered execution's own tenant id
	// forward explicitly — same reasoning as Execute.Execute's background
	// goroutine (see execute.go's doc comment): this dispatch must outlive
	// Execute's own ctx (the boot-time scan's, not any RPC's), and every
	// recovered execution needs ITS OWN tenant id, not a shared one, since
	// this scan spans every tenant in one pass.
	dispatchCtx := tenant.WithTenantID(context.Background(), exec.TenantID)
	go uc.resumeToCompletion(dispatchCtx, exec, waves, resumeWave, byStepID)
}

// resumeToCompletion is recoverOne's async tail — the resumed-dispatch
// counterpart to Execute's runToCompletion, reusing the same waveDispatcher
// machinery (dispatchWavesFrom, see wave_dispatcher.go) instead of a
// bespoke recovery-only dispatch loop.
func (uc *RecoverExecutions) resumeToCompletion(ctx context.Context, exec domain.WorkflowExecution, waves [][]domain.Step, resumeWave int, existingRows map[string]domain.StepExecution) {
	// Inputs are NOT recoverable here — ExecuteRequest.inputs_json is
	// never persisted (see execute.go), so a resumed-after-crash execution
	// loses access to {{...}} input tokens for its remaining waves; earlier
	// waves' {{outputs.*}} ARE recoverable, reconstructed from each
	// completed step's persisted OutputJSON below. Documented as a known
	// gap alongside recoverOne's ad-hoc-execution one, not a silent loss.
	execCtx := newExecutionContext(domain.ExecutionContext{ProjectID: exec.ProjectID})
	for stepID, se := range existingRows {
		if se.Status != domain.StepExecutionStatusCompleted {
			continue
		}
		var parsed map[string]any
		_ = json.Unmarshal([]byte(se.OutputJSON), &parsed)
		execCtx.recordOutput(stepID, parsed)
	}

	succeeded := uc.dispatcher.dispatchWavesFrom(ctx, exec.ID, waves, resumeWave, existingRows, execCtx)
	uc.finish(ctx, exec, succeeded)
}

// finish persists exec's final status — completed or failed, mirroring
// runToCompletion's own terminal-status logic exactly (see execute.go).
func (uc *RecoverExecutions) finish(ctx context.Context, exec domain.WorkflowExecution, succeeded bool) {
	exec.Status = domain.StatusCompleted
	if !succeeded {
		exec.Status = domain.StatusFailed
	}
	if err := uc.executions.UpdateExecution(ctx, exec); err != nil {
		slog.ErrorContext(ctx, "workflow: recovery scan: persisting final execution status failed", slog.String("execution_id", exec.ID), slog.String("status", string(exec.Status)), slog.Any("error", err))
	}
}
