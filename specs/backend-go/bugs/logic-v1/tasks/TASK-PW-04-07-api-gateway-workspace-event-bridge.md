# TASK-PW-04-07: `api-gateway` — `workspace.subscribe` push channel bridging task/workflow outbox events to WS

**From Solution:** SOL-PW-04
**Priority:** P1 — this is the dependency `TASK-PW-02-05` (file-explorer auto-refresh) and BUG-PW-04's Flow 1/3 both need
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/workspace_events.go` (new), `backend-go/services/api-gateway/cmd/server/main.go`
**Depends on:** TASK-PW-04-04, TASK-PW-04-06
**Status:** `[ ]` TODO

---

## Context

**Grounding correction versus SOL-PW-04's own sketch**: SOL-PW-04
proposes a bespoke `wsSessionRegistry`/`RegisterWorkspaceEventBridge`
function. `api-gateway` already has the exact reusable primitive this
needs: `channels_push.go`'s `ClientEventBus` (`Subscribe()`/`Publish()`,
already an in-process pub/sub feeding `PushEvent`s to
`Registry.RegisterStream`-backed channels — see `runtime.clientEvents.subscribe`
for the precedent). This task extends that shape with per-subscriber
project-ID filtering rather than inventing a new registry type from
scratch. It also does **not** implement Flow 3's "ref-sync fetch on a
create-pr-shaped step" — TASK-PW-04-06 already established that
`WorkflowExecution` carries no step-level `step_type` field to filter on;
that half of Flow 3 needs its own follow-up once workflow-service exposes
step-level data on the wire, and must not be silently dropped from this
task's scope without saying so (this note IS that disclosure).

## Changes to make

New `internal/adapter/wscompat/workspace_events.go`:

```go
// Package wscompat (workspace_events.go) — bridges task-service's
// orca.task.task.statuschanged and workflow-service's
// orca.workflow.execution.completed/.failed outbox events (TASK-PW-04-03/06)
// to connected WS sessions via workspace.subscribe, the direct backend
// counterpart to BL-PW-04's frontend-side workspaceEvents sketch.
// 08-inter-service-communication.md's API Gateway responsibility #5
// ("Manages WebSocket sessions for real-time surfaces") already names
// this as api-gateway's job.
package wscompat

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// WorkspaceEventBus fans out task/workflow outbox events to every
// connection subscribed to a given projectId — same in-process
// pub/sub shape as ClientEventBus (channels_push.go), extended with
// per-subscriber project filtering since, unlike runtime.clientEvents,
// most subscribers only care about ONE project at a time.
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

func registerWorkspaceSubscribeChannel(r *Registry, bus *WorkspaceEventBus) {
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
// consumer doc comment) and republishes onto bus, filtered by the
// event's project_id at delivery time. Call once from main.go's
// composition root, in its own goroutine (blocks until ctx is
// cancelled), alongside notification-service's identical pattern
// (internal/adapter/eventbus/consumer.go's Consumer.Run — this is the
// same shape, second independent consumer of the same two subjects).
func RunWorkspaceEventBridge(ctx context.Context, consumer *commoneventbus.Consumer, bus *WorkspaceEventBus, logger *slog.Logger) {
	var wg sync.WaitGroup
	wg.Add(2)
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
	go func() {
		defer wg.Done()
		// Two FilterSubject values on one JetStream stream need two
		// SubscribeEphemeral calls (one per subject) — verify
		// commoneventbus.Consumer has no multi-subject variant before
		// assuming this is the only way; if it does, prefer that instead
		// of two goroutines here.
		err := consumer.SubscribeEphemeral(ctx, "WORKFLOW", "orca.workflow.execution.completed", func(ctx context.Context, ev commoneventbus.Event) error {
			var payload workflowExecutionTerminalPayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				return err
			}
			bus.publish(payload.ProjectID, PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "workflow.execution.completed", "payload": payload}}})
			return nil
		})
		if err != nil {
			logger.WarnContext(ctx, "workspace event bridge: workflow subscription ended", slog.Any("error", err))
		}
	}()
	wg.Wait()
}
```

In `cmd/server/main.go`, alongside the existing `RegisterPushChannels`
call: construct a `WorkspaceEventBus`, call
`registerWorkspaceSubscribeChannel(registry, bus)`, dial a
`commoneventbus.Consumer` the same way `notification-service`'s main.go
does (`eventbus.Connect`), and start `RunWorkspaceEventBridge` in its own
goroutine. Graceful-degradation posture matches every other eventbus
consumer in this codebase: NATS unavailable at startup logs a warning,
does not fail service startup.

**Frontend note (out of this task's backend-go scope, recorded so the
frame shape is unambiguous to whoever wires the frontend side):** the
frontend's file-explorer auto-refresh (`TASK-PW-02-05`) is expected to
call `workspace.subscribe({projectId})` once per open project, listen for
`workspace.event` push frames, and call the existing
`workspace.refreshFileTree` channel on receipt — same trigger shape
BUG-PW-02 already names as the missing piece.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -run 'TestWorkspaceSubscribe|TestWorkspaceEventBus' -v
```

Expected: clean build; a fake `WorkspaceEventBus.publish` call with a
`projectId` matching an active subscriber delivers the frame to that
subscriber's channel; a non-matching `projectId` delivers nothing (the
per-project filtering regression guard).
