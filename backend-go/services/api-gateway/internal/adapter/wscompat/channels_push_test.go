package wscompat

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// fakeNotificationStream is a minimal notificationv1.NotificationService_StreamNotificationsClient
// test double — embeds the (nil) grpc.ClientStream interface so it
// satisfies every method (same "embed the nil interface, override only
// what's called" trick channels_test.go's fakeInfraFleetClient already
// uses in this package), overriding only Recv.
type fakeNotificationStream struct {
	grpc.ClientStream

	mu    sync.Mutex
	items []*notificationv1.NotificationServiceStreamNotificationsResponse
	err   error
}

func (f *fakeNotificationStream) Recv() (*notificationv1.NotificationServiceStreamNotificationsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.items) > 0 {
		item := f.items[0]
		f.items = f.items[1:]
		return item, nil
	}
	if f.err != nil {
		return nil, f.err
	}
	return nil, io.EOF
}

// TestNotificationsSubscribe_DeliversPushFrame is the integration test
// TASK-016 calls out: using a fake StreamOpener that emits one item, assert
// notifications.subscribe delivers a push frame with
// channel:"notifications.event" over the WS transport (handler.go's
// handleSubscribe + push_bridge.go's pipePush + this file's
// registerNotificationStreamChannel, wired end to end).
func TestNotificationsSubscribe_DeliversPushFrame(t *testing.T) {
	stream := &fakeNotificationStream{items: []*notificationv1.NotificationServiceStreamNotificationsResponse{
		{Id: "evt-1", Type: "task.completed", PayloadJson: `{"taskId":"t1"}`},
	}}
	opener := NotificationStreamOpener(func(ctx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error) {
		if userID != "user-1" {
			t.Errorf("opener called with userID = %q, want %q", userID, "user-1")
		}
		return stream, nil
	})

	registry := NewRegistry()
	RegisterPushChannels(registry, opener, NewClientEventBus())

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "sub-1", Type: "invoke", Channel: "notifications.subscribe"}); err != nil {
		t.Fatalf("writing subscribe invoke: %v", err)
	}

	ack := readWireMessage(t, ctx, client)
	if ack.Type != "result" || ack.ID != "sub-1" {
		t.Fatalf("first frame = %+v, want the subscribe ack", ack)
	}

	push := readWireMessage(t, ctx, client)
	if push.Type != "push" || push.Channel != "notifications.event" {
		t.Fatalf("second frame = %+v, want type=push channel=notifications.event", push)
	}
	if len(push.Args) != 1 {
		t.Fatalf("push.Args = %v, want exactly one item", push.Args)
	}
}

func TestNotificationsSubscribe_OpenerErrorSurfacesAsErrorMessage(t *testing.T) {
	opener := NotificationStreamOpener(func(ctx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error) {
		return nil, errors.New("upstream unavailable")
	})

	registry := NewRegistry()
	RegisterPushChannels(registry, opener, NewClientEventBus())

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{ID: "sub-1", Type: "invoke", Channel: "notifications.subscribe"}); err != nil {
		t.Fatalf("writing subscribe invoke: %v", err)
	}

	got := readWireMessage(t, ctx, client)
	if got.Type != "error" || got.ID != "sub-1" {
		t.Fatalf("got %+v, want an ErrorMessage for id=sub-1", got)
	}
}

func TestClientEventBus_PublishFansOutToEverySubscriber(t *testing.T) {
	bus := NewClientEventBus()
	ch1, unsub1 := bus.Subscribe()
	defer unsub1()
	ch2, unsub2 := bus.Subscribe()
	defer unsub2()

	bus.Publish(PushEvent{Channel: "runtime.clientEvent", Args: []any{"hello"}})

	for i, ch := range []<-chan PushEvent{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Channel != "runtime.clientEvent" {
				t.Fatalf("subscriber %d got channel %q, want %q", i, ev.Channel, "runtime.clientEvent")
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive the published event", i)
		}
	}
}

func TestClientEventBus_UnsubscribeStopsDelivery(t *testing.T) {
	bus := NewClientEventBus()
	ch, unsubscribe := bus.Subscribe()
	unsubscribe()

	bus.Publish(PushEvent{Channel: "runtime.clientEvent"})

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected the channel to be closed after unsubscribe, got a delivered event")
		}
		// closed channel — expected.
	case <-time.After(time.Second):
		t.Fatal("expected the unsubscribed channel to be closed, but reading blocked")
	}
}

func TestRegisterClientEventsChannel_SubscribeThenContextCancelUnsubscribes(t *testing.T) {
	bus := NewClientEventBus()
	registry := NewRegistry()
	registerClientEventsChannel(registry, bus)

	sh, ok := registry.StreamHandlerFor("runtime.clientEvents.subscribe")
	if !ok {
		t.Fatal("expected runtime.clientEvents.subscribe to be registered")
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, err := sh(ctx, Identity{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bus.mu.Lock()
	subCountBefore := len(bus.subs)
	bus.mu.Unlock()
	if subCountBefore != 1 {
		t.Fatalf("expected 1 subscriber registered on the bus, got %d", subCountBefore)
	}

	cancel()
	// unsubscribe runs in its own goroutine (see registerClientEventsChannel) — poll briefly.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		bus.mu.Lock()
		n := len(bus.subs)
		bus.mu.Unlock()
		if n == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	bus.mu.Lock()
	subCountAfter := len(bus.subs)
	bus.mu.Unlock()
	if subCountAfter != 0 {
		t.Fatalf("expected the bus to have 0 subscribers after ctx cancel, got %d", subCountAfter)
	}
	if _, ok := <-events; ok {
		t.Fatal("expected the returned events channel to be closed after unsubscribe")
	}
}
