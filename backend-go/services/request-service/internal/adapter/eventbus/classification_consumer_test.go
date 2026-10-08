package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	"github.com/stablyai/orca-go/common/tenant"
)

type fakeTrigger struct {
	calls []struct{ tenantID, requestID, eventID, trigger string }
	err   error
}

func (f *fakeTrigger) Run(ctx context.Context, requestID, eventID, trigger string) error {
	tid, _ := tenant.TenantID(ctx)
	f.calls = append(f.calls, struct{ tenantID, requestID, eventID, trigger string }{tid, requestID, eventID, trigger})
	return f.err
}

func event(payload string) commoneventbus.Event {
	return commoneventbus.Event{ID: "ev-1", TenantID: "tenant-1", Payload: []byte(payload)}
}

func TestConsumer_IgnoresOtherTargets(t *testing.T) {
	tr := &fakeTrigger{}
	c := NewClassificationConsumer(nil, tr)
	if err := c.Handle(context.Background(), event(`{"request_id":"r1","to":"analyzing","trigger":"type_confirmed"}`)); err != nil || len(tr.calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(tr.calls))
	}
}

func TestConsumer_TriggersOnClassifying(t *testing.T) {
	tr := &fakeTrigger{}
	c := NewClassificationConsumer(nil, tr)
	if err := c.Handle(context.Background(), event(`{"request_id":"r1","to":"classifying","trigger":"start_classification"}`)); err != nil {
		t.Fatal(err)
	}
	if len(tr.calls) != 1 || tr.calls[0].requestID != "r1" || tr.calls[0].eventID != "ev-1" || tr.calls[0].tenantID != "tenant-1" || tr.calls[0].trigger != "start_classification" {
		t.Fatalf("%+v", tr.calls)
	}
}

func TestConsumer_MalformedAndTenantlessAreAcked(t *testing.T) {
	tr := &fakeTrigger{}
	c := NewClassificationConsumer(nil, tr)
	for _, ev := range []commoneventbus.Event{event(`not json`), event(`{"to":"classifying"}`), {ID: "e", Payload: []byte(`{"request_id":"r","to":"classifying"}`)}} {
		if err := c.Handle(context.Background(), ev); err != nil {
			t.Errorf("must ack, got %v", err)
		}
	}
	if len(tr.calls) != 0 {
		t.Fatal("trigger must not run")
	}
}

func TestConsumer_TransientErrorNotAcked(t *testing.T) {
	tr := &fakeTrigger{err: errors.New("db down")}
	c := NewClassificationConsumer(nil, tr)
	if err := c.Handle(context.Background(), event(`{"request_id":"r1","to":"classifying"}`)); err == nil {
		t.Fatal("error must propagate so JetStream redelivers")
	}
}

type fakePruner struct{ calls chan time.Time }

func (f *fakePruner) MarkProcessed(context.Context, string, string) (bool, error) { return false, nil }
func (f *fakePruner) Prune(_ context.Context, before time.Time) (int64, error) {
	f.calls <- before
	return 3, nil
}

func TestRunPruneLoop_PrunesOlderThanRetention(t *testing.T) {
	p := &fakePruner{calls: make(chan time.Time, 4)}
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	done := make(chan struct{})
	go func() { RunPruneLoop(ctx, p, time.Hour, 7*24*time.Hour, func() time.Time { return now }); close(done) }()
	select {
	case got := <-p.calls:
		if !got.Equal(now.Add(-7 * 24 * time.Hour)) {
			t.Fatalf("cutoff = %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no prune")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not stop")
	}
}
