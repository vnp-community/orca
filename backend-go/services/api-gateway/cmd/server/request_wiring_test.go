package main

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	grpcadapter "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// startMetadataServer serves the gRPC health service and reports the metadata of each call.
func startMetadataServer(t *testing.T) (addr string, seen <-chan metadata.MD) {
	t.Helper()
	ch := make(chan metadata.MD, 4)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		ch <- md
		return h(ctx, req)
	}))
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), ch
}

func callCheck(t *testing.T, addr string, ctx context.Context) {
	t.Helper()
	conn, err := dialRequestService(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestDialRequestService_WithTokenSendsInternalToken(t *testing.T) {
	t.Setenv("REQUEST_INTERNAL_CALLER_TOKEN", "secret-token")
	addr, seen := startMetadataServer(t)
	ctx := tenant.WithActorType(context.Background(), tenant.ActorAgent)
	callCheck(t, addr, grpcadapter.AttachIdentity(ctx, usecase.Identity{TenantID: "t1", UserID: "u1"}))
	md := <-seen
	if got := md.Get(internalcaller.MetadataKey); len(got) != 1 || got[0] != "secret-token" {
		t.Fatalf("internal token metadata = %v", got)
	}
	if got := md.Get("x-orca-actor-type"); len(got) != 1 || got[0] != "agent" {
		t.Fatalf("actor metadata = %v", got)
	}
}

func TestDialRequestService_EmptyTokenSendsNone(t *testing.T) {
	t.Setenv("REQUEST_INTERNAL_CALLER_TOKEN", "")
	addr, seen := startMetadataServer(t)
	callCheck(t, addr, context.Background())
	if got := (<-seen).Get(internalcaller.MetadataKey); len(got) != 0 {
		t.Fatalf("no token expected, got %v", got)
	}
}
