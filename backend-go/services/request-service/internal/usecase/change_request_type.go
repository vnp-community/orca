package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ChangeInput struct {
	RequestID       string
	NewType         string
	Size            string
	Urgency         string
	Reason          string
	ExpectedVersion int64
	ActorID         string
	ActorKind       domain.ActorKind
}

type ChangeRequestType struct {
	repo       RequestRepository
	history    RequestTypeHistoryRepository
	transition RequestTransitioner
	canceller  ApprovalCanceller
	guard      ExecutionGuard
	tx         TxRunner
	outbox     OutboxWriter
	clarifs    ClarificationCanceller
}

// WithClarifications closes the open Clarification when the type of a request in awaiting_information changes.
func (uc *ChangeRequestType) WithClarifications(c ClarificationCanceller) *ChangeRequestType {
	uc.clarifs = c
	return uc
}

func NewChangeRequestType(repo RequestRepository, history RequestTypeHistoryRepository, transition RequestTransitioner,
	canceller ApprovalCanceller, guard ExecutionGuard, tx TxRunner, outbox OutboxWriter) *ChangeRequestType {
	return &ChangeRequestType{repo: repo, history: history, transition: transition, canceller: canceller, guard: guard, tx: tx, outbox: outbox}
}

func typeChangeStatus(s domain.RequestStatus) bool {
	switch s {
	case domain.RequestStatusAnalyzing, domain.RequestStatusAwaitingAnalysisApproval, domain.RequestStatusPlanning,
		domain.RequestStatusAwaitingPlanApproval, domain.RequestStatusExecuting, domain.RequestStatusAwaitingInformation:
		return true
	}
	return false
}

// Execute changes the type of a Request already past confirmation. It never touches solutions,
// plans or tasks: work done so far stays as reference for the re-confirmed type.
func (uc *ChangeRequestType) Execute(ctx context.Context, in ChangeInput) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	if in.ActorKind != domain.ActorKindUser {
		return domain.Request{}, domain.ErrRequestActorNotAllowed(in.ActorKind)
	}
	newType, err := domain.ParseRequestType(in.NewType)
	if err != nil {
		return domain.Request{}, err
	}
	if strings.TrimSpace(in.Reason) == "" {
		return domain.Request{}, domain.ErrTypeReasonRequired(newType)
	}
	var size domain.RequestSize
	if in.Size != "" {
		if size, err = domain.ParseSize(in.Size); err != nil {
			return domain.Request{}, err
		}
	}
	var urgency domain.Urgency
	if in.Urgency != "" {
		if urgency, err = domain.ParseUrgency(in.Urgency); err != nil {
			return domain.Request{}, err
		}
	}

	var out domain.Request
	err = uc.tx.InTx(ctx, func(txCtx context.Context) error {
		r, err := uc.repo.Get(txCtx, in.RequestID)
		if err != nil {
			return err
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != r.Version {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		if !typeChangeStatus(r.Status) {
			return domain.ErrRequestTypeActionWrongStatus(r.Status, "changing the type")
		}
		if err := domain.ChangeTypeAllowed(r.Type, newType); err != nil {
			return err
		}
		if r.Status == domain.RequestStatusExecuting {
			active, err := uc.guard.HasActiveExecution(txCtx, r.ID)
			if err != nil {
				return err
			}
			if active {
				return domain.ErrRequestTypeChangeBlockedActiveExecution()
			}
		}
		status := r.Status
		previous := r.Type
		r.Type, r.TypeSource = newType, domain.TypeSourceHuman
		if size != "" {
			r.Size = size
		}
		if urgency != "" {
			r.Urgency = urgency
		}
		if _, err := uc.repo.Update(txCtx, r, r.Version); err != nil {
			return err
		}
		if err := uc.history.Append(txCtx, domain.RequestTypeChange{
			RequestID: r.ID, FromType: previous, ToType: newType, ActorID: in.ActorID, ActorKind: domain.ActorKindUser, Reason: in.Reason,
		}); err != nil {
			return err
		}
		if err := uc.canceller.CancelPending(txCtx, r.ID, "type_changed"); err != nil {
			return err
		}
		if status == domain.RequestStatusAwaitingInformation && uc.clarifs != nil {
			if _, err := uc.clarifs.CancelOpenForRequest(txCtx, r.ID, "type_changed"); err != nil {
				return err
			}
		}
		res, err := uc.transition.Execute(txCtx, TransitionInput{
			RequestID: r.ID, Trigger: domain.TriggerTypeChange, ExpectedFrom: &status, ActorID: in.ActorID, ActorKind: domain.ActorKindUser, Reason: in.Reason,
		})
		if err != nil {
			return err
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectRequestTypeChanged, map[string]any{
			"request_id": r.ID, "from": previous, "to": newType, "actor_id": in.ActorID, "reason": in.Reason,
		})
		if err != nil {
			return err
		}
		if err := uc.outbox.InsertOutboxEvent(txCtx, ev); err != nil {
			return err
		}
		out = res.Request
		return nil
	})
	return out, err
}
