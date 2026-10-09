package grpcclient

import (
	"context"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// recordingInfraFleet answers the three calls project-service makes during project creation and records the
// tenant it received, as the real service would through TenantExtractionInterceptor.
type recordingInfraFleet struct {
	infrafleetv1.UnimplementedInfraFleetServiceServer
	mu      sync.Mutex
	tenants map[string]string
}

func (r *recordingInfraFleet) note(ctx context.Context, rpc string) {
	id, _ := tenant.TenantID(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[rpc] = id
}

func (r *recordingInfraFleet) GetFleetHealth(ctx context.Context, _ *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
	r.note(ctx, "GetFleetHealth")
	return &infrafleetv1.GetFleetHealthResponse{Statuses: []*infrafleetv1.DevServerHealth{{DevServerId: "ds-1", Reachable: true}}}, nil
}

func (r *recordingInfraFleet) CreateConnection(ctx context.Context, _ *infrafleetv1.CreateConnectionRequest) (*infrafleetv1.CreateConnectionResponse, error) {
	r.note(ctx, "CreateConnection")
	return &infrafleetv1.CreateConnectionResponse{ConnectionId: "conn-1"}, nil
}

func (r *recordingInfraFleet) Relay(ctx context.Context, _ *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
	r.note(ctx, "Relay")
	return &infrafleetv1.RelayResponse{ResultJson: `{"exists":true,"isDir":true}`}, nil
}

// Regression for BUG-E2E-003: project.create against a dev server failed with INFRA_NO_TENANT because two adapters
// forgot to attach the tenant. They now rely on the dial helper, so this test goes through a real connection built
// the way every service builds one.
func TestInfraFleetAdaptersCarryTheTenantThroughGrpcmwNewClient(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	fake := &recordingInfraFleet{tenants: map[string]string{}}
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.TenantExtractionInterceptor()))
	infrafleetv1.RegisterInfraFleetServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpcmw.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	client := infrafleetv1.NewInfraFleetServiceClient(conn)
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "tenant-5"), "user-5")

	reachable, err := NewInfraFleetHealthChecker(client).IsReachable(ctx, "tenant-5", "ds-1")
	if err != nil || !reachable {
		t.Fatalf("IsReachable = %v, %v", reachable, err)
	}
	relay := &DevServerRelay{client: client}
	connID, err := relay.CreateConnection(ctx, "ds-1", "/repo", "")
	if err != nil || connID != "conn-1" {
		t.Fatalf("CreateConnection = %q, %v", connID, err)
	}
	if _, err := relay.Relay(ctx, connID, "fs.checkPath", []byte(`{"path":"/repo"}`)); err != nil {
		t.Fatal(err)
	}

	for _, rpc := range []string{"GetFleetHealth", "CreateConnection", "Relay"} {
		if got := fake.tenants[rpc]; got != "tenant-5" {
			t.Errorf("%s reached infra-fleet with tenant %q, want tenant-5", rpc, got)
		}
	}
}
