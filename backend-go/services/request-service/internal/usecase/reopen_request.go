package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ReopenInput struct {
	RequestID       string
	Note            string
	ExpectedVersion int64 // 0 skips the check
	ActorID         string
}

// ReopenRequest sends a backlog request back to classification. No `reopened` event exists
// (README v6 section 8 point 4): consumers see status_changed with trigger=reopen.
type ReopenRequest struct {
	repo       RequestRepository
	transition RequestTransitioner
	history    ReturnHistoryRepository
	attempts   ClassificationAttemptsResetter // nil until CR-REQ-005 adds the counter
	tx         TxRunner
}

func NewReopenRequest(repo RequestRepository, transition RequestTransitioner, history ReturnHistoryRepository,
	attempts ClassificationAttemptsResetter, tx TxRunner) *ReopenRequest {
	return &ReopenRequest{repo: repo, transition: transition, history: history, attempts: attempts, tx: tx}
}

func (uc *ReopenRequest) Execute(ctx context.Context, in ReopenInput) (domain.Request, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, domain.ErrRequestTenantRequired()
	}
	var out domain.Request
	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		r, err := uc.repo.Get(ctx, in.RequestID)
		if err != nil {
			return err
		}
		if r.Status != domain.RequestStatusRequestBacklog {
			return domain.ErrReopenNotAllowed(r.Status)
		}
		if in.ExpectedVersion != 0 && r.Version != in.ExpectedVersion {
			return domain.ErrRequestVersionConflict(r.ID, in.ExpectedVersion)
		}
		from := r.Status
		res, err := uc.transition.Execute(ctx, TransitionInput{
			RequestID: r.ID, Trigger: domain.TriggerReopen, ExpectedFrom: &from, ActorID: in.ActorID, ActorKind: domain.ActorKindUser,
		})
		if err != nil {
			return err
		}
		// A user reopening asks for a fresh AI proposal, so the attempt budget starts over.
		if uc.attempts != nil {
			if err := uc.attempts.ResetClassificationAttempts(ctx, r.ID); err != nil {
				return err
			}
		}
		if err := uc.history.Append(ctx, domain.ReturnHistoryEntry{
			RequestID: r.ID, Action: domain.ReturnActionReopened, Reason: in.Note, ActorID: in.ActorID, ActorKind: domain.ActorKindUser,
		}); err != nil {
			return err
		}
		out = res.Request
		if uc.attempts != nil { // the reset bumped the version: answer with the stored row
			if out, err = uc.repo.Get(ctx, r.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Request{}, err
	}
	return out, nil
}
