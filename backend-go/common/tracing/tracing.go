// Package tracing wires OpenTelemetry for a service. With no OTLPEndpoint
// configured, Init falls back to an always-sample, exporter-less provider —
// this keeps local dev and unit tests working without requiring a
// collector to be running, while still exercising the real instrumentation
// code paths services call into. Full span-attribute/RED-metrics work
// described in specs/backend-go/architecture/09-observability-reliability.md
// is still a follow-up; only the exporter itself is wired here.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Shutdown flushes and stops the tracer provider; call it deferred from
// cmd/server/main.go.
type Shutdown func(context.Context) error

// Init installs a TracerProvider tagged with the service name. When
// otlpEndpoint is set, spans are batched and shipped to it over gRPC
// (insecure/plaintext — every caller in this scaffold reaches its collector
// over a private mesh network, not the public internet, matching the
// intra-cluster trust the shared health/eventbus adapters already assume).
// otlptracegrpc.New only opens the client; it doesn't dial eagerly, so a
// service still starts cleanly even if the collector is briefly unreachable.
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
		exporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("tracing: creating otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}, nil
}
