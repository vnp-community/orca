// Package tracing wires OpenTelemetry for a service. Kept intentionally
// minimal in this scaffold (a no-op-safe setup that's a real, working
// starting point) — full OTLP exporter wiring, span attributes per RPC, and
// the RED-metrics-from-tracing pipeline described in
// specs/backend-go/architecture/09-observability-reliability.md are left as
// a follow-up once a concrete OTel collector endpoint exists to point at.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Shutdown flushes and stops the tracer provider; call it deferred from
// cmd/server/main.go.
type Shutdown func(context.Context) error

// Init installs a TracerProvider tagged with the service name. With no
// OTLPEndpoint configured, it uses an always-sample, exporter-less provider
// (spans are created and can be asserted on in tests, just never shipped
// anywhere) — this keeps local dev and unit tests working without requiring
// a collector to be running, while still exercising the real
// instrumentation code paths services call into.
func Init(ctx context.Context, serviceName, otlpEndpoint string) (Shutdown, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, err
	}

	var opts []sdktrace.TracerProviderOption
	opts = append(opts, sdktrace.WithResource(res))
	if otlpEndpoint != "" {
		// Production wiring: add an OTLP span exporter here
		// (otlptracegrpc.New) pointed at otlpEndpoint. Omitted from this
		// scaffold to avoid a hard dependency on a reachable collector at
		// service-startup time for local dev / CI unit tests.
		_ = otlpEndpoint
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}, nil
}
