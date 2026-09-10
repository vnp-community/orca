package grpcclient

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stablyai/orca-go/common/grpcmw"
)

// echoServiceDesc mirrors common/grpcmw's own regression test — a
// hand-written grpc.ServiceDesc using wrapperspb.StringValue (a real
// proto.Message with no .proto needed) whose handler reports back the
// TraceID it observes server-side, via common/grpcmw.StatsHandler()'s
// otelgrpc.NewServerHandler() extracting the client's injected traceparent.
var dialTestEchoServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpcclient.test.Echo",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Call",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				req := new(wrapperspb.StringValue)
				if err := dec(req); err != nil {
					return nil, err
				}
				handler := func(ctx context.Context, req any) (any, error) {
					sc := trace.SpanContextFromContext(ctx)
					return wrapperspb.String(sc.TraceID().String()), nil
				}
				if interceptor == nil {
					return handler(ctx, req)
				}
				info := &grpc.UnaryServerInfo{FullMethod: "/grpcclient.test.Echo/Call"}
				return interceptor(ctx, req, info, handler)
			},
		},
	},
}

var _ proto.Message = (*wrapperspb.StringValue)(nil)

// TestDial_TraceIDPropagatesToServer is TASK-BE-FFT-005's mandatory
// regression: a client-side span's TraceID must reach the server as the
// SAME TraceID (child span), proving Dial's grpc.WithStatsHandler(
// otelgrpc.NewClientHandler()) actually injects traceparent end-to-end
// against a real server wired with common/grpcmw.StatsHandler()
// (TASK-BE-FFT-001) — this is the empirical evidence for CR-FFT-001's "one
// request through N services is one trace" acceptance criterion.
func TestDial_TraceIDPropagatesToServer(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := grpc.NewServer(grpcmw.ChainUnary(logger), grpcmw.StatsHandler())
	srv.RegisterService(&dialTestEchoServiceDesc, nil)
	reflection.Register(srv)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := Dial(lis.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	tracer := tp.Tracer("dial_test")
	ctx, span := tracer.Start(context.Background(), "client-span")
	defer span.End()
	wantTraceID := span.SpanContext().TraceID().String()

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out := new(wrapperspb.StringValue)
	if err := conn.Invoke(callCtx, "/grpcclient.test.Echo/Call", wrapperspb.String("hello"), out); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	if out.GetValue() != wantTraceID {
		t.Fatalf("server observed TraceID %q, want client's TraceID %q — trace did not propagate across the gRPC hop", out.GetValue(), wantTraceID)
	}
}

// TestDial_UnreachableTargetDoesNotBlockOrError is the regression required
// alongside the propagation change: grpc.NewClient (what Dial wraps) must
// keep its lazy-dial behavior — adding grpc.WithStatsHandler must not turn
// this into an eager, blocking, or error-returning connect.
func TestDial_UnreachableTargetDoesNotBlockOrError(t *testing.T) {
	conn, err := Dial("127.0.0.1:1")
	if err != nil {
		t.Fatalf("Dial to an unreachable target should not error eagerly, got: %v", err)
	}
	defer conn.Close()
}
