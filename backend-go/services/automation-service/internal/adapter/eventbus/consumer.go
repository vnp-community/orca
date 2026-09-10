package eventbus

import (
	"context"
	"log/slog"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
	"github.com/stablyai/orca-go/services/automation-service/internal/usecase"
)

// durableConsumerName is the shared, stable JetStream consumer name every
// automation-service replica subscribes with — a single competing-consumer
// group, so each event is delivered to exactly ONE replica cluster-wide.
// Contrast with tenant-service/notification-service's SubscribeEphemeral,
// which is per-replica by design for cache-invalidation/WS fan-out; using
// that here would multiply-dispatch the same event across every replica —
// the opposite bug from what BL-AT-03's "each automation runs once per
// matching event" requires.
const durableConsumerName = "automation-service-event-trigger"

// SubjectBinding pairs a subject with the JetStream stream it lives on.
type SubjectBinding struct {
	StreamName string
	Subject    string
}

// Subjects maps BL-AT-03's 5 event names to real (or planned) subjects.
// worktree:created, pr:merged, and issue:assigned have no publisher yet —
// see SOL-AT-03's "Cross-service work needed" section; subscribing here is
// safe regardless (no events arrive until those publishers exist).
var Subjects = []SubjectBinding{
	{StreamName: "TASK", Subject: "orca.task.task.completed"},
	{StreamName: "TASK", Subject: "orca.task.task.failed"},
	{StreamName: "PROJECT", Subject: "orca.project.worktree.created"},
	{StreamName: "SCMINTEGRATION", Subject: "orca.scmintegration.pull_request.merged"},
	{StreamName: "SCMINTEGRATION", Subject: "orca.scmintegration.issue.assigned"},
}

// subjectToEventName maps a subject exhaustively over the 5 documented
// EventName values — panics on an unmapped subject (a Subjects entry with
// no case here is a programmer error caught by consumer_test.go's
// exhaustiveness test, not a runtime condition a live cluster can hit).
func subjectToEventName(subject string) domain.EventName {
	switch subject {
	case "orca.task.task.completed":
		return domain.EventAgentCompleted
	case "orca.task.task.failed":
		return domain.EventAgentError
	case "orca.project.worktree.created":
		return domain.EventWorktreeCreated
	case "orca.scmintegration.pull_request.merged":
		return domain.EventPRMerged
	case "orca.scmintegration.issue.assigned":
		return domain.EventIssueAssigned
	default:
		panic("eventbus: unmapped subject " + subject)
	}
}

// Consumer subscribes automation-service to the 5 event-trigger subjects
// and dispatches each delivered event through usecase.HandleEventTrigger.
type Consumer struct {
	bus      *commoneventbus.Consumer
	dispatch *usecase.HandleEventTrigger
}

func NewConsumer(bus *commoneventbus.Consumer, dispatch *usecase.HandleEventTrigger) *Consumer {
	return &Consumer{bus: bus, dispatch: dispatch}
}

// Run subscribes to every Subjects entry until ctx is cancelled — one
// goroutine per subject, matching this codebase's existing per-subject
// subscribe pattern. A subscribe failure is logged, not fatal to service
// startup (mirrors notification-service/tenant-service's graceful
// degradation posture for optional eventbus availability).
func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	for _, b := range Subjects {
		b := b
		go func() {
			// Durable (NOT SubscribeEphemeral) — see durableConsumerName's
			// doc comment for why.
			if err := c.bus.Subscribe(ctx, b.StreamName, durableConsumerName, b.Subject, func(ctx context.Context, event commoneventbus.Event) error {
				return c.dispatch.Execute(ctx, usecase.HandleEventTriggerInput{
					EventID:   event.ID,
					TenantID:  event.TenantID,
					EventName: subjectToEventName(b.Subject),
					Payload:   string(event.Payload),
				})
			}); err != nil {
				logger.ErrorContext(ctx, "automation eventbus subscribe failed", slog.Any("error", err), slog.String("subject", b.Subject))
			}
		}()
	}
}
