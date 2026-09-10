package tracing

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestInit_SetsTraceContextPropagator proves Init installs a real W3C
// traceparent-capable propagator (TASK-BE-FFT-002), not the SDK's default
// no-op — the whole point of a cross-service trace depends on this.
func TestInit_SetsTraceContextPropagator(t *testing.T) {
	shutdown, err := Init(context.Background(), "svc-under-test", "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	prop := otel.GetTextMapPropagator()

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	carrier := propagation.MapCarrier{}
	prop.Inject(ctx, carrier)
	if carrier.Get("traceparent") == "" {
		t.Fatal("expected propagator to inject a traceparent header, got none — otel.GetTextMapPropagator() is still the no-op default")
	}

	extracted := prop.Extract(context.Background(), carrier)
	gotSC := trace.SpanContextFromContext(extracted)
	if gotSC.TraceID() != sc.TraceID() || gotSC.SpanID() != sc.SpanID() {
		t.Fatalf("round-trip mismatch: want trace=%s span=%s, got trace=%s span=%s",
			sc.TraceID(), sc.SpanID(), gotSC.TraceID(), gotSC.SpanID())
	}
}

// TestInit_SignatureUnchanged_ExistingCallSitesCompile is a compile-time
// guard: every one of the 17 gRPC-serving services' main.go calls
// Init(ctx, serviceName, otlpEndpoint) with exactly these 3 args. If this
// stops compiling, someone changed Init's signature.
func TestInit_SignatureUnchanged_ExistingCallSitesCompile(t *testing.T) {
	shutdown, err := Init(context.Background(), "svc", "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	_ = shutdown(context.Background())
}

// TestInit_NoOTLPEndpoint_StillWorksSameAsBefore guards the pre-existing
// always-sample, exporter-less fallback behavior — must survive the
// propagator addition unchanged.
func TestInit_NoOTLPEndpoint_StillWorksSameAsBefore(t *testing.T) {
	shutdown, err := Init(context.Background(), "svc", "")
	if err != nil {
		t.Fatalf("Init with empty otlpEndpoint returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected a non-nil Shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// TestInit_NoRaceWhenCalledConcurrently guards CR-FFT-001's acceptance
// criterion that concurrent Init calls (global otel state) don't race —
// run with -race.
func TestInit_NoRaceWhenCalledConcurrently(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			shutdown, err := Init(context.Background(), "svc-concurrent", "")
			if err != nil {
				t.Errorf("Init: %v", err)
				return
			}
			_ = shutdown(context.Background())
		}()
	}
	wg.Wait()
}
