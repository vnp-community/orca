package wscompat

import (
	"context"
	"testing"
	"time"
)

// TestWorkspaceEventBus_PublishDeliversToMatchingProjectSubscriber is the
// per-project filtering regression guard TASK-PW-04-07's own Verify section
// calls for: a publish with a projectId matching an active subscriber
// delivers the frame to that subscriber's channel.
func TestWorkspaceEventBus_PublishDeliversToMatchingProjectSubscriber(t *testing.T) {
	bus := NewWorkspaceEventBus()
	ch, unsubscribe := bus.Subscribe("proj-1")
	defer unsubscribe()

	bus.publish("proj-1", PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "task.statuschanged"}}})

	select {
	case ev := <-ch:
		if ev.Channel != "workspace.event" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the matching-project event to be delivered")
	}
}

// TestWorkspaceEventBus_PublishSkipsNonMatchingProjectSubscriber is the
// other half of the same regression guard: a non-matching projectId
// delivers nothing.
func TestWorkspaceEventBus_PublishSkipsNonMatchingProjectSubscriber(t *testing.T) {
	bus := NewWorkspaceEventBus()
	ch, unsubscribe := bus.Subscribe("proj-1")
	defer unsubscribe()

	bus.publish("proj-2", PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "task.statuschanged"}}})

	select {
	case ev := <-ch:
		t.Fatalf("expected no event for a non-matching projectId, got %+v", ev)
	case <-time.After(100 * time.Millisecond):
		// expected: nothing delivered
	}
}

// TestWorkspaceEventBus_TwoSubscribersDifferentProjects_EachGetsOwnEvents
// proves the demux is genuinely per-project, not just "not obviously
// wrong" — mirrors devserveragent's TestClientStreamPty_TwoConcurrentSubscriptions
// precedent for the same class of regression.
func TestWorkspaceEventBus_TwoSubscribersDifferentProjects_EachGetsOwnEvents(t *testing.T) {
	bus := NewWorkspaceEventBus()
	chA, unsubA := bus.Subscribe("proj-a")
	defer unsubA()
	chB, unsubB := bus.Subscribe("proj-b")
	defer unsubB()

	bus.publish("proj-a", PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "for-a"}}})
	bus.publish("proj-b", PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "for-b"}}})

	select {
	case ev := <-chA:
		payload, _ := ev.Args[0].(map[string]any)
		if payload["type"] != "for-a" {
			t.Errorf("proj-a subscriber got wrong event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for proj-a's event")
	}
	select {
	case ev := <-chB:
		payload, _ := ev.Args[0].(map[string]any)
		if payload["type"] != "for-b" {
			t.Errorf("proj-b subscriber got wrong event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for proj-b's event")
	}
}

// TestWorkspaceEventBus_UnsubscribeClosesChannel guards the same
// exactly-once-close discipline ClientEventBus's own Subscribe/unsubscribe
// pair relies on.
func TestWorkspaceEventBus_UnsubscribeClosesChannel(t *testing.T) {
	bus := NewWorkspaceEventBus()
	ch, unsubscribe := bus.Subscribe("proj-1")
	unsubscribe()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected the channel to be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the channel to close after unsubscribe")
	}
}

// TestRegisterWorkspaceSubscribeChannel_DeliversMatchingProjectEvent drives
// workspace.subscribe's StreamHandler directly (bypassing the WS
// transport, same shape as channels_push_test.go's
// TestRegisterNotificationStreamChannel_* tests) — decodes the projectId
// arg, subscribes, and confirms a bus.publish for that project reaches the
// returned channel.
func TestRegisterWorkspaceSubscribeChannel_DeliversMatchingProjectEvent(t *testing.T) {
	bus := NewWorkspaceEventBus()
	registry := NewRegistry()
	RegisterWorkspaceSubscribeChannel(registry, bus)

	sh, ok := registry.StreamHandlerFor("workspace.subscribe")
	if !ok {
		t.Fatal("expected workspace.subscribe to be registered")
	}

	events, err := sh(context.Background(), Identity{UserID: "user-1"}, argsJSON(t, map[string]any{"projectId": "proj-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bus.publish("proj-1", PushEvent{Channel: "workspace.event", Args: []any{map[string]any{"type": "task.statuschanged"}}})

	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("expected a delivered event, got a closed channel")
		}
		if ev.Channel != "workspace.event" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the subscribed event")
	}
}

// TestRegisterWorkspaceSubscribeChannel_ContextCancelUnsubscribes verifies
// the StreamHandler's `go func() { <-ctx.Done(); unsubscribe() }()` wiring:
// cancelling ctx must close the returned channel (via WorkspaceEventBus's
// own unsubscribe), not leak the subscription.
func TestRegisterWorkspaceSubscribeChannel_ContextCancelUnsubscribes(t *testing.T) {
	bus := NewWorkspaceEventBus()
	registry := NewRegistry()
	RegisterWorkspaceSubscribeChannel(registry, bus)

	sh, ok := registry.StreamHandlerFor("workspace.subscribe")
	if !ok {
		t.Fatal("expected workspace.subscribe to be registered")
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, err := sh(ctx, Identity{UserID: "user-1"}, argsJSON(t, map[string]any{"projectId": "proj-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cancel()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to close after ctx cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the events channel to close after ctx cancellation")
	}
}
