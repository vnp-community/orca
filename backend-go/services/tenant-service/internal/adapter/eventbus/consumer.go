package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// StreamName is the JetStream stream tenant-service's own main.go creates
// (via Publisher's underlying commoneventbus.Publisher.EnsureStream) for
// Subject — this package is both the publisher and the consumer of the same
// subject, unlike most services' adapter/eventbus split.
const StreamName = "TENANT"

// ProfileInvalidator is the minimal slice of usecase.ProfileCache this
// consumer needs — kept narrow so this adapter package doesn't have to
// import the full usecase.ProfileCache port just to call one method.
type ProfileInvalidator interface {
	Invalidate(ctx context.Context, userID string)
}

// Consumer subscribes every tenant-service replica to Subject via
// commoneventbus.Consumer.SubscribeEphemeral — see that method's doc
// comment for why ephemeral, not Subscribe's shared-durable, is the correct
// primitive here: every replica must invalidate its OWN local cache for
// every event, not have the cluster round-robin events across replicas.
type Consumer struct {
	bus   *commoneventbus.Consumer
	cache ProfileInvalidator
}

// NewConsumer constructs a Consumer. Named distinctly from Publisher's New
// since both live in this same adapter package (tenant-service is both
// publisher and consumer of Subject, unlike most services' adapter/eventbus
// split — see this file's package-level doc in publisher.go).
func NewConsumer(bus *commoneventbus.Consumer, cache ProfileInvalidator) *Consumer {
	return &Consumer{bus: bus, cache: cache}
}

// Run subscribes until ctx is cancelled, logging (not failing service
// startup on) a subscription that ends early — matching
// notification-service's consumer's graceful-degradation posture, the
// established pattern for optional eventbus availability in this codebase.
func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	err := c.bus.SubscribeEphemeral(ctx, StreamName, Subject, func(ctx context.Context, event commoneventbus.Event) error {
		var payload invalidatedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			// A permanently-unparseable payload will never succeed on
			// redelivery — log and ack (return nil) rather than NAK forever,
			// same reasoning as notification-service's HandleIncomingEvent
			// for its own malformed-payload case.
			logger.WarnContext(ctx, "malformed profile-invalidated payload, dropping", slog.Any("error", err))
			return nil
		}
		c.cache.Invalidate(ctx, payload.UserID)
		return nil
	})
	if err != nil {
		logger.WarnContext(ctx, "profile-invalidation subscription ended", slog.Any("error", err))
	}
}
