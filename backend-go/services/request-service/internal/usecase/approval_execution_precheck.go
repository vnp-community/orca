package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ExecutionStartPrecheck stops a plan approval that could only leave the Request stuck. In a continuous run the approver is
// the execution actor for every Phase, and nobody is there to press Start when task-service refuses them; the Request would
// sit in executing until the reconcile loop gave up. Refusing before the decision is recorded lets the approver fix the access first.
type ExecutionStartPrecheck struct {
	Requests   RequestReader
	Modes      *ExecutionModeResolver
	Access     *ExecutionAccess
	Authorizer ApprovalAuthorizer
}

// Check runs for an approval that is about to be approved by approverID. It never writes.
func (p *ExecutionStartPrecheck) Check(ctx context.Context, a domain.Approval, approverID string) error {
	if p == nil || p.Access == nil || p.Requests == nil || a.Stage != string(domain.RequestStatusAwaitingPlanApproval) {
		return nil
	}
	switch a.SubjectType {
	case domain.SubjectPlan, domain.SubjectTaskList, domain.SubjectPreDeploy:
	default:
		return nil
	}
	req, err := p.Requests.Get(ctx, a.RequestID)
	if err != nil {
		return err
	}
	mode, err := p.Modes.Mode(ctx, req)
	if err != nil {
		return err
	}
	if mode != domain.ExecutionContinuous {
		return nil
	}
	// Someone who may not decide gets the normal refusal from the decision itself, not a hint about task access.
	if p.Authorizer != nil {
		if err := p.Authorizer.CanDecide(ctx, req, a); err != nil && !errors.Is(err, domain.ErrSelfApprovalForbidden) {
			return nil
		}
	}
	if err := p.Access.CheckReady(ctx, req, approverID); err != nil {
		if errorHasCode(err, "REQUEST_EXECUTE_FORBIDDEN") {
			return domain.ErrExecutionActorNotAllowed()
		}
		return err
	}
	return nil
}
