package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestTypeConfirmer is *ConfirmRequestType.
type RequestTypeConfirmer interface {
	Execute(ctx context.Context, in ConfirmInput) (domain.Request, error)
}

var _ RequestTypeConfirmer = (*ConfirmRequestType)(nil)

// RequestTypeApprovalHandler settles the request_type gate: approving confirms the AI-proposed type through
// ConfirmRequestType (which emits type_confirmed and moves the Request on), rejecting returns the Request to the backlog.
// Confirm is bound after construction because ConfirmRequestType itself depends on the approval recorder.
type RequestTypeApprovalHandler struct {
	Confirm  RequestTypeConfirmer
	Returner RequestReturner
	Requests RequestLocker
}

var _ SubjectHandler = (*RequestTypeApprovalHandler)(nil)

func (h *RequestTypeApprovalHandler) ValidateForRequest(_ context.Context, _ context.Context, req domain.Request, st domain.SubjectType) (string, string, error) {
	if st != domain.SubjectRequestType {
		return "", "", domain.ErrApprovalSubjectTypeNotAllowed
	}
	if req.Status != domain.RequestStatusAwaitingTypeConfirmation {
		return "", "", domain.ErrApprovalStageMismatch
	}
	if req.Type == "" {
		return "", "", domain.ErrApprovalSubjectNotFound // nothing proposed yet
	}
	return req.ID, domain.SubjectDigest(st, string(req.Type), string(req.Size), string(req.Urgency)), nil
}

func (h *RequestTypeApprovalHandler) OnApproved(ctx context.Context, _ context.Context, a domain.Approval) error {
	if h.Confirm == nil {
		return domain.ErrApprovalSubjectUnavailable
	}
	// Re-read the locked Request so the confirmed values are the ones the digest covered.
	req, err := h.Requests.LockRequest(ctx, a.RequestID)
	if err != nil {
		return err
	}
	_, err = h.Confirm.Execute(ctx, ConfirmInput{
		RequestID: a.RequestID, Type: string(req.Type), Size: string(req.Size), Urgency: string(req.Urgency),
		ActorID: decidedByOf(a), ActorKind: domain.ActorKindUser,
	})
	return err
}

func (h *RequestTypeApprovalHandler) OnRejected(ctx context.Context, _ context.Context, a domain.Approval) error {
	if h.Returner == nil {
		return domain.ErrApprovalSubjectUnavailable
	}
	_, err := h.Returner.Execute(ctx, ReturnInput{
		RequestID: a.RequestID, Stage: domain.ReturnStageClassification, Category: domain.ReturnCategoryRejected,
		Reason: a.Comment, ActorID: decidedByOf(a), ActorKind: domain.ActorKindUser,
	})
	return err
}

func (h *RequestTypeApprovalHandler) OnClosedWithoutDecision(context.Context, context.Context, domain.Approval, string) error {
	return nil // expiry/return is driven by the sweeper and the lifecycle use cases that already close the approval
}

func decidedByOf(a domain.Approval) string {
	if a.DecidedBy != nil {
		return *a.DecidedBy
	}
	return systemActor
}
