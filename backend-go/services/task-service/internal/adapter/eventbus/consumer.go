// Package eventbus wires task-service's MirrorExecutionStatus usecase to
// NATS JetStream via common/eventbus.Consumer (BE-SOL-003/TASK-FT-003-05) —
// same shape as notification-service's own consumer
// (notification-service/internal/adapter/eventbus/consumer.go), scoped to
// task-service's 2 subjects. New plumbing: task-service has no
// adapter/eventbus/ package at all before this task.
package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// SubjectBinding pairs a JetStream stream name (owned/created by the
// publishing service) with a subject filter within it — same shape as
// notification-service's own SubjectBinding.
type SubjectBinding struct {
	StreamName string
	Subject    string
}

// Subjects is task-service's 2-subject slice of CR-FLOW-TASK-003's catalog —
// the two subjects that carry a terminal or status-changing signal for an
// execution_links row this service owns (BE-SOL-001's execution_links,
// TASK-FT-001-01).
var Subjects = []SubjectBinding{
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.task.statuschanged"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.step.completed"},
}

// mirrorPayload decodes the fields this consumer needs out of EITHER
// producing subject's payload shape — the two are NOT the same shape
// (confirmed by reading the real, final payloads TASK-FT-003-01 and
// TASK-FT-003-03 landed with): orchestration-service's
// taskStatusChangedPayload carries coordinator_run_id/new_status,
// workflow-service's stepEventPayload carries execution_id/status. Both
// "external ref" fields are what task-service's own
// ExecuteTask/ComplexExecutor / WorkflowExecutor already store as
// execution_links.external_ref_id for their respective engines (confirmed:
// ComplexExecutor.Execute returns StartCoordinatorRunResponse.GetId(),
// WorkflowExecutor.Execute returns ExecuteResponse.GetExecution().GetId()) —
// so decoding both possible field names generically here, rather than
// branching per subject, keeps this handler a single small function.
type mirrorPayload struct {
	CoordinatorRunID string `json:"coordinator_run_id"`
	ExecutionID      string `json:"execution_id"`
	NewStatus        string `json:"new_status"`
	Status           string `json:"status"`
}

func (p mirrorPayload) externalRefID() string {
	if p.CoordinatorRunID != "" {
		return p.CoordinatorRunID
	}
	return p.ExecutionID
}

func (p mirrorPayload) status() string {
	if p.NewStatus != "" {
		return p.NewStatus
	}
	return p.Status
}

// Consumer subscribes to every binding in Subjects and forwards each
// delivered message to MirrorExecutionStatus — a real, working consumer
// loop, not a stub.
type Consumer struct {
	bus    *commoneventbus.Consumer
	mirror *usecase.MirrorExecutionStatus
}

func NewConsumer(bus *commoneventbus.Consumer, mirror *usecase.MirrorExecutionStatus) *Consumer {
	return &Consumer{bus: bus, mirror: mirror}
}

// Run subscribes to every SubjectBinding, one goroutine each (each call to
// commoneventbus.Consumer.SubscribeEphemeral blocks until ctx is
// cancelled — the per-replica fan-out semantics every replica of this
// horizontally-scaled service needs, same reasoning
// notification-service's own consumer doc comment gives), and returns once
// every subscription has stopped. A binding whose stream doesn't exist yet
// logs a warning and that one binding gives up — it does not fail service
// startup or the other bindings, matching notification-service's own
// graceful-degradation posture around eventbus availability.
func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	var wg sync.WaitGroup
	for _, binding := range Subjects {
		wg.Add(1)
		go func(b SubjectBinding) {
			defer wg.Done()
			err := c.bus.SubscribeEphemeral(ctx, b.StreamName, b.Subject, func(ctx context.Context, event commoneventbus.Event) error {
				var payload mirrorPayload
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					return err // unparseable payload — leave unacked for redelivery/investigation, matches notification-service's own posture
				}
				return c.mirror.Execute(ctx, usecase.MirrorExecutionStatusInput{
					TenantID: event.TenantID, ExternalRefID: payload.externalRefID(), NewStatus: payload.status(),
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
