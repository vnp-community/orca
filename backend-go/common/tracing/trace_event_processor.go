package tracing

import (
	"context"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/stablyai/orca-go/common/eventbus"
)

// tracePublisher is the minimal *eventbus.Publisher surface
// TraceEventSpanProcessor needs — narrowed to an interface (rather than
// depending on the concrete type directly) purely so tests can inject a
// fake without a live NATS connection; eventbus.Publisher has no exported
// way to construct one against a fake jetstream.JetStream, and this repo
// has no embedded-NATS test server precedent. *eventbus.Publisher
// satisfies this interface unmodified — production callers are unaffected.
type tracePublisher interface {
	Publish(ctx context.Context, subject string, event eventbus.Event) error
}

// TraceEventSpanProcessor bridges OTel spans onto NATS as F40 TraceEvent
// JSON (see trace_event.go), powering TracePanel via api-gateway's SSE
// endpoint (CR-FFT-003). Publishes directly via eventbus.Publisher.Publish
// — deliberately bypassing the transactional-outbox pattern that
// Publisher.Publish's own doc comment otherwise expects, because trace
// data is diagnostic/best-effort (CR-FFT-002's doc comment), not
// domain-of-record: a dropped event on a mid-flight service restart is
// acceptable, unlike an outbox-backed domain event whose loss would be a
// real data-integrity bug.
type TraceEventSpanProcessor struct {
	serviceName string
	pub         tracePublisher // nil-safe: OnStart/OnEnd no-op when nil
}

// NewTraceEventSpanProcessor wires pub as this processor's publish target.
// A nil pub is valid and makes OnStart/OnEnd no-ops (WithTraceEventPublisher
// is never called at 11/17 services, per CR-FFT-002's 6-service rollout).
func NewTraceEventSpanProcessor(serviceName string, pub *eventbus.Publisher) *TraceEventSpanProcessor {
	p := &TraceEventSpanProcessor{serviceName: serviceName}
	if pub != nil {
		p.pub = pub
	}
	return p
}

func (p *TraceEventSpanProcessor) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	p.publish(s.SpanContext().TraceID().String(), s.Name(), TraceLevelStart, s.Attributes(), s.StartTime())
}

func (p *TraceEventSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	level := TraceLevelOK
	if s.Status().Code == codes.Error {
		level = TraceLevelFail
	}
	p.publish(s.SpanContext().TraceID().String(), s.Name(), level, s.Attributes(), s.EndTime())
}

// Shutdown/ForceFlush are no-ops — eventbus.Publisher has no client-side
// buffering to flush; each publish() call already sent (or is sending)
// independently.
func (p *TraceEventSpanProcessor) Shutdown(context.Context) error   { return nil }
func (p *TraceEventSpanProcessor) ForceFlush(context.Context) error { return nil }

func (p *TraceEventSpanProcessor) publish(traceID, spanName, level string, attrs []attribute.KeyValue, ts time.Time) {
	if p.pub == nil {
		return
	}
	event := TraceEvent{
		ID:     traceID,
		Flow:   p.serviceName + ":" + spanName,
		Level:  level,
		Fields: SpanAttributesToFields(attrs),
		TS:     ts.UnixMilli(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return // malformed attrs shouldn't crash the tracer callback path
	}
	subject := "orca." + p.serviceName + ".trace.span"
	// Best-effort, fire-and-forget — a trace span callback must never
	// block request handling on NATS availability. Publish order across
	// concurrent spans is not guaranteed; TracePanel orders by TS, not
	// network arrival, so this is acceptable for diagnostic data.
	go func() {
		_ = p.pub.Publish(context.Background(), subject, eventbus.Event{
			ID:         traceID + ":" + level,
			OccurredAt: ts,
			Version:    1,
			Payload:    payload,
		})
	}()
}
