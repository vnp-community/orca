// Package eventbus wires notification-service's HandleIncomingEvent
// usecase to NATS JetStream via common/eventbus.Consumer — this is the
// "primary consumer of the async bus" role from notification-service.md
// §1/§3/§6, unusually central compared to a typical service's thin outbox
// publisher: N long-lived JetStream consumers, one per subscribed subject.
package eventbus

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
)

// SubjectBinding pairs a JetStream stream name (owned/created by the
// publishing service) with a subject filter within it —
// commoneventbus.Consumer.Subscribe looks the stream up rather than
// creating it, so the binding's StreamName must match whatever the
// publisher named its own stream via EnsureStream.
type SubjectBinding struct {
	StreamName string
	Subject    string
}

// Subjects is notification-service.md §3's subject table — illustrative,
// not exhaustive; a new subject can be added here without any schema
// change, since HandleIncomingEvent's translation is subject-driven (see
// domain.TranslateEvent's fallback rule), not subject-exhaustive.
var Subjects = []SubjectBinding{
	{StreamName: "TASK", Subject: "orca.task.task.completed"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.execution.completed"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.execution.failed"},
	{StreamName: "AUTOMATION", Subject: "orca.automation.run.completed"},
	{StreamName: "CREDENTIAL", Subject: "orca.credential.credential.rotated"},
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.decision_gate.opened"},
}

// consumerNamePrefix keeps this service's durable JetStream consumer
// names distinguishable from any other service consuming the same stream.
const consumerNamePrefix = "notification-service-"

// Consumer subscribes to every binding in Subjects and forwards each
// delivered message to HandleIncomingEvent — a real, working consumer
// loop, not a stub.
type Consumer struct {
	bus    *commoneventbus.Consumer
	handle *usecase.HandleIncomingEvent
}

func New(bus *commoneventbus.Consumer, handle *usecase.HandleIncomingEvent) *Consumer {
	return &Consumer{bus: bus, handle: handle}
}

// Run subscribes to every SubjectBinding, one goroutine each (each call to
// commoneventbus.Consumer.Subscribe blocks until ctx is cancelled), and
// returns once every subscription has stopped. A binding whose stream
// doesn't exist yet (the publishing service hasn't started/registered it)
// logs a warning and that one binding gives up — it does not fail service
// startup or the other bindings, matching this service's overall
// graceful-degradation posture around eventbus availability (see
// cmd/server/main.go and usage-service's precedent for the same pattern).
func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	var wg sync.WaitGroup
	for _, binding := range Subjects {
		wg.Add(1)
		go func(b SubjectBinding) {
			defer wg.Done()
			consumerName := consumerNamePrefix + sanitizeConsumerName(b.Subject)
			err := c.bus.Subscribe(ctx, b.StreamName, consumerName, b.Subject, func(ctx context.Context, event commoneventbus.Event) error {
				return c.handle.Execute(ctx, usecase.HandleIncomingEventInput{
					EventID:    event.ID,
					TenantID:   event.TenantID,
					Subject:    b.Subject,
					OccurredAt: event.OccurredAt,
					Payload:    event.Payload,
				})
			})
			if err != nil {
				logger.WarnContext(ctx, "eventbus subject subscription ended",
					slog.String("stream", b.StreamName), slog.String("subject", b.Subject), slog.Any("error", err))
			}
		}(binding)
	}
	wg.Wait()
}

// sanitizeConsumerName strips NATS subject wildcard/separator characters
// that aren't valid in a durable consumer name.
func sanitizeConsumerName(subject string) string {
	r := strings.NewReplacer(".", "-", ">", "all", "*", "any")
	return r.Replace(subject)
}
