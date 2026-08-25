package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeInfraFleetClient is a minimal test double for
// infrafleetv1.InfraFleetServiceClient — embeds the (nil) interface so it
// satisfies every method, and overrides only the three this file's channel
// handlers actually call. Calling an unset method panics on a nil-pointer
// deref, which is fine: no test here should ever reach one.
type fakeInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	listDevServersFunc    func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error)
	registerDevServerFunc func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error)
	getFleetHealthFunc    func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error)
	relayFunc             func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeInfraFleetClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}

func (f *fakeInfraFleetClient) ListDevServers(ctx context.Context, in *infrafleetv1.ListDevServersRequest, _ ...grpc.CallOption) (*infrafleetv1.ListDevServersResponse, error) {
	return f.listDevServersFunc(ctx, in)
}

func (f *fakeInfraFleetClient) RegisterDevServer(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest, _ ...grpc.CallOption) (*infrafleetv1.RegisterDevServerResponse, error) {
	return f.registerDevServerFunc(ctx, in)
}

func (f *fakeInfraFleetClient) GetFleetHealth(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest, _ ...grpc.CallOption) (*infrafleetv1.GetFleetHealthResponse, error) {
	return f.getFleetHealthFunc(ctx, in)
}

// outgoingTenantUser reads back the metadata AttachIdentity is expected to
// have stamped onto ctx, so tests can assert it actually ran.
func outgoingTenantUser(ctx context.Context) (tenant, user string) {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return "", ""
	}
	if v := md.Get(grpcmw.MetadataTenantID); len(v) > 0 {
		tenant = v[0]
	}
	if v := md.Get(grpcmw.MetadataUserID); len(v) > 0 {
		user = v[0]
	}
	return tenant, user
}

func argsJSON(t *testing.T, v any) []json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling test args: %v", err)
	}
	return []json.RawMessage{raw}
}

func TestDevServerListChannel_Success(t *testing.T) {
	var gotCtx context.Context
	fake := &fakeInfraFleetClient{
		listDevServersFunc: func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
			gotCtx = ctx
			return &infrafleetv1.ListDevServersResponse{
				DevServers: []*infrafleetv1.DevServer{
					{Id: "ds-1", TenantId: "tenant-1", Host: "ws://devserver.local:6799", Mode: infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET},
					{Id: "ds-2", TenantId: "tenant-1", Host: "10.0.0.5", Mode: infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH},
				},
			}, nil
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "devServer.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	views, ok := result.([]devServerView)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if len(views) != 2 {
		t.Fatalf("want 2 dev servers, got %d", len(views))
	}
	if views[0].ID != "ds-1" || views[0].ConnectionType != "relay-websocket" || views[0].Status != "disconnected" {
		t.Errorf("unexpected first view: %+v", views[0])
	}
	if views[0].WSUrl == nil || *views[0].WSUrl != "ws://devserver.local:6799" {
		t.Errorf("expected WSUrl to carry host, got %+v", views[0].WSUrl)
	}
	if views[1].ConnectionType != "relay-ssh" {
		t.Errorf("want relay-ssh, got %q", views[1].ConnectionType)
	}

	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

func TestDevServerListChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("infra-fleet-service unavailable")
	fake := &fakeInfraFleetClient{
		listDevServersFunc: func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
			return nil, wantErr
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.list", nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want error %v, got %v", wantErr, err)
	}
}

func TestDevServerAddChannel_Success(t *testing.T) {
	var gotReq *infrafleetv1.RegisterDevServerRequest
	var gotCtx context.Context
	fake := &fakeInfraFleetClient{
		registerDevServerFunc: func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			gotCtx = ctx
			gotReq = in
			return &infrafleetv1.RegisterDevServerResponse{
				DevServer: &infrafleetv1.DevServer{Id: "ds-new", TenantId: in.TenantId, Host: in.Host, Mode: in.Mode},
			}, nil
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"name":           "MacBook Pro M3",
		"connectionType": "direct-websocket",
		"wsUrl":          "ws://devserver.local:6799",
	})

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "devServer.add", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReq.TenantId != "tenant-1" {
		t.Errorf("want tenant-1 in request, got %q", gotReq.TenantId)
	}
	if gotReq.Host != "ws://devserver.local:6799" {
		t.Errorf("want wsUrl to win host precedence, got %q", gotReq.Host)
	}
	if gotReq.Mode != infrafleetv1.ConnectionMode_CONNECTION_MODE_DIRECT_WEBSOCKET {
		t.Errorf("unexpected mode: %v", gotReq.Mode)
	}

	view, ok := result.(devServerView)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if view.ID != "ds-new" || view.ConnectionType != "direct-websocket" {
		t.Errorf("unexpected view: %+v", view)
	}

	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

func TestDevServerAddChannel_HostPrecedenceFallsBackToNameWhenNoWSUrlOrSSHTarget(t *testing.T) {
	var gotReq *infrafleetv1.RegisterDevServerRequest
	fake := &fakeInfraFleetClient{
		registerDevServerFunc: func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			gotReq = in
			return &infrafleetv1.RegisterDevServerResponse{DevServer: &infrafleetv1.DevServer{Id: "ds-x"}}, nil
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"name":           "fallback-name",
		"connectionType": "relay-ssh",
	})

	if _, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.add", args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.Host != "fallback-name" {
		t.Errorf("want name fallback, got %q", gotReq.Host)
	}
}

func TestDevServerAddChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("host unreachable")
	fake := &fakeInfraFleetClient{
		registerDevServerFunc: func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			return nil, wantErr
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	args := argsJSON(t, map[string]any{"name": "x", "connectionType": "relay-ssh"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.add", args)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want error %v, got %v", wantErr, err)
	}
}

func TestFleetHealthCheckAllChannel_FiltersByRequestedServerIDs(t *testing.T) {
	var gotCtx context.Context
	fake := &fakeInfraFleetClient{
		getFleetHealthFunc: func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			gotCtx = ctx
			return &infrafleetv1.GetFleetHealthResponse{
				Statuses: []*infrafleetv1.DevServerHealth{
					{DevServerId: "ds-1", Reachable: true, CpuPercent: 12.5, RamPercent: 40, DiskPercent: 60, LatencyMs: 5},
					{DevServerId: "ds-2", Reachable: false, CpuPercent: 0, RamPercent: 0, DiskPercent: 0, LatencyMs: 0},
					{DevServerId: "ds-3-not-requested", Reachable: true},
				},
			}, nil
		},
	}

	r := NewRegistry()
	registerFleetChannels(r, fake)

	args := argsJSON(t, map[string]any{"serverIds": []string{"ds-1", "ds-2"}})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "fleet.health.checkAll", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	views, ok := result.([]serverHealthView)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if len(views) != 2 {
		t.Fatalf("want 2 filtered results (ds-3 excluded), got %d: %+v", len(views), views)
	}
	byID := map[string]serverHealthView{}
	for _, v := range views {
		byID[v.ServerID] = v
	}
	if !byID["ds-1"].IsReachable || byID["ds-1"].CPUUsagePercent != 12.5 {
		t.Errorf("unexpected ds-1 view: %+v", byID["ds-1"])
	}
	if byID["ds-2"].IsReachable {
		t.Errorf("ds-2 should be unreachable: %+v", byID["ds-2"])
	}
	if _, present := byID["ds-3-not-requested"]; present {
		t.Errorf("ds-3-not-requested should have been filtered out")
	}

	tenant, user := outgoingTenantUser(gotCtx)
	if tenant != "tenant-1" || user != "user-1" {
		t.Errorf("AttachIdentity not applied: tenant=%q user=%q", tenant, user)
	}
}

func TestFleetHealthCheckAllChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("infra-fleet-service unavailable")
	fake := &fakeInfraFleetClient{
		getFleetHealthFunc: func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			return nil, wantErr
		},
	}

	r := NewRegistry()
	registerFleetChannels(r, fake)

	args := argsJSON(t, map[string]any{"serverIds": []string{"ds-1"}})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "fleet.health.checkAll", args)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want error %v, got %v", wantErr, err)
	}
}
