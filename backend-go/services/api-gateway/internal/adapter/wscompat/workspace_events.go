// Package wscompat (workspace_events.go) — bridges task-service's
// orca.task.task.statuschanged and workflow-service's
// orca.workflow.execution.completed/.failed outbox events (TASK-PW-04-03/06)
// to connected WS sessions via workspace.subscribe, the direct backend
// counterpart to BL-PW-04's frontend-side workspaceEvents sketch.
// 08-inter-service-communication.md's API Gateway responsibility #5
// ("Manages WebSocket sessions for real-time surfaces") already names
// this as api-gateway's job.
//
// Grounding correction versus SOL-PW-04's own sketch (TASK-PW-04-07): that
// solution doc proposes a bespoke wsSessionRegistry/RegisterWorkspaceEventBridge
// pair; this file instead extends channels_push.go's ClientEventBus shape
// (Subscribe/Publish, already an in-process pub/sub feeding PushEvents to
// Registry.RegisterStream-backed channels — see runtime.clientEvents.subscribe
// for the precedent) with per-subscriber project-ID filtering, rather than
// inventing a new registry type from scratch.
//
// Out of scope (disclosed, not silently dropped): Flow 3's "ref-sync fetch
// on a create-pr-shaped step" is NOT implemented here — TASK-PW-04-06
// already established that WorkflowExecution carries no step-level
// step_type field to filter on; that half of Flow 3 needs its own
// follow-up once workflow-service exposes step-level data on the wire.
package wscompat

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// WorkspaceEventBus fans out task/workflow outbox events to every
// connection subscribed to a given projectId — same in-process pub/sub
// shape as ClientEventBus (channels_push.go), extended with per-subscriber
// project filtering since, unlike runtime.clientEvents, most subscribers
// only care about ONE project at a time.
type WorkspaceEventBus struct {
	mu   sync.Mutex
	subs map[chan PushEvent]string // chan -> projectId filter
}

func NewWorkspaceEventBus() *WorkspaceEventBus {
	return &WorkspaceEventBus{subs: make(map[chan PushEvent]string)}
}

func (b *WorkspaceEventBus) Subscribe(projectID string) (<-chan PushEvent, func()) {
	ch := make(chan PushEvent, 16)
	b.mu.Lock()
	b.subs[ch] = projectID
	b.mu.Unlock()
	unsubscribe := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

func (b *WorkspaceEventBus) publish(projectID string, ev PushEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch, want := range b.subs {
		if want != projectID {
			continue
		}
		select {
		case ch <- ev:
		default: // slow subscriber — drop rather than block Publish
		}
	}
}

// RegisterWorkspaceSubscribeChannel wires the workspace.subscribe channel
// against bus. Exported (unlike this package's other register* helpers,
// which main.go reaches only via the RegisterRealChannels/RegisterPushChannels
// composition functions) since main.go's composition root constructs
// WorkspaceEventBus itself, mirroring RegisterPushChannels's ClientEventBus
// precedent.
func RegisterWorkspaceSubscribeChannel(r *Registry, bus *WorkspaceEventBus) {
	r.RegisterStream("workspace.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		type subscribeArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[subscribeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ch, unsubscribe := bus.Subscribe(in.ProjectID)
		go func() { <-ctx.Done(); unsubscribe() }()
		return ch, nil
	})
}

type taskStatusChangedPayload struct {
	TaskID         string `json:"task_id"`
	ProjectID      string `json:"project_id"`
	WorktreeID     string `json:"worktree_id"`
	PreviousStatus string `json:"previous_status"`
	NewStatus      string `json:"new_status"`
}

type workflowExecutionTerminalPayload struct {
	ExecutionID string `json:"execution_id"`
	TemplateID  string `json:"template_id"`
	ProjectID   string `json:"project_id"`
	Status      string `json:"status"`
}

// RunWorkspaceEventBridge subscribes ONE shared, per-replica ephemeral
// consumer per subject (not per-connection — every replica must
// independently learn about the event to push it to whichever
// locally-held WS connections it owns, per
// commoneventbus.Consumer.SubscribeEphemeral's own fan-out-vs-competing-
// consumer doc comment) and republishes onto bus, filtered by the event's
// project_id at delivery time. Call once from main.go's composition root,
// in its own goroutine (blocks until ctx is cancelled), alongside
// notification-service's identical pattern
// (internal/adapter/eventbus/consumer.go's Consumer.Run — this is the same
// shape, second independent consumer of the same two subjects).
func RunWorkspaceEventBridge(ctx context.Context, consumer *commoneventbus.Consumer, bus *WorkspaceEventBus, logger *slog.Logger) {
	// Three FilterSubject values across two JetStream streams need three
	// SubscribeEphemeral calls (one per subject) — commoneventbus.Consumer
	// has no multi-subject variant (see notification-service's own
	// Consumer.Run, which does the identical one-goroutine-per-SubjectBinding
	// fan-out for its 7 subjects), so this mirrors that established pattern
	// rather than inventing a new one.
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		err := consumer.SubscribeEphemeral(ctx, "TASK", "orca.task.task.statuschanged", func(ctx context.Context, ev commoneventbus.Event) error {
			var payload taskStatusChangedPayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				return err
			}
			bus.publish(payload.ProjectID, PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "task.statuschanged", "payload": payload}}})
			return nil
		})
		if err != nil {
			logger.WarnContext(ctx, "workspace event bridge: task subscription ended", slog.Any("error", err))
		}
	}()
	runWorkflowTerminalSubscription := func(subject, eventType string) {
		defer wg.Done()
		err := consumer.SubscribeEphemeral(ctx, "WORKFLOW", subject, func(ctx context.Context, ev commoneventbus.Event) error {
			var payload workflowExecutionTerminalPayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				return err
			}
			bus.publish(payload.ProjectID, PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": eventType, "payload": payload}}})
			return nil
		})
		if err != nil {
			logger.WarnContext(ctx, "workspace event bridge: workflow subscription ended", slog.String("subject", subject), slog.Any("error", err))
		}
	}
	go runWorkflowTerminalSubscription("orca.workflow.execution.completed", "workflow.execution.completed")
	go runWorkflowTerminalSubscription("orca.workflow.execution.failed", "workflow.execution.failed")
	wg.Wait()
}
