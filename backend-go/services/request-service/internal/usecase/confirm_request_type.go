package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ConfirmInput struct {
	RequestID       string
	Type            string
	Size            string
	Urgency         string
	Reason          string
	ExpectedVersion int64
	ActorID         string
	ActorKind       domain.ActorKind
}

type ConfirmRequestType struct {
	repo       RequestRepository
	history    RequestTypeHistoryRepository
	transition RequestTransitioner
	approvals  ApprovalRecorder
	tx         TxRunner
	outbox     OutboxWriter
	readiness  *ReadinessGate
}

// WithReadiness makes a confirmation that leaves the request short of the Definition of Ready open a
// Clarification instead of moving on (CR-REQ-028). Without it the request always moves on.
func (uc *ConfirmRequestType) WithReadiness(g *ReadinessGate) *ConfirmRequestType {
	uc.readiness = g
	return uc
}

func NewConfirmRequestType(repo RequestRepository, history RequestTypeHistoryRepository, transition RequestTransitioner,
	approvals ApprovalRecorder, tx TxRunner, outbox OutboxWriter) *ConfirmRequestType {
	return &ConfirmRequestType{repo: repo, history: history, transition: transition, approvals: approvals, tx: tx, outbox: outbox}
}

// pastTypeConfirmation lists statuses a Request reaches only after its type was confirmed.
func pastTypeConfirmation(s domain.RequestStatus) bool {
	switch s {
	case domain.RequestStatusAnalyzing, domain.RequestStatusAwaitingAnalysisApproval, domain.RequestStatusPlanning,
		domain.RequestStatusAwaitingPlanApproval, domain.RequestStatusExecuting, domain.RequestStatusCompleted, domain.RequestStatusAwaitingInformation:
		return true
	}
	return false
}

func (uc *ConfirmRequestType) Execute(ctx context.Context, in ConfirmInput) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	if in.ActorKind != domain.ActorKindUser {
		return domain.Request{}, domain.ErrRequestActorNotAllowed(in.ActorKind)
	}
	typ, err := domain.ParseRequestType(in.Type)
	if err != nil {
		return domain.Request{}, err
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
		if pastTypeConfirmation(r.Status) && r.Type == typ {
			out = r // repeated confirm: succeed without writing or emitting anything
			return nil
		}
		if r.Status != domain.RequestStatusAwaitingTypeConfirmation {
			return domain.ErrRequestTypeActionWrongStatus(r.Status, "confirming the type")
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != r.Version {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		if urgency == "" {
			urgency = r.Urgency // an omitted urgency must not erase what the AI proposed
		}
		if err := domain.ValidateConfirmation(domain.ConfirmationInput{Type: typ, Size: size, Urgency: urgency, Reason: in.Reason}); err != nil {
			return err
		}

		acceptedAI := r.TypeSource == domain.TypeSourceAI && r.Type == typ
		previous := r.Type
		r.Type, r.Size, r.Urgency = typ, size, urgency
		if acceptedAI {
			r.TypeSource = domain.TypeSourceAI
		} else {
			r.TypeSource = domain.TypeSourceHuman
		}
		updated, err := uc.repo.Update(txCtx, r, r.Version)
		if err != nil {
			return err
		}
		if !acceptedAI {
			if err := uc.history.Append(txCtx, domain.RequestTypeChange{
				RequestID: r.ID, FromType: previous, ToType: typ, ActorID: in.ActorID, ActorKind: domain.ActorKindUser, Reason: in.Reason,
			}); err != nil {
				return err
			}
		}
		if err := uc.approvals.Approve(txCtx, r.ID, in.ActorID); err != nil {
			return err
		}
		emitConfirmed := func() error {
			ev, err := NewOutboxEvent(txCtx, domain.SubjectRequestTypeConfirmed, map[string]any{
				"request_id": r.ID, "type": typ, "size": size, "urgency": urgency, "type_source": updated.TypeSource, "actor_id": in.ActorID,
			})
			if err != nil {
				return err
			}
			return uc.outbox.InsertOutboxEvent(txCtx, ev)
		}
		// The person did confirm the type, so type_confirmed and the request_type approval stand either way; a request
		// short of the Definition of Ready just waits for information instead of moving to analysis.
		if uc.readiness != nil {
			gate, err := uc.readiness.GateTypeConfirmed(txCtx, updated, in.ActorID)
			if err != nil {
				return err
			}
			if gate.Handled {
				out = gate.Request
				return emitConfirmed()
			}
		}
		from := domain.RequestStatusAwaitingTypeConfirmation
		res, err := uc.transition.Execute(txCtx, TransitionInput{
			RequestID: r.ID, Trigger: domain.TriggerTypeConfirmed, ExpectedFrom: &from, ActorID: in.ActorID, ActorKind: domain.ActorKindUser,
		})
		if err != nil {
			return err
		}
		if err := emitConfirmed(); err != nil {
			return err
		}
		out = res.Request
		return nil
	})
	return out, err
}
