package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeDedup struct {
	mu       sync.Mutex
	failN    int
	calls    int
	ids      []string
	block    chan struct{}
	received int
}

func (f *fakeDedup) PublishDedup(_ context.Context, _ string, ev Event) error {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.ids = append(f.ids, ev.ID)
	if f.calls <= f.failN {
		return errors.New("no responders")
	}
	return nil
}

type outcomes struct {
	mu   sync.Mutex
	list []string
	errs []error
}

func (o *outcomes) hook(_, outcome string, err error) {
	o.mu.Lock()
	o.list = append(o.list, outcome)
	o.errs = append(o.errs, err)
	o.mu.Unlock()
}

func newTestAsync(f DedupPublisher, cfg AsyncPublishConfig, o *outcomes, slept *[]time.Duration) *AsyncPublisher {
	a := NewAsyncPublisher(f, cfg, o.hook)
	var mu sync.Mutex
	a.SetSleep(func(d time.Duration) { mu.Lock(); *slept = append(*slept, d); mu.Unlock() })
	return a
}

func TestAsyncPublisher_RetriesWithBackoffThenSucceedsKeepingTheSameID(t *testing.T) {
	f, o, slept := &fakeDedup{failN: 2}, &outcomes{}, []time.Duration{}
	a := newTestAsync(f, AsyncPublishConfig{Attempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: 150 * time.Millisecond}, o, &slept)
	a.Publish("orca.mcp.x", Event{ID: "id-1"})
	if err := a.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.calls != 3 || len(o.list) != 1 || o.list[0] != AsyncPublished {
		t.Fatalf("calls=%d outcomes=%v", f.calls, o.list)
	}
	for _, id := range f.ids {
		if id != "id-1" {
			t.Fatalf("retry changed the dedupe id: %v", f.ids)
		}
	}
	if len(slept) != 2 || slept[0] != 100*time.Millisecond || slept[1] != 150*time.Millisecond {
		t.Fatalf("backoff = %v (doubling, capped)", slept)
	}
}

func TestAsyncPublisher_FinalFailureIsReported(t *testing.T) {
	f, o, slept := &fakeDedup{failN: 100}, &outcomes{}, []time.Duration{}
	a := newTestAsync(f, AsyncPublishConfig{Attempts: 3}, o, &slept)
	a.Publish("orca.mcp.x", Event{ID: "id-2"})
	_ = a.Wait(context.Background())
	if f.calls != 3 || len(o.list) != 1 || o.list[0] != AsyncFailed || o.errs[0] == nil {
		t.Fatalf("calls=%d outcomes=%v errs=%v", f.calls, o.list, o.errs)
	}
	if len(slept) != 2 {
		t.Fatalf("must not sleep after the last attempt: %v", slept)
	}
}

func TestAsyncPublisher_DoesNotBlockAndDropsWhenSaturated(t *testing.T) {
	f := &fakeDedup{block: make(chan struct{})}
	o, slept := &outcomes{}, []time.Duration{}
	a := newTestAsync(f, AsyncPublishConfig{MaxInFlight: 1}, o, &slept)
	a.Publish("s", Event{ID: "a"}) // occupies the only slot; returns at once
	a.Publish("s", Event{ID: "b"})
	o.mu.Lock()
	if len(o.list) != 1 || o.list[0] != AsyncDropped {
		o.mu.Unlock()
		t.Fatalf("outcomes = %v", o.list)
	}
	o.mu.Unlock()
	close(f.block)
	_ = a.Wait(context.Background())
	if len(o.list) != 2 || o.list[1] != AsyncPublished {
		t.Fatalf("outcomes = %v", o.list)
	}
}
