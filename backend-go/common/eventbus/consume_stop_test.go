package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type fakeConsumeContext struct {
	stopped atomic.Bool
	closed  chan struct{}
}

func (f *fakeConsumeContext) Stop()                   { f.stopped.Store(true); close(f.closed) }
func (f *fakeConsumeContext) Drain()                  {}
func (f *fakeConsumeContext) Closed() <-chan struct{} { return f.closed }

type fakeConsumer struct {
	jetstream.Consumer
	cc *fakeConsumeContext
	h  jetstream.MessageHandler
}

func (f *fakeConsumer) Consume(h jetstream.MessageHandler, _ ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
	f.h = h
	return f.cc, nil
}

type fakeMsg struct {
	jetstream.Msg
	naked atomic.Bool
}

func (m *fakeMsg) Data() []byte { return []byte(`{}`) }
func (m *fakeMsg) Nak() error   { m.naked.Store(true); return nil }
func (m *fakeMsg) Ack() error   { return nil }

// A cancelled subscription must stop its pull loop, else the handler keeps
// running after the caller tore down whatever the handler writes to.
func TestConsumeUntilDoneStopsConsumerWhenContextEnds(t *testing.T) {
	cons := &fakeConsumer{cc: &fakeConsumeContext{closed: make(chan struct{})}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var calls atomic.Int32
	go func() {
		done <- consumeUntilDone(ctx, cons, func(context.Context, Event) error { calls.Add(1); return nil })
	}()
	for cons.h == nil {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !cons.cc.stopped.Load() {
		t.Fatal("consume loop was not stopped")
	}
	// A message already buffered when ctx ended is nak'd, not handled.
	m := &fakeMsg{}
	cons.h(m)
	if calls.Load() != 0 || !m.naked.Load() {
		t.Fatalf("handler ran after cancel: calls=%d naked=%v", calls.Load(), m.naked.Load())
	}
}
