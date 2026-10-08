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

	"github.com/stablyai/orca-go/common/internalcaller"
	mcpgrpc "github.com/stablyai/orca-go/services/mcp-service/internal/adapter/grpc"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

func TestExternalCallRPCsAreInternalOnly(t *testing.T) {
	want := []string{mcpv1.McpRegistryService_CallExternalTool_FullMethodName, mcpv1.McpRegistryService_ReadExternalResource_FullMethodName}
	for _, m := range want {
		found := false
		for _, g := range registryInternalMethods {
			found = found || g == m
		}
		if !found {
			t.Fatalf("%s must be in registryInternalMethods", m)
		}
	}

	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(internalcaller.Guard("s3cret", registryInternalMethods...)))
	// nil use cases: a request that passes the guard answers Unavailable, never PermissionDenied.
	mcpv1.RegisterMcpRegistryServiceServer(srv, mcpgrpc.NewRegistryServer(nil, nil, nil))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	dial := func(opts ...grpc.DialOption) mcpv1.McpRegistryServiceClient {
		opts = append(opts, grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		cc, err := grpc.NewClient("passthrough:///bufnet", opts...)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cc.Close() })
		return mcpv1.NewMcpRegistryServiceClient(cc)
	}
	calls := map[string]func(mcpv1.McpRegistryServiceClient) error{
		"CallExternalTool": func(c mcpv1.McpRegistryServiceClient) error {
			_, err := c.CallExternalTool(context.Background(), &mcpv1.CallExternalToolRequest{})
			return err
		},
		"ReadExternalResource": func(c mcpv1.McpRegistryServiceClient) error {
			_, err := c.ReadExternalResource(context.Background(), &mcpv1.ReadExternalResourceRequest{})
			return err
		},
	}
	for name, call := range calls {
		if got := status.Code(call(dial())); got != codes.PermissionDenied {
			t.Errorf("%s without token: want PermissionDenied, got %v", name, got)
		}
		if got := status.Code(call(dial(grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor("wrong"))))); got != codes.PermissionDenied {
			t.Errorf("%s wrong token: want PermissionDenied, got %v", name, got)
		}
		if got := status.Code(call(dial(grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor("s3cret"))))); got == codes.PermissionDenied {
			t.Errorf("%s with the internal token must pass the guard, got %v", name, got)
		}
	}
}
