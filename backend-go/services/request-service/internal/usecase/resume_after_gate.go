package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ResumeAfterGate re-evaluates a Request whose execution-time gate (an ops_request pre_deploy) was approved,
// so the task that waited for it is dispatched. A phase approval does not resume anything: a person starts the Phase.
type ResumeAfterGate struct {
	Requests RequestReader
	Evaluate *EvaluateExecution
}

func (uc *ResumeAfterGate) Execute(ctx context.Context, requestID string, subject domain.SubjectType) error {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ErrRequestTenantRequired()
	}
	if subject != domain.SubjectPreDeploy {
		return nil
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	if req.Status != domain.RequestStatusExecuting {
		return nil
	}
	return uc.Evaluate.Run(ctx, req)
}
