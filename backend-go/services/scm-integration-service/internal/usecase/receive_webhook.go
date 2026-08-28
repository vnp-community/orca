package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// ReceiveWebhookInput mirrors ReceiveWebhookRequest 1:1 (BUG-PI-03).
type ReceiveWebhookInput struct {
	Provider         domain.ScmProvider
	RawBody          []byte
	SignatureHeader  string
	DeliveryIDHeader string
}

type ReceiveWebhookOutput struct {
	Accepted  bool
	Duplicate bool
}

// ReceiveWebhook gives scm.webhook_delivery_log its first writer (BUG-PI-03:
// an externally-merged PR — merged directly on github.com, not through
// Orca — had no way to reach issue-status-sync). Only "PR/MR merged"
// events publish a lifecycle event; every other event type is still
// recorded for delivery idempotency, per this task's own scope note.
type ReceiveWebhook struct {
	verifier   WebhookVerifier
	deliveries WebhookDeliveryStore
	outbox     OutboxEnqueuer
}

func NewReceiveWebhook(verifier WebhookVerifier, deliveries WebhookDeliveryStore, outbox OutboxEnqueuer) *ReceiveWebhook {
	return &ReceiveWebhook{verifier: verifier, deliveries: deliveries, outbox: outbox}
}

func (uc *ReceiveWebhook) Execute(ctx context.Context, in ReceiveWebhookInput) (ReceiveWebhookOutput, error) {
	if !uc.verifier.Verify(ctx, in.Provider, in.RawBody, in.SignatureHeader) {
		return ReceiveWebhookOutput{}, apperrors.New(apperrors.KindPermissionDenied, "SCM_WEBHOOK_BAD_SIGNATURE", "signature verification failed", nil)
	}
	// Dedup check runs BEFORE the delivery is recorded — a duplicate
	// delivery must never enqueue a second lifecycle event, even if the
	// original delivery's own Record call is still in flight.
	if seen, err := uc.deliveries.Exists(ctx, in.Provider, in.DeliveryIDHeader); err == nil && seen {
		return ReceiveWebhookOutput{Accepted: true, Duplicate: true}, nil
	}

	parsed, isMerge := parseMergeEvent(in.Provider, in.RawBody) // only "PR/MR merged" events are relevant

	if err := uc.deliveries.Record(ctx, in.Provider, in.DeliveryIDHeader, "processed"); err != nil {
		return ReceiveWebhookOutput{}, apperrors.New(apperrors.KindInternal, "SCM_WEBHOOK_RECORD_FAILED", "failed to record webhook delivery", err)
	}

	if isMerge && uc.outbox != nil {
		event := prMergedEventFromWebhook(parsed)
		// Best-effort, same posture as CreatePullRequest's own enqueue
		// (TASK-PI-03-05) — a failed enqueue must not turn an already-
		// accepted, already-recorded webhook into a 5xx.
		_ = uc.outbox.Enqueue(ctx, parsed.TenantID, event)
	}
	return ReceiveWebhookOutput{Accepted: true}, nil
}
