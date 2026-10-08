package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ClarificationCanceller closes the open Clarification of a request that is leaving awaiting_information
// through another door (type change, cancel, return to backlog). It joins the caller's transaction.
type ClarificationCanceller interface {
	CancelOpenForRequest(ctx context.Context, requestID, reason string) (cancelled bool, err error)
}

// OpenClarificationCanceller is the production ClarificationCanceller.
type OpenClarificationCanceller struct {
	clarifications ClarificationRepository
	requests       RequestRepository
	outbox         OutboxWriter
	clock          func() time.Time
}

func NewOpenClarificationCanceller(clarifications ClarificationRepository, requests RequestRepository, outbox OutboxWriter) *OpenClarificationCanceller {
	return &OpenClarificationCanceller{clarifications: clarifications, requests: requests, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

var _ ClarificationCanceller = (*OpenClarificationCanceller)(nil)

// ClarificationEventPayload is the wire shape of orca.request.clarification.answered/cancelled (never question or answer text).
type ClarificationEventPayload struct {
	ClarificationID string `json:"clarification_id"`
	RequestID       string `json:"request_id"`
	DisplayID       string `json:"display_id"`
	Reason          string `json:"reason,omitempty"`
}

func (c *OpenClarificationCanceller) CancelOpenForRequest(ctx context.Context, requestID, reason string) (bool, error) {
	open, err := c.clarifications.GetOpenByRequest(ctx, requestID)
	if err != nil || open == nil {
		return false, err
	}
	now := c.clock()
	if err := c.clarifications.MarkCancelled(ctx, open.ID, reason, now, 0); err != nil {
		return false, err
	}
	r, err := c.requests.Get(ctx, requestID)
	if err != nil {
		return false, err
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectClarificationCancelled, ClarificationEventPayload{
		ClarificationID: open.ID, RequestID: requestID, DisplayID: open.DisplayID(r.Number), Reason: reason,
	})
	if err != nil {
		return false, err
	}
	ev.OccurredAt = now
	return true, c.outbox.InsertOutboxEvent(ctx, ev)
}
