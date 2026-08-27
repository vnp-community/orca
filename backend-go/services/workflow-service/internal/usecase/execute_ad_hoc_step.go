package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// adHocStepID is the synthetic step_executions.step_id every
// ExecuteAdHocStep run persists — there's no real DAG step id since ad hoc
// runs have no template, just the one step the caller supplied directly.
const adHocStepID = "ad-hoc"

// ExecuteAdHocStepInput mirrors ExecuteAdHocStepRequest — one step
// definition, no template, no dependsOn. See workflow-service.md §3.1:
// this is automation-service's route to closing TS Gap 3 ("automation.runNow
// should not gain execution by create a throwaway template, then Execute
// it").
type ExecuteAdHocStepInput struct {
	StepType       domain.StepType
	StepConfigJSON string
	RequestID      string
}

// ExecuteAdHocStep looks up the StepExecutor registered for the requested
// StepType and runs it directly — the same StepExecutor.Execute contract a
// step inside a real template-driven Execute would go through (§3.1), just
// without the surrounding template/DAG/wave machinery.
//
// It persists a synthetic one-step execution (domain.NewAdHocWorkflowExecution
// — no backing template) plus one step_executions row (wave 0), reusing
// waveDispatcher.runStep — the same single-step run logic Execute's real
// wave dispatch uses — instead of a bespoke ad hoc path. This closes §3.1's
// persistence gap the previous scaffold left (see README): an ad hoc run
// now gets the same observability/resumability/history as a
// template-driven step.
//
// Unlike Execute, this runs SYNCHRONOUSLY: automation-service's RunNow
// needs the step's result before it can report the automation run's
// outcome (§7: "synchronous, since automation needs the result before
// reporting a run's outcome") — a single step, unlike a whole DAG, doesn't
// carry the unbounded-latency concern that makes Execute's dispatch
// asynchronous (see execute.go's doc comment).
type ExecuteAdHocStep struct {
	executions     ExecutionRepository
	stepExecutions StepExecutionRepository
	dispatcher     *waveDispatcher
}

func NewExecuteAdHocStep(executions ExecutionRepository, stepExecutions StepExecutionRepository, registry StepExecutorRegistry) *ExecuteAdHocStep {
	return &ExecuteAdHocStep{
		executions:     executions,
		stepExecutions: stepExecutions,
		// concurrency=1: a single step never fans out, but newWaveDispatcher
		// still needs a positive value.
		dispatcher: newWaveDispatcher(stepExecutions, registry, 1),
	}
}

func (uc *ExecuteAdHocStep) Execute(ctx context.Context, in ExecuteAdHocStepInput) (domain.StepResult, error) {
	// Tenant/user context flows through here both to scope persistence and
	// because §9 requires step execution to inherit the acting caller's
	// identity all the way to the executor (never workflow-service's own
	// identity).
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.StepResult{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	if !in.StepType.Valid() {
		return domain.StepResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_STEP_TYPE", "unknown step type", nil)
	}

	exec, err := domain.NewAdHocWorkflowExecution(uuid.NewString(), tenantID, uuid.NewString())
	if err != nil {
		return domain.StepResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_EXECUTION", err.Error(), err)
	}
	if err := uc.executions.CreateExecution(ctx, exec); err != nil {
		return domain.StepResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_EXECUTION_SAVE_FAILED", "failed to persist ad hoc execution", err)
	}

	se, err := domain.NewStepExecution(uuid.NewString(), exec.ID, adHocStepID, uuid.NewString(), 0)
	if err != nil {
		return domain.StepResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_INVALID_STEP_EXECUTION", err.Error(), err)
	}
	if err := uc.stepExecutions.CreateStepExecution(ctx, se); err != nil {
		return domain.StepResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_STEP_EXECUTION_SAVE_FAILED", "failed to persist ad hoc step execution", err)
	}

	se.MarkRunning()
	if uerr := uc.stepExecutions.UpdateStepExecution(ctx, se); uerr != nil {
		slog.ErrorContext(ctx, "workflow: marking ad hoc step execution running failed", slog.String("step_execution_id", se.ID), slog.Any("error", uerr))
	}

	step := domain.Step{ID: adHocStepID, Type: in.StepType, Config: json.RawMessage(in.StepConfigJSON)}
	result, runErr := uc.dispatcher.runStep(ctx, step, &se)

	if uerr := uc.stepExecutions.UpdateStepExecution(ctx, se); uerr != nil {
		slog.ErrorContext(ctx, "workflow: persisting terminal ad hoc step execution failed", slog.String("step_execution_id", se.ID), slog.Any("error", uerr))
	}

	exec.Status = domain.StatusCompleted
	if runErr != nil || result.Status == domain.ResultStatusFailed {
		exec.Status = domain.StatusFailed
	}
	if uerr := uc.executions.UpdateExecution(ctx, exec, nil); uerr != nil {
		slog.ErrorContext(ctx, "workflow: persisting final ad hoc execution status failed", slog.String("execution_id", exec.ID), slog.Any("error", uerr))
	}

	if runErr != nil {
		if errors.Is(runErr, ErrStepExecutorNotRegistered) {
			return domain.StepResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_NO_STEP_EXECUTOR", "no executor registered for this step type", runErr)
		}
		// A hard executor error (as opposed to a business-level "failed"
		// StepResult, which executors return with err == nil) means the
		// executor itself couldn't run the step at all — propagate as an
		// RPC failure so the caller (automation-service's RunNow) doesn't
		// mistake it for a completed-but-unsuccessful step.
		return domain.StepResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_STEP_EXECUTION_FAILED", runErr.Error(), runErr)
	}

	return result, nil
}
