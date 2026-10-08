package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type CancelClarificationInput struct {
	ClarificationID string
	Reason          string
	ExpectedVersion int64
}

// CancelClarification closes an open clarification by hand. Without the missing facts the request cannot go on,
// so it returns to the backlog with category missing_info (an interpretation; the CR is silent, see open question 1).
type CancelClarification struct {
	repo           RequestRepository
	clarifications ClarificationRepository
	returner       *ReturnRequestToBacklog
	tx             TxRunner
	outbox         OutboxWriter
	clock          func() time.Time
}

func NewCancelClarification(repo RequestRepository, clarifications ClarificationRepository, returner *ReturnRequestToBacklog, tx TxRunner, outbox OutboxWriter) *CancelClarification {
	return &CancelClarification{repo: repo, clarifications: clarifications, returner: returner, tx: tx, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *CancelClarification) Execute(ctx context.Context, in CancelClarificationInput) (domain.Clarification, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Clarification{}, domain.ErrRequestTenantRequired()
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return domain.Clarification{}, domain.ErrReasonRequired()
	}
	var out domain.Clarification
	err := uc.tx.InTx(ctx, func(txCtx context.Context) error {
		c, err := uc.clarifications.Get(txCtx, in.ClarificationID)
		if err != nil {
			return err
		}
		r, err := uc.repo.Get(txCtx, c.RequestID)
		if err != nil {
			return err
		}
		if callerIsMachine(txCtx) || !(canWriteRequest(txCtx, r) || (callerID(txCtx) != "" && callerID(txCtx) == c.CreatedBy)) {
			return errRequestForbidden("only the reporter, the person who asked, or an admin may cancel a clarification")
		}
		if c.Status != domain.ClarificationStatusOpen {
			return domain.ErrClarificationNotOpen(c.ID, c.Status)
		}
		now := uc.clock()
		if err := uc.clarifications.MarkCancelled(txCtx, c.ID, reason, now, in.ExpectedVersion); err != nil {
			return err
		}
		ev, err := NewOutboxEvent(txCtx, domain.SubjectClarificationCancelled, ClarificationEventPayload{
			ClarificationID: c.ID, RequestID: r.ID, DisplayID: c.DisplayID(r.Number), Reason: reason,
		})
		if err != nil {
			return err
		}
		ev.OccurredAt = now
		if err := uc.outbox.InsertOutboxEvent(txCtx, ev); err != nil {
			return err
		}
		if _, err := uc.returner.Execute(txCtx, ReturnInput{
			RequestID: r.ID, Stage: domain.ReturnStageForResume(c.Source, c.ResumeStatus), Category: domain.ReturnCategoryMissingInfo,
			Reason: reason, ActorID: callerID(txCtx), ActorKind: domain.ActorKindUser,
		}); err != nil {
			return err
		}
		out, err = uc.clarifications.Get(txCtx, c.ID)
		return err
	})
	return out, err
}
