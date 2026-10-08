package eventbus

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// startLinkedSpan starts a new span (not a child) that links to the producer's
// trace carried in the payload: the outbox does not propagate trace context, and
// a consumer run is a separate unit of work from the publishing request.
func startLinkedSpan(ctx context.Context, traceparent string) (context.Context, trace.Span) {
	var opts []trace.SpanStartOption
	if traceparent != "" {
		remote := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(
			context.Background(), propagation.MapCarrier{"traceparent": traceparent}))
		if remote.IsValid() {
			opts = append(opts, trace.WithLinks(trace.Link{SpanContext: remote}))
		}
	}
	return otel.Tracer("issue-status-sync").Start(ctx, "issuesync.HandleRequestStatus", opts...)
}
