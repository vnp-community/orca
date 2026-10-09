package grpcmw

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stablyai/orca-go/common/tenant"
)

func TestWithForwardedIdentity(t *testing.T) {
	base := tenant.WithRole(tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1"), "admin")

	t.Run("forwards what the context carries", func(t *testing.T) {
		md, _ := metadata.FromOutgoingContext(withForwardedIdentity(tenant.WithClientIP(base, "10.0.0.9")))
		want := map[string]string{MetadataTenantID: "t1", MetadataUserID: "u1", MetadataRole: "admin", MetadataClientIP: "10.0.0.9"}
		for k, v := range want {
			if got := md.Get(k); len(got) != 1 || got[0] != v {
				t.Errorf("%s = %v, want %q", k, got, v)
			}
		}
		if got := md.Get(MetadataActorType); len(got) != 0 {
			t.Errorf("a user actor must not be sent explicitly, got %v", got)
		}
	})

	t.Run("always forwards a non-user actor so it cannot be mistaken for a person", func(t *testing.T) {
		for _, actor := range []string{tenant.ActorAgent, tenant.ActorSystem} {
			md, _ := metadata.FromOutgoingContext(withForwardedIdentity(tenant.WithActorType(base, actor)))
			if got := md.Get(MetadataActorType); len(got) != 1 || got[0] != actor {
				t.Errorf("actor %s: got %v", actor, got)
			}
		}
	})

	t.Run("does not override identity the adapter set on purpose", func(t *testing.T) {
		ctx := metadata.AppendToOutgoingContext(base, MetadataUserID, "someone-else")
		md, _ := metadata.FromOutgoingContext(withForwardedIdentity(ctx))
		if got := md.Get(MetadataUserID); len(got) != 1 || got[0] != "someone-else" {
			t.Errorf("user id = %v, want only someone-else", got)
		}
		if got := md.Get(MetadataTenantID); len(got) != 1 || got[0] != "t1" {
			t.Errorf("tenant should still be filled in, got %v", got)
		}
	})

	t.Run("never invents a tenant", func(t *testing.T) {
		md, ok := metadata.FromOutgoingContext(withForwardedIdentity(context.Background()))
		if ok && len(md) != 0 {
			t.Errorf("empty context produced metadata %v", md)
		}
	})
}

// A real client and server over an in-memory connection: the identity set on the caller's context must arrive on the
// callee's context through the production TenantExtractionInterceptor.
func TestClientIdentityRoundTrip(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	var seen context.Context
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		TenantExtractionInterceptor(),
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			seen = ctx
			return h(ctx, req)
		},
	))
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	opts := append([]grpc.DialOption{
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, ClientIdentityDialOptions()...)
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-9"), "user-9")
	if _, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := tenant.TenantID(seen); got != "tenant-9" {
		t.Errorf("callee tenant = %q", got)
	}
	if got, _ := tenant.UserID(seen); got != "user-9" {
		t.Errorf("callee user = %q", got)
	}
}
