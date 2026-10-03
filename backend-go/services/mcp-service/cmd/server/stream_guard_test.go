package main

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/internalcaller"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type streamStub struct {
	mcpv1.UnimplementedMcpServiceServer
}

func (streamStub) StreamEvents(*emptypb.Empty, mcpv1.McpService_StreamEventsServer) error { return nil }

func TestStreamEventsIsGuarded(t *testing.T) {
	if len(gatewayOnlyStreamMethods) != 1 || gatewayOnlyStreamMethods[0] != mcpv1.McpService_StreamEvents_FullMethodName {
		t.Fatalf("StreamEvents must be in gatewayOnlyStreamMethods: %v", gatewayOnlyStreamMethods)
	}
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer(grpc.ChainStreamInterceptor(internalcaller.StreamGuard("s3cret", gatewayOnlyStreamMethods...)))
	mcpv1.RegisterMcpServiceServer(srv, streamStub{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	dial := func(opts ...grpc.DialOption) mcpv1.McpServiceClient {
		opts = append(opts, grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		cc, err := grpc.NewClient("passthrough:///bufnet", opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cc.Close() })
		return mcpv1.NewMcpServiceClient(cc)
	}
	recv := func(c mcpv1.McpServiceClient) error {
		st, err := c.StreamEvents(context.Background(), &emptypb.Empty{})
		if err != nil {
			return err
		}
		_, err = st.Recv()
		return err
	}
	if got := status.Code(recv(dial())); got != codes.PermissionDenied {
		t.Fatalf("no token: want PermissionDenied, got %v", got)
	}
	if got := status.Code(recv(dial(grpc.WithChainStreamInterceptor(internalcaller.StreamClientInterceptor("wrong"))))); got != codes.PermissionDenied {
		t.Fatalf("wrong token: want PermissionDenied, got %v", got)
	}
	// The stub ends the stream at once, so io.EOF (not PermissionDenied) means the guard let it through.
	if got := status.Code(recv(dial(grpc.WithChainStreamInterceptor(internalcaller.StreamClientInterceptor("s3cret"))))); got == codes.PermissionDenied {
		t.Fatal("valid token must pass the guard")
	}
}
