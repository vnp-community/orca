package grpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

type memoryCapabilityStore struct {
	mu       sync.Mutex
	profiles map[string]domain.CapabilityProfile
}

func (s *memoryCapabilityStore) Get(_ context.Context, tenantID, devServerID string) (domain.CapabilityProfile, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.profiles[tenantID+":"+devServerID]
	return p, ok, nil
}

func (s *memoryCapabilityStore) Upsert(_ context.Context, p domain.CapabilityProfile) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.profiles[p.TenantID+":"+p.DevServerID]
	s.profiles[p.TenantID+":"+p.DevServerID] = p
	return old.Fingerprint, ok, nil
}

type discardOutbox struct{}

func (discardOutbox) EnqueueOutboxEvent(context.Context, string, string, string, time.Time, int, []byte) error {
	return nil
}

// probingAgent answers agent.capabilities with a fixed report.
type probingAgent struct {
	capsFakeAgentClient
	result map[string]any
	err    error
}

func (a *probingAgent) Exec(context.Context, domain.DevServer, string, map[string]any) (map[string]any, error) {
	return a.result, a.err
}

func newCapabilityServer(t *testing.T, agent *probingAgent) *Server {
	t.Helper()
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}
	repo := &capsFakeRepo{relayFakeDevServerRepository: relayFakeDevServerRepository{ds: ds}}
	resolver := &fakeConnectionResolver{connected: true, devServer: ds}
	store := &memoryCapabilityStore{profiles: map[string]domain.CapabilityProfile{}}
	refresh := usecase.NewRefreshDevServerCapabilities(repo, agent, store, discardOutbox{}, usecase.RealClock{}, time.Minute)
	get := usecase.NewGetDevServerCapabilities(resolver, repo, store, refresh, agent, time.Hour, usecase.RealClock{})
	return (&Server{}).WithDevServerCapabilities(get)
}

func TestServer_GetDevServerCapabilities_UnimplementedWhenNotConfigured(t *testing.T) {
	_, err := (&Server{}).GetDevServerCapabilities(context.Background(), &infrafleetv1.GetDevServerCapabilitiesRequest{DevServerId: "ds-1"})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("want Unimplemented, got %v", err)
	}
}

func TestServer_GetDevServerCapabilities_MapsFields(t *testing.T) {
	agent := &probingAgent{
		capsFakeAgentClient: capsFakeAgentClient{
			isConnected: true, lastHandshakeOK: true,
			lastHandshakeInfo: usecase.HandshakeInfo{
				Platform: "linux", Arch: "x64", NodeVersion: "v22.3.0",
				Features: []string{"agent.capabilities", "agent.execPrompt"}, ProtocolVersion: 2, BuildVersion: "2.2.0",
			},
		},
		result: map[string]any{"schemaVersion": float64(1), "partial": false},
	}
	s := newCapabilityServer(t, agent)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")

	resp, err := s.GetDevServerCapabilities(ctx, &infrafleetv1.GetDevServerCapabilitiesRequest{DevServerId: "ds-1"})
	if err != nil {
		t.Fatalf("GetDevServerCapabilities: %v", err)
	}
	if resp.GetDevServerId() != "ds-1" || resp.GetSource() != "probe" || resp.GetAgentBuildVersion() != "2.2.0" ||
		resp.GetProtocolVersion() != 2 || resp.GetDegraded() || !resp.GetConnected() {
		t.Errorf("unexpected response: %+v", resp)
	}
	if len(resp.GetFeatures()) != 2 || resp.GetFeatures()[0] != "agent.capabilities" {
		t.Errorf("features = %v", resp.GetFeatures())
	}
	if resp.GetProfileJson() != `{"partial":false,"schemaVersion":1}` {
		t.Errorf("profile_json = %q", resp.GetProfileJson())
	}
	if resp.GetProbedAt() == nil || resp.GetProbedAt().AsTime().IsZero() {
		t.Errorf("probed_at not set")
	}
}

func TestServer_GetDevServerCapabilities_OldAgentIsDegraded(t *testing.T) {
	agent := &probingAgent{
		capsFakeAgentClient: capsFakeAgentClient{isConnected: true, lastHandshakeOK: true,
			lastHandshakeInfo: usecase.HandshakeInfo{Platform: "linux"}},
		err: domain.ErrAgentMethodNotFound,
	}
	s := newCapabilityServer(t, agent)
	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	resp, err := s.GetDevServerCapabilities(ctx, &infrafleetv1.GetDevServerCapabilitiesRequest{ConnectionId: "conn-1"})
	if err != nil {
		t.Fatalf("GetDevServerCapabilities: %v", err)
	}
	if resp.GetSource() != "handshake_only" || !resp.GetDegraded() || len(resp.GetFeatures()) != 0 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestServer_GetDevServerCapabilities_ErrorMapping(t *testing.T) {
	agent := &probingAgent{capsFakeAgentClient: capsFakeAgentClient{isConnected: false}}
	s := newCapabilityServer(t, agent)
	withTenant := tenant.WithTenantID(context.Background(), "tenant-1")

	cases := []struct {
		name string
		ctx  context.Context
		req  *infrafleetv1.GetDevServerCapabilitiesRequest
		want codes.Code
	}{
		{"no tenant", context.Background(), &infrafleetv1.GetDevServerCapabilitiesRequest{DevServerId: "ds-1"}, codes.Unauthenticated},
		{"both ids", withTenant, &infrafleetv1.GetDevServerCapabilitiesRequest{DevServerId: "ds-1", ConnectionId: "c"}, codes.InvalidArgument},
		{"no id", withTenant, &infrafleetv1.GetDevServerCapabilitiesRequest{}, codes.InvalidArgument},
		{"never probed and disconnected", withTenant, &infrafleetv1.GetDevServerCapabilitiesRequest{DevServerId: "ds-1"}, codes.NotFound},
	}
	for _, tc := range cases {
		if _, err := s.GetDevServerCapabilities(tc.ctx, tc.req); status.Code(err) != tc.want {
			t.Errorf("%s: want %v, got %v", tc.name, tc.want, err)
		}
	}
}

// Over a real gRPC listener and through the WithAgentSessionList wrapper
// main.go registers: the RPC must be served, not fall through to
// UnimplementedInfraFleetServiceServer.
func TestServer_GetDevServerCapabilities_ServedOverGRPCThroughWrapper(t *testing.T) {
	agent := &probingAgent{
		capsFakeAgentClient: capsFakeAgentClient{isConnected: true, lastHandshakeOK: true,
			lastHandshakeInfo: usecase.HandshakeInfo{Platform: "linux", BuildVersion: "2.2.0", ProtocolVersion: 2}},
		result: map[string]any{"schemaVersion": float64(1)},
	}
	inner := newCapabilityServer(t, agent)

	srv := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		return h(tenant.WithTenantID(ctx, "tenant-1"), req)
	}))
	infrafleetv1.RegisterInfraFleetServiceServer(srv, WithAgentSessionList(inner, nil))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := infrafleetv1.NewInfraFleetServiceClient(conn).GetDevServerCapabilities(ctx, &infrafleetv1.GetDevServerCapabilitiesRequest{DevServerId: "ds-1"})
	if err != nil {
		t.Fatalf("rpc: %v", err)
	}
	if resp.GetSource() != "probe" || resp.GetAgentBuildVersion() != "2.2.0" {
		t.Errorf("unexpected response %+v", resp)
	}
}
