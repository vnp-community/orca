package postgres

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceIDFromContext(t *testing.T) {
	if got := traceIDFromContext(context.Background()); got != "" {
		t.Fatalf("untraced ctx = %q", got)
	}
	tid, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled}))
	if got := traceIDFromContext(ctx); got != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("got %q", got)
	}
}
