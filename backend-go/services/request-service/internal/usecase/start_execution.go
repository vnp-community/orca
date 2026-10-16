package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// StartExecution starts a Request that needs no button: a Plan without Phases, a task list, a hotfix.
// Phased flows wait for StartPhase unless the Request runs continuously. It runs the same state-based loop as the reconcile job, so a
// duplicate or late delivery of the status event only repeats idempotent work.
type StartExecution struct {
	Requests RequestReader
	Evaluate *EvaluateExecution
	// Pinner and Modes fix the execution mode of the Request when it enters executing (CR-REQ-038). Both nil keeps the
	// Request on step_by_step.
	Pinner ExecutionModePinner
	Modes  *ExecutionModeResolver
}

// Execute reports started=false for a Request that is not executing or waits for StartPhase.
func (uc *StartExecution) Execute(ctx context.Context, requestID string) (started bool, err error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return false, domain.ErrRequestTenantRequired()
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return false, err
	}
	if req.Status != domain.RequestStatusExecuting {
		return false, nil
	}
	flow, err := domain.FlowFor(req.Type)
	if err != nil {
		return false, err
	}
	if req.ExecutionMode == "" && uc.Modes != nil {
		mode, err := PinExecutionMode(ctx, uc.Pinner, uc.Modes, req)
		if err != nil {
			return false, err
		}
		req.ExecutionMode = mode
	}
	if flow.PhasesFor(req.Size) && !flow.StartsPhasesItself(req.ExecutionMode, req.Size) {
		// A person starts each Phase, but a gated flow must have the first Phase's Approval open for them to decide.
		return false, uc.Evaluate.Run(ctx, req)
	}
	// Continuous: Evaluate starts the first Phase itself, as the person who approved the plan.
	return true, uc.Evaluate.Run(ctx, req)
}
