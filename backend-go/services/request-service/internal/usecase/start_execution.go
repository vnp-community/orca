package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// StartExecution starts a Request that needs no button: a Plan without Phases, a task list, a hotfix.
// Phased flows wait for StartPhase. It runs the same state-based loop as the reconcile job, so a
// duplicate or late delivery of the status event only repeats idempotent work.
type StartExecution struct {
	Requests RequestReader
	Evaluate *EvaluateExecution
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
	if flow.PhasesFor(req.Size) {
		return false, nil // a person starts each Phase
	}
	return true, uc.Evaluate.Run(ctx, req)
}
