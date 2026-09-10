// task.activity's push-capable channel (BE-SOL-003/TASK-FT-003-04) — kept
// in this SEPARATE file (not appended to channels_push.go), same
// isolation-from-high-churn-files reasoning that file's own doc comment
// already gives.
//
// Grounding correction versus SOL-PW-04's own sketch, already identified by
// BE-SOL-003 and confirmed here by reading the real code directly:
// SOL-PW-04 invents a wsSessionRegistry/RegisterWorkspaceEventBridge pair
// from scratch. api-gateway's wscompat layer has since gained a real,
// working, general-purpose mechanism for exactly this shape of problem —
// StreamHandler/PushEvent (push_bridge.go) plus Registry.RegisterStream
// (registry.go), with registerNotificationStreamChannel (channels_push.go)
// as the real, shipped precedent using both. task.activity differs from
// that precedent in one way: its source is not one gRPC stream but multiple
// NATS subjects (4 orchestration.* + 2 workflow.step.* + 1
// orca.task.agent_output_partial, TASK-AG-FLOWTASK-003), filtered by
// task_id per connection — this channel therefore combines RegisterStream
// with a direct commoneventbus.Consumer.SubscribeEphemeral per subject (the
// same per-replica-fan-out mechanism notification-service's own consumer
// uses), rather than introducing a third mechanism.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// TaskActivityFrame is task.activity.event's wire payload — one entry per
// orchestration/workflow-step/direct-agent event matching the subscribed
// taskId.
type TaskActivityFrame struct {
	TaskID string `json:"taskId"`
	// Engine is "orchestration" | "workflow" | "direct_agent"
	// (BE-SOL-001's domain.ExecutionEngine). direct_agent's only event
	// source is TASK-AG-FLOWTASK-003's throttled agent_output_partial
	// (orca.task.agent_output_partial, published by task-service's
	// SimpleExecutor) — Engine 1 is otherwise fully synchronous end to end,
	// this is the one deliberate exception, not a general async event
	// source for it (corrects this field's own prior "direct_agent never
	// appears here" claim, true before this pass).
	Engine string `json:"engine"`
	// EventType — "status_changed" | "message_posted" | "step_completed" |
	// "step_failed" | "decision_gate_opened" | "agent_output_partial".
	EventType  string          `json:"eventType"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurredAt"`
}

type taskActivitySubject struct {
	StreamName string
	Subject    string
}

// taskActivitySubjects is the closed set of subjects this channel
// forwards — 4 orchestration.* (TASK-FT-003-01/-02) + 2 workflow.step.*
// (TASK-FT-003-03) + 1 task.* (TASK-AG-FLOWTASK-003 — task-service's first
// published event family, its own outbox relay, "TASK" stream, see that
// service's cmd/server/main.go).
var taskActivitySubjects = []taskActivitySubject{
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.task.dispatched"},
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.task.statuschanged"},
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.message.posted"},
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.decision_gate.opened"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.step.completed"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.step.failed"},
	{StreamName: "TASK", Subject: "orca.task.agent_output_partial"},
}

// ephemeralSubscriber is the minimal slice of *commoneventbus.Consumer this
// channel needs — a real *commoneventbus.Consumer satisfies it structurally
// (Go interfaces, no explicit implements), so main.go's composition root
// passes one unchanged; test code passes a fake, since
// *commoneventbus.Consumer is a concrete struct with no NATS-free
// substitute of its own.
type ephemeralSubscriber interface {
	SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error
}

// RegisterTaskActivityStreamChannel registers task.activity.subscribe — see
// this file's doc comment for why this combines RegisterStream with a
// direct per-subject SubscribeEphemeral rather than SOL-PW-04's
// from-scratch wsSessionRegistry sketch.
func RegisterTaskActivityStreamChannel(r *Registry, bus ephemeralSubscriber) {
	r.RegisterStream("task.activity.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		type subscribeArgs struct {
			TaskID string `json:"taskId"`
		}
		a, err := decodeArg[subscribeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if a.TaskID == "" {
			// wscompat's channels_*.go convention is plain fmt.Errorf, not
			// common/apperrors (that package's Kind/gRPC-status mapping is
			// for the gRPC-facing services this gateway calls into, not
			// this WS-facing layer itself — confirmed: no channels_*.go
			// file in this package imports apperrors).
			return nil, fmt.Errorf("task.activity.subscribe: taskId is required")
		}

		out := make(chan PushEvent)
		go func() {
			defer close(out)
			var wg sync.WaitGroup
			for _, sub := range taskActivitySubjects {
				sub := sub
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = bus.SubscribeEphemeral(ctx, sub.StreamName, sub.Subject, func(ctx context.Context, ev commoneventbus.Event) error {
						frame, ok := translateToTaskActivity(sub.Subject, ev, a.TaskID)
						if !ok {
							return nil
						}
						select {
						case out <- PushEvent{Channel: "task.activity.event", Args: []any{frame}}:
						case <-ctx.Done():
						}
						return nil
					})
				}()
			}
			wg.Wait()
		}()
		return out, nil
	})
}

// translateToTaskActivity decodes ev's payload and reports whether it
// matches taskID — filtering by origin_task_id, which every one of the 7
// subjects' payloads carries directly (TASK-FT-003-01/-02/-03,
// TASK-AG-FLOWTASK-003's agentOutputPartialPayload), so this function needs
// no cross-service lookup of its own.
func translateToTaskActivity(subject string, ev commoneventbus.Event, taskID string) (TaskActivityFrame, bool) {
	var origin struct {
		OriginTaskID string `json:"origin_task_id"`
	}
	if err := json.Unmarshal(ev.Payload, &origin); err != nil || origin.OriginTaskID == "" || origin.OriginTaskID != taskID {
		return TaskActivityFrame{}, false
	}
	engine, eventType := classifySubject(subject)
	return TaskActivityFrame{
		TaskID: taskID, Engine: engine, EventType: eventType,
		Payload: ev.Payload, OccurredAt: ev.OccurredAt,
	}, true
}

func classifySubject(subject string) (engine, eventType string) {
	switch subject {
	case "orca.orchestration.task.dispatched":
		return "orchestration", "status_changed"
	case "orca.orchestration.task.statuschanged":
		return "orchestration", "status_changed"
	case "orca.orchestration.message.posted":
		return "orchestration", "message_posted"
	case "orca.orchestration.decision_gate.opened":
		return "orchestration", "decision_gate_opened"
	case "orca.workflow.step.completed":
		return "workflow", "step_completed"
	case "orca.workflow.step.failed":
		return "workflow", "step_failed"
	case "orca.task.agent_output_partial":
		return "direct_agent", "agent_output_partial"
	default:
		return "", ""
	}
}
