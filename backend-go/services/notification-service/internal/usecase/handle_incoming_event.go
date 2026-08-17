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
	broadcaster NotificationBroadcaster
	logger      *slog.Logger
}

func NewHandleIncomingEvent(broadcaster NotificationBroadcaster, logger *slog.Logger) *HandleIncomingEvent {
	if logger == nil {
		logger = slog.Default()
	}
	return &HandleIncomingEvent{broadcaster: broadcaster, logger: logger}
}

// Execute translates in into a NotificationEvent and broadcasts it. A
// payload naming no recipient (domain.ErrNoRecipients) is logged and
// treated as a successful no-op — not every event needs a route to a
// user, and NAK-ing (via a returned error) would just cause pointless
// JetStream redelivery of a message that will never translate
// differently. Any other error is returned so the eventbus adapter NAKs
// for redelivery.
func (uc *HandleIncomingEvent) Execute(ctx context.Context, in HandleIncomingEventInput) error {
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
	return nil
}
