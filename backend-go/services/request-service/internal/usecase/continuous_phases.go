package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// startNextPhase is the continuous-mode replacement for opening a Phase approval: once the plan is approved, Orca releases
// the next ready Phase itself, one at a time, as the person who approved the plan (the execution actor).
//
// It only marks the Phase as started; the dispatch step of the same evaluation then runs its tasks. A Phase that is
// waiting for a human decision (a drift review, any other pending Phase approval) keeps the run paused.
func (e *EvaluateExecution) startNextPhase(ctx context.Context, req domain.Request, tree domain.ExecutionTree, started map[string]bool) error {
	for _, p := range tree.Phases {
		if started[p.ID] && !taskFinishedStatus(p.Status) {
			return nil // a Phase is still running: the next one waits
		}
	}
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	approvals, err := e.Approvals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil {
		return err
	}
	for _, a := range approvals {
		if a.SubjectType == domain.SubjectPhase && a.Status == domain.ApprovalStatusPending {
			return nil // a person has to look at a Phase first (a plan drift review)
		}
	}
	actor, err := e.Actors.ActorFor(ctx, req, "")
	if err != nil {
		return err
	}
	next, ok, err := e.nextReadyPhase(ctx, actor, tree, started)
	if err != nil || !ok {
		return err
	}
	inserted, err := claimPhase(ctx, e.Tx, e.PhaseStarts, e.Outbox, req, next.ID, actor, domain.PhaseStartAuto)
	if err != nil {
		return err
	}
	started[next.ID] = true
	if inserted {
		e.log().Info("phase started automatically", slog.String("request_id", req.ID), slog.String("phase_task_id", next.ID), slog.String("actor", actor))
	}
	return nil
}

// returnWhenActorForbidden sends a continuous Request back instead of letting it retry forever: the person who approved
// the plan may not run its tasks, which no amount of waiting fixes. A person can reopen it after fixing the access.
func (e *EvaluateExecution) returnWhenActorForbidden(ctx context.Context, req domain.Request, cause error) (bool, error) {
	if req.ExecutionMode != domain.ExecutionContinuous || !errorHasCode(cause, "REQUEST_EXECUTE_FORBIDDEN") {
		return false, nil
	}
	_, err := e.Returner.Execute(ctx, ReturnInput{
		RequestID: req.ID, Stage: domain.ReturnStageTask, Category: domain.ReturnCategoryBlockedDependency,
		Reason:    "The person who approved the plan may not run its tasks. Give them execute access, then reopen the request.",
		ActorKind: domain.ActorKindSystem,
	})
	if errorHasCode(err, "REQUEST_RETURN_BLOCKED_ACTIVE_EXECUTION") {
		return true, nil // a task is still running; the reconcile loop looks again once nothing runs
	}
	return err == nil, err
}
