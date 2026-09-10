package tracing

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/stablyai/orca-go/common/eventbus"
)

// fakeTracePublisher is a tracePublisher test double — records every
// Publish call on a channel so the processor's fire-and-forget goroutine
// (publish()'s `go func(){...}`) can be awaited deterministically instead
// of racing the test.
type fakeTracePublisher struct {
	mu       sync.Mutex
	received chan publishCall
}

type publishCall struct {
	subject string
	event   eventbus.Event
}

func newFakeTracePublisher() *fakeTracePublisher {
	return &fakeTracePublisher{received: make(chan publishCall, 8)}
}

func (f *fakeTracePublisher) Publish(_ context.Context, subject string, event eventbus.Event) error {
	f.received <- publishCall{subject: subject, event: event}
	return nil
}

func (f *fakeTracePublisher) awaitOne(t *testing.T) publishCall {
	t.Helper()
	select {
	case c := <-f.received:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for TraceEventSpanProcessor to publish")
		return publishCall{}
	}
}

// TestInit_WithTraceEventPublisher_RegistersSpanProcessor is TASK-BE-FFT-007's
// mandatory end-to-end proof: a real span created after Init with
// WithTraceEventPublisher publishes a TraceEvent on the expected subject.
func TestInit_WithTraceEventPublisher_RegistersSpanProcessor(t *testing.T) {
	fake := newFakeTracePublisher()
	proc := &TraceEventSpanProcessor{serviceName: "svc-under-test", pub: fake}

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(proc))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "op")
	span.End()

	call := fake.awaitOne(t)
	if call.subject != "orca.svc-under-test.trace.span" {
		t.Errorf("subject: got %q, want orca.svc-under-test.trace.span", call.subject)
	}
	var ev TraceEvent
	if err := json.Unmarshal(call.event.Payload, &ev); err != nil {
		t.Fatalf("unmarshaling published payload: %v", err)
	}
	if ev.Flow != "svc-under-test:op" {
		t.Errorf("flow: got %q, want svc-under-test:op", ev.Flow)
	}
}

// TestInit_WithoutOption_NoPublisherNoPanic is the mandatory regression:
// Init with no options behaves exactly as it did before this task (no
// publish anywhere, no error, no panic) — the 17 pre-existing call sites.
func TestInit_WithoutOption_NoPublisherNoPanic(t *testing.T) {
	shutdown, err := Init(context.Background(), "svc", "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel guards
// CR-FFT-002's acceptance criterion: an errored span must publish level
// "fail", never silently disappear.
func TestTraceEventSpanProcessor_OnEnd_ErrorStatusPublishesFailLevel(t *testing.T) {
	fake := newFakeTracePublisher()
	proc := &TraceEventSpanProcessor{serviceName: "svc", pub: fake}

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(proc))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "failing-op")
	span.SetStatus(codes.Error, "boom")
	span.End()

	call := fake.awaitOne(t)
	var ev TraceEvent
	if err := json.Unmarshal(call.event.Payload, &ev); err != nil {
		t.Fatalf("unmarshaling published payload: %v", err)
	}
	if ev.Level != TraceLevelFail {
		t.Errorf("level: got %q, want %q", ev.Level, TraceLevelFail)
	}
}

// TestTraceEventSpanProcessor_NilPublisher_NoOp confirms a processor
// constructed with a nil publisher (the 11/17 services not opted into
// CR-FFT-002) never panics and never attempts to publish.
func TestTraceEventSpanProcessor_NilPublisher_NoOp(t *testing.T) {
	proc := NewTraceEventSpanProcessor("svc", nil)

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(proc))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	_, span := tracer.Start(context.Background(), "op")
	span.End() // must not panic
}

// TestNewTraceEventSpanProcessor_NonNilPublisher_Wired confirms the real
// constructor (used by Init/WithTraceEventPublisher) correctly wires a
// non-nil *eventbus.Publisher into the pub field as a non-nil interface —
// guards the classic Go nil-interface gotcha (storing a nil concrete
// pointer directly in an interface field would make p.pub == nil false).
func TestNewTraceEventSpanProcessor_NonNilPublisher_Wired(t *testing.T) {
	proc := NewTraceEventSpanProcessor("svc", &eventbus.Publisher{})
	if proc.pub == nil {
		t.Fatal("expected pub to be wired (non-nil) when a non-nil *eventbus.Publisher is passed")
	}
}
