package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ReturnInput struct {
	RequestID       string
	Stage           domain.ReturnStage
	Category        domain.ReturnCategory
	Reason          string
	ExpectedVersion int64 // 0 skips the check
	ActorID         string
	ActorKind       domain.ActorKind
}

// ReturnRequestToBacklog is also the entry for Approval rejections (CR-REQ-007/009) and execution failures (CR-REQ-013, ActorKind system).
type ReturnRequestToBacklog struct {
	repo       RequestRepository
	transition RequestTransitioner
	history    ReturnHistoryRepository
	canceller  ApprovalCanceller
	guard      ExecutionGuard
	tx         TxRunner
	outbox     OutboxWriter
	clarifs    ClarificationCanceller
}

// WithClarifications closes the open Clarification when a request in awaiting_information is returned.
func (uc *ReturnRequestToBacklog) WithClarifications(c ClarificationCanceller) *ReturnRequestToBacklog {
	uc.clarifs = c
	return uc
}

func NewReturnRequestToBacklog(repo RequestRepository, transition RequestTransitioner, history ReturnHistoryRepository,
	canceller ApprovalCanceller, guard ExecutionGuard, tx TxRunner, outbox OutboxWriter) *ReturnRequestToBacklog {
	return &ReturnRequestToBacklog{repo: repo, transition: transition, history: history, canceller: canceller, guard: guard, tx: tx, outbox: outbox}
}

// ReturnedPayload is the wire shape of orca.request.request.returned.
type ReturnedPayload struct {
	RequestID string `json:"request_id"`
	Stage     string `json:"stage"`
	Category  string `json:"category"`
	Reason    string `json:"reason"`
	ActorID   string `json:"actor_id"`
}

// Execute does not retry a lost CAS: its caller is an RPC and the UI must reload.
func (uc *ReturnRequestToBacklog) Execute(ctx context.Context, in ReturnInput) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	if in.ActorKind == "" {
		in.ActorKind = domain.ActorKindUser
	}
	var out domain.Request
	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		r, err := uc.repo.Get(ctx, in.RequestID)
		if err != nil {
			return err
		}
		if r.Status == domain.RequestStatusRequestBacklog {
			if r.ReturnedFromStage == in.Stage {
				out = r // redelivery: already returned from this stage
				return nil
			}
			return domain.ErrTransitionNotAllowed(r.Status, domain.TriggerReturnToBacklog)
		}
		if in.ExpectedVersion != 0 && r.Version != in.ExpectedVersion {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		flow, _ := domain.FlowFor(r.Type) // untyped requests are returnable from classification
		valid, err := domain.StageForStatus(r.Status, flow, r.Size)
		if err != nil {
			return err
		}
		if !domain.ContainsStage(valid, in.Stage) {
			return domain.ErrReturnStageInvalid(string(in.Stage))
		}
		if strings.TrimSpace(in.Reason) == "" {
			return domain.ErrReasonRequired()
		}
		if _, err := domain.ParseReturnCategory(string(in.Category)); err != nil {
			return err
		}
		if r.Status == domain.RequestStatusExecuting {
			active, err := uc.guard.HasActiveExecution(ctx, r.ID)
			if err != nil {
				return err
			}
			if active {
				return domain.ErrReturnBlockedActiveExecution(r.ID)
			}
		}

		if r.Status == domain.RequestStatusAwaitingInformation && uc.clarifs != nil {
			if _, err := uc.clarifs.CancelOpenForRequest(ctx, r.ID, "returned_to_backlog"); err != nil {
				return err
			}
		}
		trigger := domain.TriggerReturnToBacklog
		if in.Category == domain.ReturnCategoryRejected {
			switch r.Status {
			case domain.RequestStatusAwaitingAnalysisApproval:
				trigger = domain.TriggerAnalysisRejected
			case domain.RequestStatusAwaitingPlanApproval:
				trigger = domain.TriggerPlanRejected
			}
		}
		from := r.Status
		res, err := uc.transition.Execute(ctx, TransitionInput{
			RequestID: r.ID, Trigger: trigger, ExpectedFrom: &from, ActorID: in.ActorID, ActorKind: in.ActorKind,
			Stage: in.Stage, Category: in.Category, Reason: in.Reason,
		})
		if err != nil {
			return err
		}
		if err := uc.history.Append(ctx, domain.ReturnHistoryEntry{
			RequestID: r.ID, Action: domain.ReturnActionReturned, Stage: in.Stage, Category: in.Category,
			Reason: in.Reason, ActorID: in.ActorID, ActorKind: in.ActorKind,
		}); err != nil {
			return err
		}
		if err := uc.canceller.CancelPending(ctx, r.ID, "returned"); err != nil {
			return err
		}
		ev, err := NewOutboxEvent(ctx, domain.SubjectRequestReturned, ReturnedPayload{
			RequestID: r.ID, Stage: string(in.Stage), Category: string(in.Category), Reason: in.Reason, ActorID: in.ActorID,
		})
		if err != nil {
			return err
		}
		if err := uc.outbox.InsertOutboxEvent(ctx, ev); err != nil {
			return err
		}
		out = res.Request
		return nil
	})
	if err != nil {
		return domain.Request{}, err
	}
	return out, nil
}
