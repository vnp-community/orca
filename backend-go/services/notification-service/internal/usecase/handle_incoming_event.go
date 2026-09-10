package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// HandleIncomingEventInput is a subject-agnostic view of one delivered bus
// message — deliberately not github.com/stablyai/orca-go/common/eventbus.Event
// itself, so this usecase stays decoupled from the NATS-specific envelope
// type (only internal/adapter/eventbus/ knows that type exists), per
// architecture/03's port/adapter boundary.
type HandleIncomingEventInput struct {
	EventID    string
	TenantID   string
	Subject    string
	OccurredAt time.Time
	Payload    []byte
}

// HandleIncomingEvent is notification-service's core consumer path — the
// "primary consumer of the async event bus" role from notification-service.md
// §1/§3: translate a consumed domain event into a NotificationEvent, then
// fan it out via NotificationBroadcaster. This service never republishes
// or claims authority over the source fact (§2) — it only derives an
// ephemeral, user-facing artifact from it.
type HandleIncomingEvent struct {
	broadcaster     NotificationBroadcaster
	processedEvents ProcessedEventRepository
	// deliverPush is nil-safe (BL-MB-02 push pipeline is additive to the
	// pre-existing WS-only broadcast path) — a nil deliverPush just skips
	// push delivery entirely, e.g. in a test that only cares about WS
	// fan-out.
	deliverPush *DeliverPush
	logger      *slog.Logger
}

func NewHandleIncomingEvent(broadcaster NotificationBroadcaster, processedEvents ProcessedEventRepository, deliverPush *DeliverPush, logger *slog.Logger) *HandleIncomingEvent {
	if logger == nil {
		logger = slog.Default()
	}
	return &HandleIncomingEvent{broadcaster: broadcaster, processedEvents: processedEvents, deliverPush: deliverPush, logger: logger}
}

// Execute translates in into a NotificationEvent and broadcasts it. A
// payload naming no recipient (domain.ErrNoRecipients) is logged and
// treated as a successful no-op — not every event needs a route to a
// user, and NAK-ing (via a returned error) would just cause pointless
// JetStream redelivery of a message that will never translate
// differently. Any other error is returned so the eventbus adapter NAKs
// for redelivery.
//
// Dedup runs first, per notification-service.md §5/§8: JetStream's
// at-least-once delivery (plus SubscribeEphemeral giving every replica its
// own independent consumer, docs/execution-plan.md Epic F) means the same
// event ID can arrive here more than once, concurrently, across replicas.
// ProcessedEventRepository.MarkProcessed atomically reserves the event ID —
// a redelivery/race loser is a successful no-op, not an error, so it isn't
// NAK'd back into another redelivery loop.
func (uc *HandleIncomingEvent) Execute(ctx context.Context, in HandleIncomingEventInput) error {
	alreadyProcessed, err := uc.processedEvents.MarkProcessed(ctx, in.EventID, in.Subject)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_DEDUP_CHECK_FAILED", "failed to record event dedup state", err)
	}
	if alreadyProcessed {
		uc.logger.DebugContext(ctx, "skipping already-processed event (JetStream redelivery)",
			slog.String("subject", in.Subject), slog.String("event_id", in.EventID))
		return nil
	}

	payload, err := domain.DecodePayload(in.Payload)
	if err != nil {
		return apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_MALFORMED_PAYLOAD", "failed to decode event payload", err)
	}

	event, err := domain.TranslateEvent(uuid.NewString(), in.EventID, in.Subject, in.TenantID, payload, in.OccurredAt)
	if err != nil {
		if errors.Is(err, domain.ErrNoRecipients) {
			uc.logger.InfoContext(ctx, "skipping event with no recipient",
				slog.String("subject", in.Subject), slog.String("event_id", in.EventID))
			return nil
		}
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_TRANSLATE_FAILED", "failed to translate event", err)
	}

	uc.broadcaster.Broadcast(ctx, event)

	// Push delivery (BL-MB-02) is best-effort and additive to WS fan-out
	// above — a push-delivery hiccup must never NAK the whole event back
	// into JetStream redelivery (DeliverPush.Execute already never
	// returns an error itself; this guard is just defense in depth plus
	// the nil-safety for callers/tests that don't wire push delivery).
	if uc.deliverPush != nil {
		if err := uc.deliverPush.Execute(ctx, event); err != nil {
			uc.logger.WarnContext(ctx, "deliver_push failed for event",
				slog.String("subject", in.Subject), slog.String("event_id", in.EventID), slog.Any("error", err))
		}
	}

	return nil
}
