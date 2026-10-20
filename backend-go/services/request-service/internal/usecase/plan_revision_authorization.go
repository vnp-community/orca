package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AuthorizePlanRevise lets through the people who may generate a Plan, the approvers of the pending Plan gate, and the
// person whose own revision just failed (their Approval is already cancelled, so they no longer count as approver).
// It must run before the revision cancels the pending Approval.
func (a planActorFromSolution) AuthorizePlanRevise(ctx context.Context, req domain.Request) error {
	baseErr := a.inner.AuthorizeGenerate(ctx, req)
	if baseErr == nil {
		return nil
	}
	if !errorHasCode(baseErr, "REQUEST_SOLUTION_FORBIDDEN") {
		return baseErr
	}
	if !a.approversMayRevise {
		return domain.ErrPlanForbidden()
	}
	err := a.inner.AuthorizeRevisePlan(ctx, req)
	if err == nil {
		return nil
	}
	if !errorHasCode(err, "REQUEST_SOLUTION_FORBIDDEN") {
		return err
	}
	retry, err := a.isRetryOfOwnFailedRevision(ctx, req)
	if err != nil {
		return err
	}
	if retry {
		return nil
	}
	return domain.ErrPlanForbidden()
}

// isRetryOfOwnFailedRevision is true when the Request is back in planning after a revision the caller asked for failed,
// or while it still runs (a repeated click gets that run back instead of a refusal). A run without feedback is not a
// revision, and another person's run gives the caller nothing.
func (a planActorFromSolution) isRetryOfOwnFailedRevision(ctx context.Context, req domain.Request) (bool, error) {
	uid, _ := tenant.UserID(ctx)
	if uid == "" || a.runs == nil || req.Status != domain.RequestStatusPlanning || req.PlanTaskID == "" {
		return false, nil
	}
	recent, err := a.runs.ListRecent(ctx, req.ID, domain.RecentRunsLimit)
	if err != nil {
		return false, err
	}
	for _, run := range recent {
		if run.Kind != domain.RunKindPlan {
			continue
		}
		return (run.Status == domain.RunStatusFailed || run.Status == domain.RunStatusRunning) && run.ActorID == uid && run.Feedback != "", nil
	}
	return false, nil
}

// authorize picks the rule by what the call does: a first Plan stays with the reporter and admins, while changing a
// Plan that exists (feedback at the gate, or a retry in planning) also admits the gate's approvers.
func (uc *GeneratePlan) authorize(ctx context.Context, req domain.Request, feedback string) error {
	if isPlanRevision(req, feedback) {
		return uc.auth.AuthorizePlanRevise(ctx, req)
	}
	return uc.auth.AuthorizePlan(ctx, req)
}
