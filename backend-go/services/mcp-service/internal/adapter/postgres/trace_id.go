package postgres

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// traceIDFromContext returns the hex trace id of the gRPC request being served
// (the server-side otelgrpc handler continues the gateway's MCP request
// trace), or "" when the request is not traced.
func traceIDFromContext(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}
