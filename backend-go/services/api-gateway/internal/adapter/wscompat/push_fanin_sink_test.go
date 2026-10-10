package wscompat

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

// lingeringSubscriber mimics the JetStream consume loop that kept delivering
// after SubscribeEphemeral returned: SubscribeEphemeral returns on ctx.Done()
// while its sender goroutines keep calling fn until release() is called.
type lingeringSubscriber struct {
	senders int
	ev      commoneventbus.Event
	stop    chan struct{}
	wg      sync.WaitGroup
	sent    atomic.Int64
}

func newLingeringSubscriber(senders int, ev commoneventbus.Event) *lingeringSubscriber {
	return &lingeringSubscriber{senders: senders, ev: ev, stop: make(chan struct{})}
}

func (l *lingeringSubscriber) SubscribeEphemeral(ctx context.Context, _, _ string, fn commoneventbus.Handler) error {
	for i := 0; i < l.senders; i++ {
		l.wg.Add(1)
		go func() {
			defer l.wg.Done()
			for {
				select {
				case <-l.stop:
					return
				default:
				}
				_ = fn(ctx, l.ev)
				l.sent.Add(1)
			}
		}()
	}
	<-ctx.Done()
	return nil
}

func (l *lingeringSubscriber) release() {
	close(l.stop)
	l.wg.Wait()
}

// churnStream opens a stream, reads a little, cancels while senders are still
// publishing, and requires the channel to close without any send panicking.
func churnStream(t *testing.T, open func(ctx context.Context) (<-chan PushEvent, error), bus *lingeringSubscriber) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out, err := open(ctx)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		select {
		case <-out:
		case <-time.After(time.Second):
			cancel()
			t.Fatal("no event before cancel")
		}
	}
	cancel() // senders are mid-send here (nobody reads out any more)
	closed := make(chan struct{})
	go func() {
		for range out {
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("stream channel was not closed after cancel")
	}
	time.Sleep(2 * time.Millisecond) // stragglers keep calling fn against the closed stream
	bus.release()
}

func TestRequestStream_CancelWhileEventsFlowDoesNotPanic(t *testing.T) {
	ev := requestEvent("t1", streamNow, `{"request_id":"r1","to":"analyzing","reporter_id":"u1"}`)
	for i := 0; i < 15; i++ {
		bus := newLingeringSubscriber(4, ev)
		r := NewRegistry()
		registerRequestStream(r, bus, &fakeRequestClient{}, requestEventRegistry, func() time.Time { return streamNow })
		sh, _ := r.StreamHandlerFor("request.subscribe")
		churnStream(t, func(ctx context.Context) (<-chan PushEvent, error) {
			return sh(ctx, Identity{TenantID: "t1", UserID: "u1"}, []json.RawMessage{json.RawMessage(`{}`)})
		}, bus)
	}
}

func TestTaskActivityStream_CancelWhileEventsFlowDoesNotPanic(t *testing.T) {
	ev := commoneventbus.Event{ID: "e", Payload: mustMarshalOrigin(t, "task-1"), OccurredAt: time.Now()}
	for i := 0; i < 15; i++ {
		bus := newLingeringSubscriber(4, ev)
		r := NewRegistry()
		RegisterTaskActivityStreamChannel(r, bus)
		sh, _ := r.StreamHandlerFor("task.activity.subscribe")
		churnStream(t, func(ctx context.Context) (<-chan PushEvent, error) {
			return sh(ctx, Identity{}, []json.RawMessage{json.RawMessage(`{"taskId":"task-1"}`)})
		}, bus)
	}
}

func TestPushFanInSink_SendAfterCloseIsDropped(t *testing.T) {
	s := newPushFanInSink()
	s.close()
	s.close() // idempotent
	if s.send(context.Background(), PushEvent{Channel: "x"}) {
		t.Fatal("send on a closed sink must be dropped, not delivered")
	}
	if _, open := <-s.out; open {
		t.Fatal("out must be closed")
	}
}

func TestPushFanInSink_CloseReleasesBlockedSenders(t *testing.T) {
	s := newPushFanInSink()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.send(context.Background(), PushEvent{}) }()
	}
	time.Sleep(5 * time.Millisecond) // all blocked: nobody reads out and ctx never ends
	done := make(chan struct{})
	go func() { s.close(); wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close deadlocked behind blocked senders")
	}
}
