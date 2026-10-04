package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// fakeJS fails Stream lookups with the given errors in order, then returns a nil stream.
type fakeJS struct {
	jetstream.JetStream
	errs  []error
	calls int
}

func (f *fakeJS) Stream(context.Context, string) (jetstream.Stream, error) {
	f.calls++
	if len(f.errs) == 0 {
		return nil, nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return nil, err
}

func fastWaits(t *testing.T) {
	i, m := streamWaitInitial, streamWaitMax
	streamWaitInitial, streamWaitMax = time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() { streamWaitInitial, streamWaitMax = i, m })
}

func TestAwaitStreamRetriesUntilStreamExists(t *testing.T) {
	fastWaits(t)
	f := &fakeJS{errs: []error{jetstream.ErrStreamNotFound, jetstream.ErrStreamNotFound}}
	if _, err := (&Consumer{js: f}).awaitStream(context.Background(), "TASK"); err != nil {
		t.Fatal(err)
	}
	if f.calls != 3 {
		t.Fatalf("calls = %d, want 3", f.calls)
	}
}

func TestAwaitStreamDoesNotRetryOtherErrors(t *testing.T) {
	fastWaits(t)
	boom := errors.New("boom")
	f := &fakeJS{errs: []error{boom}}
	if _, err := (&Consumer{js: f}).awaitStream(context.Background(), "TASK"); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("calls = %d, want 1", f.calls)
	}
}

func TestAwaitStreamStopsWhenContextEnds(t *testing.T) {
	fastWaits(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	f := &fakeJS{errs: make([]error, 1000)}
	for i := range f.errs {
		f.errs[i] = jetstream.ErrStreamNotFound
	}
	if _, err := (&Consumer{js: f}).awaitStream(ctx, "TASK"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}
