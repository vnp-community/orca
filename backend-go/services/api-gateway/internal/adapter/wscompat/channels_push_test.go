package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
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
	RegisterPushChannels(registry, opener, NewClientEventBus(), &fakeInfraFleetClient{})

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
	RegisterPushChannels(registry, opener, NewClientEventBus(), &fakeInfraFleetClient{})

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

// TestRegisterNotificationStreamChannel_RecvErrorClosesOutputChannel drives
// registerNotificationStreamChannel's StreamHandler directly (bypassing the
// WS transport, same shape as TestRegisterClientEventsChannel_* below) to
// verify a non-EOF stream.Recv() error closes the returned channel cleanly
// instead of leaving it open or panicking.
func TestRegisterNotificationStreamChannel_RecvErrorClosesOutputChannel(t *testing.T) {
	stream := &fakeNotificationStream{err: errors.New("stream closed")}
	opener := NotificationStreamOpener(func(ctx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error) {
		return stream, nil
	})

	registry := NewRegistry()
	registerNotificationStreamChannel(registry, opener)

	sh, ok := registry.StreamHandlerFor("notifications.subscribe")
	if !ok {
		t.Fatal("expected notifications.subscribe to be registered")
	}

	events, err := sh(context.Background(), Identity{UserID: "user-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to be closed after a Recv error, got a delivered item")
		}
	case <-time.After(time.Second):
		t.Fatal("expected the events channel to close after a Recv error, but reading blocked")
	}
}

// TestRegisterNotificationStreamChannel_ContextCancelClosesOutputChannel
// verifies the forwarding goroutine's `case <-ctx.Done(): return` branch:
// cancelling ctx while the goroutine is (or may be) blocked trying to send
// on out must still close the channel, with no leaked goroutine or panic.
func TestRegisterNotificationStreamChannel_ContextCancelClosesOutputChannel(t *testing.T) {
	stream := &fakeNotificationStream{items: []*notificationv1.NotificationServiceStreamNotificationsResponse{
		{Id: "evt-1", Type: "task.completed", PayloadJson: `{}`},
	}}
	opener := NotificationStreamOpener(func(ctx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error) {
		return stream, nil
	})

	registry := NewRegistry()
	registerNotificationStreamChannel(registry, opener)

	sh, ok := registry.StreamHandlerFor("notifications.subscribe")
	if !ok {
		t.Fatal("expected notifications.subscribe to be registered")
	}

	ctx, cancel := context.WithCancel(context.Background())
	events, err := sh(ctx, Identity{UserID: "user-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Never drain events — the forwarding goroutine may be parked trying to
	// send the one queued item. Cancelling must still unblock and close it.
	cancel()

	select {
	case _, ok := <-events:
		if ok {
			// A single buffered item may legitimately win the send/ctx.Done
			// race; drain once more and require closure on the next read.
			select {
			case _, ok := <-events:
				if ok {
					t.Fatal("expected the events channel to be closed after ctx cancel")
				}
			case <-time.After(time.Second):
				t.Fatal("expected the events channel to close after ctx cancel, but reading blocked")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("expected the events channel to close after ctx cancel, but reading blocked")
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

// TestClientEventBus_PublishDropsWhenSubscriberBufferFull exercises
// Publish's `select { case ch <- ev: default: }` branch: once a
// subscriber's buffered channel (capacity 16) is full, further Publish
// calls must drop the event and return immediately rather than block.
func TestClientEventBus_PublishDropsWhenSubscriberBufferFull(t *testing.T) {
	bus := NewClientEventBus()
	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	const capacity = 16
	for i := 0; i < capacity; i++ {
		bus.Publish(PushEvent{Channel: "runtime.clientEvent", Args: []any{i}})
	}
	if n := len(ch); n != capacity {
		t.Fatalf("buffered subscriber channel has %d items, want %d (full)", n, capacity)
	}

	done := make(chan struct{})
	go func() {
		bus.Publish(PushEvent{Channel: "runtime.clientEvent", Args: []any{"overflow"}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber instead of dropping it (select default: branch)")
	}

	if n := len(ch); n != capacity {
		t.Fatalf("buffered channel has %d items after the overflow publish, want unchanged %d (event dropped)", n, capacity)
	}

	// Drain and confirm the surviving items are the original ones (0..15),
	// not the overflow event that should have been dropped.
	first := <-ch
	if len(first.Args) != 1 || first.Args[0] != 0 {
		t.Fatalf("first drained event = %+v, want Args[0]=0 (the overflow event, not an existing one, should be dropped)", first)
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

// fakePortForwardEventsStream is a minimal
// infrafleetv1.InfraFleetService_StreamPortForwardEventsClient test double —
// same "embed the nil grpc.ClientStream, override only Recv" shape as
// fakeNotificationStream above.
type fakePortForwardEventsStream struct {
	grpc.ClientStream

	mu    sync.Mutex
	items []*infrafleetv1.PortForwardEvent
	err   error
}

func (f *fakePortForwardEventsStream) Recv() (*infrafleetv1.PortForwardEvent, error) {
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

// TestWorkspacePortsSubscribe_DeliversOpenedAndClosedPushFrames is
// TASK-SSH-04-08's end-to-end assertion: a fake InfraFleetServiceClient
// stream emitting one "opened" and one "closed" PortForwardEvent is
// delivered as workspacePorts.opened/workspacePorts.closed push frames over
// the WS transport (mirrors TestNotificationsSubscribe_DeliversPushFrame's
// shape for notifications.subscribe).
func TestWorkspacePortsSubscribe_DeliversOpenedAndClosedPushFrames(t *testing.T) {
	stream := &fakePortForwardEventsStream{items: []*infrafleetv1.PortForwardEvent{
		{Kind: "opened", Forward: &infrafleetv1.PortForward{Id: "pf-1", ConnectionId: "conn-1", LocalPort: 3001, RemotePort: 3000, ProcessName: "node", Status: "active"}},
		{Kind: "closed", Forward: &infrafleetv1.PortForward{Id: "pf-1", ConnectionId: "conn-1", LocalPort: 3001, RemotePort: 3000, ProcessName: "node", Status: "closed"}},
	}}
	fake := &fakeInfraFleetClient{
		streamPortForwardEventsFunc: func(ctx context.Context, in *infrafleetv1.StreamPortForwardEventsRequest) (infrafleetv1.InfraFleetService_StreamPortForwardEventsClient, error) {
			if in.GetConnectionId() != "conn-1" {
				t.Errorf("StreamPortForwardEvents called with connectionId = %q, want %q", in.GetConnectionId(), "conn-1")
			}
			return stream, nil
		},
	}

	registry := NewRegistry()
	RegisterPushChannels(registry, NotificationStreamOpener(func(ctx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error) {
		return nil, errors.New("not used by this test")
	}), NewClientEventBus(), fake)

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeJSONFrame(ctx, client, InboundMessage{
		ID: "sub-1", Type: "invoke", Channel: "workspacePorts.subscribe",
		Args: []json.RawMessage{json.RawMessage(`{"connectionId":"conn-1"}`)},
	}); err != nil {
		t.Fatalf("writing subscribe invoke: %v", err)
	}

	ack := readWireMessage(t, ctx, client)
	if ack.Type != "result" || ack.ID != "sub-1" {
		t.Fatalf("first frame = %+v, want the subscribe ack", ack)
	}

	opened := readWireMessage(t, ctx, client)
	if opened.Type != "push" || opened.Channel != "workspacePorts.opened" {
		t.Fatalf("second frame = %+v, want type=push channel=workspacePorts.opened", opened)
	}

	closed := readWireMessage(t, ctx, client)
	if closed.Type != "push" || closed.Channel != "workspacePorts.closed" {
		t.Fatalf("third frame = %+v, want type=push channel=workspacePorts.closed", closed)
	}
}

// TestRegisterWorkspacePortsStreamChannel_RecvErrorClosesOutputChannel
// drives registerWorkspacePortsStreamChannel's StreamHandler directly
// (bypassing the WS transport), mirroring
// TestRegisterNotificationStreamChannel_RecvErrorClosesOutputChannel.
func TestRegisterWorkspacePortsStreamChannel_RecvErrorClosesOutputChannel(t *testing.T) {
	stream := &fakePortForwardEventsStream{err: errors.New("stream closed")}
	fake := &fakeInfraFleetClient{
		streamPortForwardEventsFunc: func(ctx context.Context, in *infrafleetv1.StreamPortForwardEventsRequest) (infrafleetv1.InfraFleetService_StreamPortForwardEventsClient, error) {
			return stream, nil
		},
	}

	registry := NewRegistry()
	registerWorkspacePortsStreamChannel(registry, fake)

	sh, ok := registry.StreamHandlerFor("workspacePorts.subscribe")
	if !ok {
		t.Fatal("expected workspacePorts.subscribe to be registered")
	}

	events, err := sh(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, []json.RawMessage{json.RawMessage(`{"connectionId":"conn-1"}`)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected the events channel to be closed after a Recv error, got a delivered item")
		}
	case <-time.After(time.Second):
		t.Fatal("expected the events channel to close after a Recv error, but reading blocked")
	}
}
