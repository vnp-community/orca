package grpcmw

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stablyai/orca-go/common/tenant"
)

func TestStatsHandler_ReturnsNonNilServerOption(t *testing.T) {
	opt := StatsHandler()
	if opt == nil {
		t.Fatal("StatsHandler() returned nil")
	}
}

// echoServiceDesc is a minimal, hand-written grpc.ServiceDesc (no .proto
// needed — google.protobuf.StringValue, a real proto.Message already
// vendored alongside grpc-go, is enough request/response shape) whose
// single method either echoes ctx-derived facts back to the test, or
// panics, depending on the request string — just enough surface to prove
// ChainUnary's 3-interceptor composition still behaves correctly
// end-to-end after adding StatsHandler() alongside it (TASK-BE-FFT-001),
// without needing to inspect grpc.ChainUnaryInterceptor's opaque internal
// composition (no public API exposes that).
var echoServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpcmw.test.Echo",
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
					if req.(*wrapperspb.StringValue).GetValue() == "panic" {
						panic("boom")
					}
					tenantID, _ := tenant.TenantID(ctx)
					return wrapperspb.String(tenantID), nil
				}
				if interceptor == nil {
					return handler(ctx, req)
				}
				info := &grpc.UnaryServerInfo{FullMethod: "/grpcmw.test.Echo/Call"}
				return interceptor(ctx, req, info, handler)
			},
		},
	},
}

func echoCall(ctx context.Context, conn *grpc.ClientConn, in string) (string, error) {
	out := new(wrapperspb.StringValue)
	if err := conn.Invoke(ctx, "/grpcmw.test.Echo/Call", wrapperspb.String(in), out); err != nil {
		return "", err
	}
	return out.GetValue(), nil
}

var _ proto.Message = (*wrapperspb.StringValue)(nil) // sanity: real proto.Message, default codec applies with no extra registration

// TestChainUnary_SignatureAndBehaviorUnchanged is TASK-BE-FFT-001's
// mandatory regression test — proves adding StatsHandler() as a SEPARATE
// grpc.ServerOption never touched ChainUnary(logger)'s own composed
// behavior: recovery still catches panics (outermost), tenant metadata
// still reaches the handler's context, and both survive
// grpc.NewServer(ChainUnary(logger), StatsHandler()) being called with
// BOTH options together, exactly as every one of the 16 gRPC-serving
// services' main.go now does.
func TestChainUnary_SignatureAndBehaviorUnchanged(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	defer lis.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := grpc.NewServer(ChainUnary(logger), StatsHandler())
	srv.RegisterService(&echoServiceDesc, nil)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("tenant metadata reaches handler context (TenantExtractionInterceptor still runs)", func(t *testing.T) {
		md := metadata.Pairs(MetadataTenantID, "tenant-regression-test")
		callCtx := metadata.NewOutgoingContext(ctx, md)
		reply, err := echoCall(callCtx, conn, "hello")
		if err != nil {
			t.Fatalf("Invoke: %v", err)
		}
		if reply != "tenant-regression-test" {
			t.Fatalf("expected the handler to see tenant_id from metadata, got %q", reply)
		}
	})

	t.Run("panic is recovered as codes.Internal, not a crash (RecoveryInterceptor still outermost)", func(t *testing.T) {
		_, err := echoCall(ctx, conn, "panic")
		if err == nil {
			t.Fatal("expected an error for a panicking handler")
		}
		if status.Code(err) != codes.Internal {
			t.Fatalf("expected codes.Internal, got %v", status.Code(err))
		}
	})
}
