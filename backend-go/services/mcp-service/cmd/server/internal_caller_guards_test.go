package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/stablyai/orca-go/common/internalcaller"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func guardedClient(t *testing.T, serverToken string) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer(internalCallerServerOptions(serverToken, slog.New(slog.NewTextHandler(io.Discard, nil)))...)
	mcpv1.RegisterMcpServiceServer(srv, mcpv1.UnimplementedMcpServiceServer{})
	mcpv1.RegisterMcpRegistryServiceServer(srv, mcpv1.UnimplementedMcpRegistryServiceServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cc, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return cc
}

func withToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), internalcaller.MetadataKey, token)
}

// callStatus invokes a unary method with an empty message (valid for any proto message) and returns the gRPC code.
func callStatus(ctx context.Context, cc *grpc.ClientConn, method string) codes.Code {
	return status.Code(cc.Invoke(ctx, method, &emptypb.Empty{}, &emptypb.Empty{}))
}

func TestInternalCallerServerOptions_EmptyTokenRefusesEveryGuardedRPC(t *testing.T) {
	cc := guardedClient(t, "")
	methods := append(append([]string{}, gatewayOnlyMethods...), registryInternalMethods...)
	if len(methods) == 0 {
		t.Fatal("no guarded methods listed")
	}
	for _, m := range methods {
		for name, ctx := range map[string]context.Context{"no token": context.Background(), "empty token": withToken("")} {
			if got := callStatus(ctx, cc, m); got != codes.PermissionDenied {
				t.Errorf("%s (%s) with an unset server token: %v, want PermissionDenied", m, name, got)
			}
		}
	}
	st, err := cc.NewStream(context.Background(), &grpc.StreamDesc{ServerStreams: true}, mcpv1.McpService_StreamEvents_FullMethodName)
	if err == nil {
		if err = st.SendMsg(&emptypb.Empty{}); err == nil {
			err = st.CloseSend()
		}
		if err == nil {
			err = st.RecvMsg(&emptypb.Empty{})
		}
	}
	if status.Code(err) != codes.PermissionDenied {
		t.Errorf("StreamEvents with an unset server token: %v, want PermissionDenied", err)
	}
}

func TestInternalCallerServerOptions_ConfiguredTokenAdmitsOnlyItsHolder(t *testing.T) {
	cc := guardedClient(t, "s3cret")
	m := gatewayOnlyMethods[0]
	if got := callStatus(context.Background(), cc, m); got != codes.PermissionDenied {
		t.Fatalf("without the token: %v", got)
	}
	if got := callStatus(withToken("wrong"), cc, m); got != codes.PermissionDenied {
		t.Fatalf("with a wrong token: %v", got)
	}
	// Unimplemented comes from the stub behind the guard, so the call was admitted.
	if got := callStatus(withToken("s3cret"), cc, m); got != codes.Unimplemented {
		t.Fatalf("with the token: %v, want to reach the handler", got)
	}
}
