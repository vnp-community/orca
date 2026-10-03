package main

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/internalcaller"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type guardedMcpStub struct {
	mcpv1.UnimplementedMcpServiceServer
}

func (guardedMcpStub) StreamEvents(*emptypb.Empty, mcpv1.McpService_StreamEventsServer) error {
	return nil
}
func (guardedMcpStub) GetKillState(context.Context, *mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) {
	return &mcpv1.GetKillStateResponse{}, nil
}

// The gateway must present the shared secret on the unary AND the stream path.
func TestDialMCPServiceSendsTokenOnStreams(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(internalcaller.Guard("s3cret", mcpv1.McpService_GetKillState_FullMethodName)),
		grpc.ChainStreamInterceptor(internalcaller.StreamGuard("s3cret", mcpv1.McpService_StreamEvents_FullMethodName)))
	mcpv1.RegisterMcpServiceServer(srv, guardedMcpStub{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	t.Setenv("MCP_INTERNAL_CALLER_TOKEN", "s3cret")
	cc, err := dialMCPService(lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	c := mcpv1.NewMcpServiceClient(cc)
	if _, err := c.GetKillState(context.Background(), &mcpv1.GetKillStateRequest{}); err != nil {
		t.Fatalf("unary: %v", err)
	}
	st, err := c.StreamEvents(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("stream open: %v", err)
	}
	if _, err := st.Recv(); status.Code(err) == codes.PermissionDenied {
		t.Fatalf("stream was refused: %v", err)
	}
}
