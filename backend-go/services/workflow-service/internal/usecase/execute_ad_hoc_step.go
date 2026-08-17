package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

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
// Known gap (see README): unlike a real Execute, this does not persist a
// synthetic one-step execution/step_executions row (§3.1's "creates a
// synthetic one-step execution... so ad hoc runs get the same
// observability/resumability/history as template-driven ones"). This
// scaffold's ExecutionRepository schema doesn't carry step-level rows at
// all (see workflow-service.md §5's fuller step_executions table, narrowed
// out of this build's data model) — extend before automation-service takes
// a real dependency on ad hoc run history existing.
type ExecuteAdHocStep struct {
	registry StepExecutorRegistry
}

func NewExecuteAdHocStep(registry StepExecutorRegistry) *ExecuteAdHocStep {
	return &ExecuteAdHocStep{registry: registry}
}

func (uc *ExecuteAdHocStep) Execute(ctx context.Context, in ExecuteAdHocStepInput) (domain.StepResult, error) {
	// Tenant/user context still flows through here even though this
	// scaffold doesn't persist ad hoc runs: §9 requires step execution to
	// inherit the acting caller's identity all the way to the executor
	// (never workflow-service's own identity), and requiring it here keeps
	// that true even before persistence is added.
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.StepResult{}, apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}

	if !in.StepType.Valid() {
		return domain.StepResult{}, apperrors.New(apperrors.KindInvalidArgument, "WORKFLOW_INVALID_STEP_TYPE", "unknown step type", nil)
	}

	executor, err := uc.registry.Resolve(in.StepType)
	if err != nil {
		if errors.Is(err, ErrStepExecutorNotRegistered) {
			return domain.StepResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKFLOW_NO_STEP_EXECUTOR", "no executor registered for this step type", err)
		}
		return domain.StepResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_STEP_EXECUTOR_RESOLVE_FAILED", "failed to resolve step executor", err)
	}

	result, err := executor.Execute(ctx, in.StepConfigJSON)
	if err != nil {
		// A hard executor error (as opposed to a business-level "failed"
		// StepResult, which executors return with err == nil) means the
		// executor itself couldn't run the step at all — propagate as an
		// RPC failure so the caller (automation-service's RunNow) doesn't
		// mistake it for a completed-but-unsuccessful step.
		return domain.StepResult{}, apperrors.New(apperrors.KindInternal, "WORKFLOW_STEP_EXECUTION_FAILED", err.Error(), err)
	}

	return result, nil
}
