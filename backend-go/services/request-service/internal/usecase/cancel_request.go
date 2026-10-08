package usecase

import (
	"context"
	"strings"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type CancelInput struct {
	RequestID       string
	Reason          string
	ExpectedVersion int64 // 0 skips the check
	ActorID         string
}

type CancelResult struct {
	Request domain.Request
	Applied bool
}

// CancelRequest does not touch child requests (they have their own lifecycle) or task-service plans.
// Like reopen, it emits only status_changed (trigger=cancel).
type CancelRequest struct {
	repo       RequestRepository
	transition RequestTransitioner
	history    ReturnHistoryRepository
	canceller  ApprovalCanceller
	guard      ExecutionGuard
	tx         TxRunner
	clarifs    ClarificationCanceller
}

// WithClarifications closes the open Clarification when a request in awaiting_information is cancelled.
func (uc *CancelRequest) WithClarifications(c ClarificationCanceller) *CancelRequest {
	uc.clarifs = c
	return uc
}

func NewCancelRequest(repo RequestRepository, transition RequestTransitioner, history ReturnHistoryRepository,
	canceller ApprovalCanceller, guard ExecutionGuard, tx TxRunner) *CancelRequest {
	return &CancelRequest{repo: repo, transition: transition, history: history, canceller: canceller, guard: guard, tx: tx}
}

func (uc *CancelRequest) Execute(ctx context.Context, in CancelInput) (CancelResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return CancelResult{}, domain.ErrRequestTenantRequired()
	}
	var out CancelResult
	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		r, err := uc.repo.Get(ctx, in.RequestID)
		if err != nil {
			return err
		}
		// The transition table has no exit from a final status, so repeat cancels are answered here.
		if r.Status == domain.RequestStatusCancelled {
			out = CancelResult{Request: r, Applied: false}
			return nil
		}
		if r.Status == domain.RequestStatusCompleted {
			return domain.ErrCancelNotAllowed(r.Status)
		}
		if strings.TrimSpace(in.Reason) == "" {
			return domain.ErrReasonRequired()
		}
		if in.ExpectedVersion != 0 && r.Version != in.ExpectedVersion {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		if r.Status == domain.RequestStatusExecuting {
			active, err := uc.guard.HasActiveExecution(ctx, r.ID)
			if err != nil {
				return err
			}
			if active {
				return domain.ErrCancelBlockedActiveExecution(r.ID)
			}
		}
		if r.Status == domain.RequestStatusAwaitingInformation && uc.clarifs != nil {
			if _, err := uc.clarifs.CancelOpenForRequest(ctx, r.ID, "request_cancelled"); err != nil {
				return err
			}
		}
		from := r.Status
		res, err := uc.transition.Execute(ctx, TransitionInput{
			RequestID: r.ID, Trigger: domain.TriggerCancel, ExpectedFrom: &from, ActorID: in.ActorID,
			ActorKind: domain.ActorKindUser, Reason: in.Reason,
		})
		if err != nil {
			return err
		}
		if err := uc.history.Append(ctx, domain.ReturnHistoryEntry{
			RequestID: r.ID, Action: domain.ReturnActionCancelled, Reason: in.Reason, ActorID: in.ActorID, ActorKind: domain.ActorKindUser,
		}); err != nil {
			return err
		}
		if err := uc.canceller.CancelPending(ctx, r.ID, "cancelled"); err != nil {
			return err
		}
		out = CancelResult{Request: res.Request, Applied: res.Applied}
		return nil
	})
	if err != nil {
		return CancelResult{}, err
	}
	return out, nil
}
