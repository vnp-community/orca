package wscompat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// fakeEphemeralSubscriber is a minimal ephemeralSubscriber test double —
// SubscribeEphemeral replays the events queued for its subject, then blocks
// on ctx.Done() (matching the real *commoneventbus.Consumer.
// SubscribeEphemeral's own consumeUntilDone loop, so callers that rely on
// "the subscription stays open until ctx is cancelled" behave identically
// in tests).
type fakeEphemeralSubscriber struct {
	events map[string][]commoneventbus.Event // keyed by subject
}

func (f *fakeEphemeralSubscriber) SubscribeEphemeral(ctx context.Context, streamName, subject string, fn commoneventbus.Handler) error {
	for _, ev := range f.events[subject] {
		if err := fn(ctx, ev); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return nil
}

func mustMarshalOrigin(t *testing.T, originTaskID string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(struct {
		OriginTaskID string `json:"origin_task_id"`
	}{OriginTaskID: originTaskID})
	if err != nil {
		t.Fatalf("marshal origin payload: %v", err)
	}
	return b
}

// TestRegisterTaskActivityStreamChannel_OneEventPerSubjectReachesMatchingSubscriber
// proves the CR's own acceptance criterion: one synthetic event per subject
// (6 total) reaches exactly one PushEvent on a connection subscribed to the
// matching taskId.
func TestRegisterTaskActivityStreamChannel_OneEventPerSubjectReachesMatchingSubscriber(t *testing.T) {
	const taskID = "task-1"
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{}}
	for _, sub := range taskActivitySubjects {
		bus.events[sub.Subject] = []commoneventbus.Event{
			{ID: "evt-" + sub.Subject, Payload: mustMarshalOrigin(t, taskID), OccurredAt: time.Now()},
		}
	}

	registry := NewRegistry()
	RegisterTaskActivityStreamChannel(registry, bus)

	sh, ok := registry.StreamHandlerFor("task.activity.subscribe")
	if !ok {
		t.Fatal("expected task.activity.subscribe to be registered")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args, err := json.Marshal(struct {
		TaskID string `json:"taskId"`
	}{TaskID: taskID})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	events, err := sh(ctx, Identity{}, []json.RawMessage{args})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := map[string]bool{}
	for i := 0; i < len(taskActivitySubjects); i++ {
		select {
		case ev := <-events:
			if ev.Channel != "task.activity.event" {
				t.Fatalf("event %d channel = %q, want task.activity.event", i, ev.Channel)
			}
			if len(ev.Args) != 1 {
				t.Fatalf("event %d Args = %v, want exactly one item", i, ev.Args)
			}
			frame, ok := ev.Args[0].(TaskActivityFrame)
			if !ok {
				t.Fatalf("event %d Args[0] = %T, want TaskActivityFrame", i, ev.Args[0])
			}
			if frame.TaskID != taskID {
				t.Errorf("event %d TaskID = %q, want %q", i, frame.TaskID, taskID)
			}
			if frame.EventType == "" || frame.Engine == "" {
				t.Errorf("event %d has empty EventType/Engine: %+v", i, frame)
			}
			seen[frame.EventType+"|"+frame.Engine] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d of %d", i, len(taskActivitySubjects))
		}
	}

	// 7 subjects classify into 6 distinct (eventType, engine) pairs since
	// task.dispatched and task.statuschanged both map to
	// (status_changed, orchestration) — see classifySubject.
	if len(seen) != 6 {
		t.Errorf("expected 6 distinct (eventType, engine) pairs observed, got %d: %v", len(seen), seen)
	}
}

// TestRegisterTaskActivityStreamChannel_MismatchedTaskIDReachesZeroEvents
// proves the CR's own acceptance criterion: an event for a different taskId
// reaches zero connections.
func TestRegisterTaskActivityStreamChannel_MismatchedTaskIDReachesZeroEvents(t *testing.T) {
	bus := &fakeEphemeralSubscriber{events: map[string][]commoneventbus.Event{
		"orca.orchestration.task.statuschanged": {
			{ID: "evt-1", Payload: mustMarshalOrigin(t, "task-OTHER"), OccurredAt: time.Now()},
		},
	}}

	registry := NewRegistry()
	RegisterTaskActivityStreamChannel(registry, bus)

	sh, ok := registry.StreamHandlerFor("task.activity.subscribe")
	if !ok {
		t.Fatal("expected task.activity.subscribe to be registered")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args, err := json.Marshal(struct {
		TaskID string `json:"taskId"`
	}{TaskID: "task-1"})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	events, err := sh(ctx, Identity{}, []json.RawMessage{args})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case ev := <-events:
		t.Fatalf("expected no event to reach a subscriber for a different taskId, got %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// expected — no event delivered.
	}
}

func TestRegisterTaskActivityStreamChannel_RequiresTaskID(t *testing.T) {
	registry := NewRegistry()
	RegisterTaskActivityStreamChannel(registry, &fakeEphemeralSubscriber{})

	sh, ok := registry.StreamHandlerFor("task.activity.subscribe")
	if !ok {
		t.Fatal("expected task.activity.subscribe to be registered")
	}

	args, err := json.Marshal(struct {
		TaskID string `json:"taskId"`
	}{TaskID: ""})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	if _, err := sh(context.Background(), Identity{}, []json.RawMessage{args}); err == nil {
		t.Fatal("expected an error for an empty taskId")
	}
}

// TestClassifySubject_AgentOutputPartial locks in TASK-AG-FLOWTASK-003's
// addition — Engine 1 (direct_agent)'s one and only async event source.
func TestClassifySubject_AgentOutputPartial(t *testing.T) {
	engine, eventType := classifySubject("orca.task.agent_output_partial")
	if engine != "direct_agent" || eventType != "agent_output_partial" {
		t.Errorf("expected (direct_agent, agent_output_partial), got (%q, %q)", engine, eventType)
	}
}

func TestClassifySubject_CoversEveryTaskActivitySubject(t *testing.T) {
	for _, sub := range taskActivitySubjects {
		engine, eventType := classifySubject(sub.Subject)
		if engine == "" || eventType == "" {
			t.Errorf("subject %q classified as engine=%q eventType=%q, want both non-empty", sub.Subject, engine, eventType)
		}
	}
	if engine, eventType := classifySubject("orca.unknown.subject"); engine != "" || eventType != "" {
		t.Errorf("unknown subject classified as engine=%q eventType=%q, want both empty", engine, eventType)
	}
}
