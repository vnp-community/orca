package tracing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/stablyai/orca-go/common/eventbus"
)

// traceSpanEvent is one finished span's wire shape, published as
// eventbus.Event.Payload — kept deliberately small (diagnostic data, not a
// full OTLP span encoding); see this package's WithTraceEventPublisher doc
// comment for how it's produced and consumed.
type traceSpanEvent struct {
	TraceID      string            `json:"traceId"`
	SpanID       string            `json:"spanId"`
	ParentSpanID string            `json:"parentSpanId,omitempty"`
	Name         string            `json:"name"`
	ServiceName  string            `json:"serviceName"`
	StartUnixMs  int64             `json:"startUnixMs"`
	EndUnixMs    int64             `json:"endUnixMs"`
	DurationMs   int64             `json:"durationMs"`
	StatusCode   string            `json:"statusCode"`
	StatusDesc   string            `json:"statusDescription,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// eventPublishingProcessor is an sdktrace.SpanProcessor that, on every
// finished span, best-effort publishes a traceSpanEvent onto the TRACE
// JetStream stream (see cmd/server/main.go's EnsureStream call) — a
// real-time sink alongside (never instead of) the batched OTLP exporter
// Init wires separately. Never blocks or fails span processing: a publish
// error is swallowed, matching every other trace/audit "best-effort, never
// affects the real operation" sink in this scaffold.
type eventPublishingProcessor struct {
	pub         *eventbus.Publisher
	serviceName string
	subject     string
}

func newEventPublishingProcessor(pub *eventbus.Publisher, serviceName string) *eventPublishingProcessor {
	return &eventPublishingProcessor{pub: pub, serviceName: serviceName, subject: "orca." + serviceName + ".trace.span"}
}

func (p *eventPublishingProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {}

func (p *eventPublishingProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	sc := s.SpanContext()
	parentID := ""
	if s.Parent().HasSpanID() {
		parentID = s.Parent().SpanID().String()
	}
	attrs := make(map[string]string, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	payload, err := json.Marshal(traceSpanEvent{
		TraceID:      sc.TraceID().String(),
		SpanID:       sc.SpanID().String(),
		ParentSpanID: parentID,
		Name:         s.Name(),
		ServiceName:  p.serviceName,
		StartUnixMs:  s.StartTime().UnixMilli(),
		EndUnixMs:    s.EndTime().UnixMilli(),
		DurationMs:   s.EndTime().Sub(s.StartTime()).Milliseconds(),
		StatusCode:   s.Status().Code.String(),
		StatusDesc:   s.Status().Description,
		Attributes:   attrs,
	})
	if err != nil {
		return
	}
	// Detached from the span's own (possibly already-canceled) context —
	// publishing happens after the span is already finished, so there is
	// no caller request context left to reuse; a short timeout bounds a
	// slow/unreachable NATS the same way every other best-effort publisher
	// in this scaffold does.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.pub.Publish(ctx, p.subject, eventbus.Event{
		ID:         uuid.NewString(),
		OccurredAt: s.EndTime(),
		Version:    1,
		Payload:    payload,
	})
}

func (p *eventPublishingProcessor) Shutdown(ctx context.Context) error   { return nil }
func (p *eventPublishingProcessor) ForceFlush(ctx context.Context) error { return nil }

var _ sdktrace.SpanProcessor = (*eventPublishingProcessor)(nil)
