package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// SubjectArtifacts is what a subject-owning feature (Solution, Plan, Phase, Findings, Answer, Task list,
// pre-deploy) provides. It is a port because those features live in other CRs; the handler below owns only
// the Request-side effect of the decision.
type SubjectArtifacts interface {
	// Describe validates the subject exists for the Request and returns its id and a digest of what the approver sees.
	// hintedID is the caller's subject id for multi-instance subjects (phases, pre-deploy gates), else empty.
	Describe(ctx context.Context, req domain.Request, st domain.SubjectType, hintedID string) (subjectID, digest string, err error)
	// Decided applies the artifact-side effect (chosen option, plan status) inside the approval transaction.
	Decided(ctx context.Context, a domain.Approval, approved bool) error
	// Closed runs when the approval ends without a decision (expired, cancelled).
	Closed(ctx context.Context, a domain.Approval, why string) error
}

// TransitionSubjectHandler is the SubjectHandler for every gate that moves the Request through TransitionRequest:
// analysis gates (solution, findings, answer), plan gates (plan, task_list, and pre_deploy as start gate) and
// execution gates (phase, pre_deploy inside executing) that only need the artifact effect.
// With no Artifacts bound it refuses to open approvals (fail closed) instead of approving nothing.
type TransitionSubjectHandler struct {
	Artifacts  SubjectArtifacts
	Transition RequestTransitioner
	Returner   RequestReturner
	// Requests supplies the Request (flow, size) that decides the return stage on rejection.
	Requests RequestLocker
}

var _ SubjectHandler = (*TransitionSubjectHandler)(nil)

func (h *TransitionSubjectHandler) ValidateForRequest(ctx context.Context, _ context.Context, req domain.Request, st domain.SubjectType) (string, string, error) {
	if h.Artifacts == nil {
		return "", "", domain.ErrApprovalSubjectUnavailable
	}
	if !domain.ApprovalAllowedInStatus(st, req.Status) {
		return "", "", domain.ErrApprovalStageMismatch
	}
	return h.Artifacts.Describe(ctx, req, st, RequestedSubjectID(ctx))
}

func (h *TransitionSubjectHandler) OnApproved(ctx context.Context, _ context.Context, a domain.Approval) error {
	if h.Artifacts == nil {
		return domain.ErrApprovalSubjectUnavailable
	}
	if err := h.Artifacts.Decided(ctx, a, true); err != nil {
		return err
	}
	var trigger domain.Trigger
	switch domain.RequestStatus(a.Stage) {
	case domain.RequestStatusAwaitingAnalysisApproval:
		trigger = domain.TriggerAnalysisApproved
	case domain.RequestStatusAwaitingPlanApproval:
		trigger = domain.TriggerPlanApproved
	default:
		return nil // execution gate: releasing it needs no Request status change
	}
	from := domain.RequestStatus(a.Stage)
	_, err := h.Transition.Execute(ctx, TransitionInput{
		RequestID: a.RequestID, Trigger: trigger, ExpectedFrom: &from, ActorID: decidedByOf(a), ActorKind: domain.ActorKindUser,
	})
	return err
}

func (h *TransitionSubjectHandler) OnRejected(ctx context.Context, _ context.Context, a domain.Approval) error {
	if h.Artifacts == nil || h.Returner == nil || h.Requests == nil {
		return domain.ErrApprovalSubjectUnavailable
	}
	if err := h.Artifacts.Decided(ctx, a, false); err != nil {
		return err
	}
	req, err := h.Requests.LockRequest(ctx, a.RequestID)
	if err != nil {
		return err
	}
	_, err = h.Returner.Execute(ctx, ReturnInput{
		RequestID: a.RequestID, Stage: domain.ApprovalReturnStage(a.SubjectType, domain.RequestStatus(a.Stage), req),
		Category: domain.ReturnCategoryRejected, Reason: a.Comment, ActorID: decidedByOf(a), ActorKind: domain.ActorKindUser,
	})
	return err
}

func (h *TransitionSubjectHandler) OnClosedWithoutDecision(ctx context.Context, _ context.Context, a domain.Approval, why string) error {
	if h.Artifacts == nil {
		return nil // nothing was ever opened without Artifacts, so there is nothing to undo
	}
	return h.Artifacts.Closed(ctx, a, why)
}
