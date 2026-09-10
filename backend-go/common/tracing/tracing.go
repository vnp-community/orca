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
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/stablyai/orca-go/common/eventbus"
)

// Shutdown flushes and stops the tracer provider; call it deferred from
// cmd/server/main.go.
type Shutdown func(context.Context) error

<<<<<<< HEAD
// Option configures Init beyond its required serviceName/otlpEndpoint args.
type Option func(*initConfig)

type initConfig struct {
	eventPublisher *eventbus.Publisher
	serviceName    string
}

// WithTraceEventPublisher additionally publishes every finished span onto
// pub, at subject "orca.<serviceName>.trace.span" (CR-FFT-002/
// TASK-BE-FFT-008) — a real-time sink for api-gateway's TracePanel SSE
// bridge (see httpgateway.TraceBroadcast), separate from and in addition to
// the batched OTLP export above. pub may be nil (e.g. eventbus
// unreachable at startup) — publishing is then skipped entirely, matching
// every other NATS-consuming service's "diagnostic, not fatal" degrade
// posture in this scaffold.
func WithTraceEventPublisher(pub *eventbus.Publisher) Option {
	return func(c *initConfig) { c.eventPublisher = pub }
=======
// Option configures optional Init behavior beyond the exporter — see
// WithTraceEventPublisher. Omitting every Option keeps the existing 17
// call sites' exporter-only behavior unchanged.
type Option func(*initOptions)

type initOptions struct {
	tracePublisher *eventbus.Publisher
}

// WithTraceEventPublisher wires the CR-FFT-002 trace-event bridge: every
// span this service creates is also published as a TraceEvent (see
// trace_event.go) onto NATS via TraceEventSpanProcessor. Only the 6
// services already NATS-connected pass this (CR-FFT-002's deliberately
// limited rollout, not a general convention) — every other call site
// keeps calling Init with no options.
func WithTraceEventPublisher(pub *eventbus.Publisher) Option {
	return func(o *initOptions) { o.tracePublisher = pub }
>>>>>>> feat/team-rbac-implementation
}

// Init installs a TracerProvider tagged with the service name. When
// otlpEndpoint is set, spans are batched and shipped to it over gRPC
// (insecure/plaintext — every caller in this scaffold reaches its collector
// over a private mesh network, not the public internet, matching the
// intra-cluster trust the shared health/eventbus adapters already assume).
// otlptracegrpc.New only opens the client; it doesn't dial eagerly, so a
// service still starts cleanly even if the collector is briefly unreachable.
func Init(ctx context.Context, serviceName, otlpEndpoint string, opts ...Option) (Shutdown, error) {
<<<<<<< HEAD
	cfg := initConfig{serviceName: serviceName}
	for _, opt := range opts {
		opt(&cfg)
=======
	var o initOptions
	for _, opt := range opts {
		opt(&o)
>>>>>>> feat/team-rbac-implementation
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, err
	}

	var tpOpts []sdktrace.TracerProviderOption
	tpOpts = append(tpOpts, sdktrace.WithResource(res))
	if otlpEndpoint != "" {
		exporter, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(otlpEndpoint),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("tracing: creating otlp exporter: %w", err)
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(exporter))
	}
<<<<<<< HEAD
	if cfg.eventPublisher != nil {
		tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(newEventPublishingProcessor(cfg.eventPublisher, serviceName)))
=======
	if o.tracePublisher != nil {
		tpOpts = append(tpOpts, sdktrace.WithSpanProcessor(
			NewTraceEventSpanProcessor(serviceName, o.tracePublisher)))
>>>>>>> feat/team-rbac-implementation
	}

	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)
	// Without this, the default no-op propagator never writes/reads the W3C
	// traceparent header, so spans never link across a gRPC hop even though
	// each service creates one (CR-FFT-001).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}, nil
}
