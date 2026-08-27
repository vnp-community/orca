package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// fakeInfraFleetClient is a minimal test double for
// infrafleetv1.InfraFleetServiceClient — embeds the (nil) interface so it
// satisfies every method, and overrides only the three this file's channel
// handlers actually call. Calling an unset method panics on a nil-pointer
// deref, which is fine: no test here should ever reach one.
type fakeInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	listDevServersFunc     func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error)
	registerDevServerFunc  func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error)
	getFleetHealthFunc     func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error)
	relayFunc              func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
	scanWorkspacePortsFunc func(ctx context.Context, in *infrafleetv1.ScanWorkspacePortsRequest) (*infrafleetv1.ScanWorkspacePortsResponse, error)
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

func (f *fakeInfraFleetClient) ScanWorkspacePorts(ctx context.Context, in *infrafleetv1.ScanWorkspacePortsRequest, _ ...grpc.CallOption) (*infrafleetv1.ScanWorkspacePortsResponse, error) {
	return f.scanWorkspacePortsFunc(ctx, in)
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

// ── TASK-007: crashReports.* tests ──────────────────────────────────────────

// TestCrashReportGetLatestPendingChannel_ReturnsNull verifies that the channel
// returns nil (JSON null) — the honest answer for a backend that has no crash
// reporting service. Frontend expects a nullable result.
func TestCrashReportGetLatestPendingChannel_ReturnsNull(t *testing.T) {
	r := NewRegistry()
	registerCrashReportChannels(r)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "crashReports.getLatestPending", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("want nil (no crash report in backend-go), got %v", result)
	}
}

// TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs verifies that the
// handler does not panic or error when called with no args or extra args.
func TestCrashReportGetLatestPendingChannel_AcceptsAnyArgs(t *testing.T) {
	r := NewRegistry()
	registerCrashReportChannels(r)

	// no args
	if _, err := r.Dispatch(context.Background(), Identity{}, "crashReports.getLatestPending", nil); err != nil {
		t.Errorf("with nil args: unexpected error: %v", err)
	}
	// extra args (frontend may pass session id etc.)
	args := argsJSON(t, map[string]any{"sessionId": "abc-123"})
	if _, err := r.Dispatch(context.Background(), Identity{}, "crashReports.getLatestPending", args); err != nil {
		t.Errorf("with extra args: unexpected error: %v", err)
	}
}

// ── TASK-007: rateLimits.* tests ─────────────────────────────────────────────

// fakeRateLimitReader is a test double for the rateLimitReader interface.
type fakeRateLimitReader struct {
	rps   float64
	burst int
}

func (f *fakeRateLimitReader) RPS() float64 { return f.rps }
func (f *fakeRateLimitReader) Burst() int   { return f.burst }

// TestRateLimitsGetChannel_ReturnsConfiguredValues verifies the channel exposes
// the limiter's configured RPS and burst — not live per-tenant counters.
func TestRateLimitsGetChannel_ReturnsConfiguredValues(t *testing.T) {
	r := NewRegistry()
	rl := &fakeRateLimitReader{rps: 100.0, burst: 200}
	registerRateLimitChannels(r, rl)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "rateLimits.get", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, ok := result.(rateLimitInfo)
	if !ok {
		t.Fatalf("unexpected result type %T, want rateLimitInfo", result)
	}
	if info.RequestsPerSecond != 100.0 {
		t.Errorf("want RequestsPerSecond=100.0, got %f", info.RequestsPerSecond)
	}
	if info.Burst != 200 {
		t.Errorf("want Burst=200, got %d", info.Burst)
	}
}

// TestRateLimitsGetChannel_JSONFieldNames verifies the JSON wire format has
// the field names the frontend expects (camelCase).
func TestRateLimitsGetChannel_JSONFieldNames(t *testing.T) {
	r := NewRegistry()
	rl := &fakeRateLimitReader{rps: 10.0, burst: 20}
	registerRateLimitChannels(r, rl)

	result, err := r.Dispatch(context.Background(), Identity{}, "rateLimits.get", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := m["requestsPerSecond"]; !ok {
		t.Errorf("JSON field 'requestsPerSecond' missing; got keys: %v", m)
	}
	if _, ok := m["burst"]; !ok {
		t.Errorf("JSON field 'burst' missing; got keys: %v", m)
	}
}

// ── TASK-009: rpcTimeout tests ───────────────────────────────────────────────

// TestRPCTimeoutConstant_ShorterThanInvokeTimeout documents the required
// relationship: rpcTimeout < invokeTimeout. Failing this test means the
// per-RPC deadline no longer leaves margin for write-back (SOL-001 / TASK-001).
func TestRPCTimeoutConstant_ShorterThanInvokeTimeout(t *testing.T) {
	if rpcTimeout >= invokeTimeout {
		t.Errorf("rpcTimeout (%s) must be < invokeTimeout (%s); "+
			"rpcTimeout occupies the dispatch window, invokeTimeout must envelope it",
			rpcTimeout, invokeTimeout)
	}
	// Write margin must be at least 5s (writeTimeout from SOL-001).
	margin := invokeTimeout - rpcTimeout
	if margin < 5*time.Second {
		t.Errorf("write margin (invokeTimeout - rpcTimeout = %s) must be >= 5s "+
			"to accommodate writeTimeout (SOL-001)", margin)
	}
}

// TestDevServerListChannel_FailsFastWhenServiceSlow verifies that devServer.list
// returns an error within rpcTimeout + small margin when infra-fleet-service
// blocks, NOT after the full invokeTimeout (25s). Regression guard for BUG-003.
func TestDevServerListChannel_FailsFastWhenServiceSlow(t *testing.T) {
	fake := &fakeInfraFleetClient{
		listDevServersFunc: func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error) {
			// Simulate a slow/hung service: block until the per-RPC context
			// is cancelled (i.e. until rpcTimeout fires).
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	start := time.Now()
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.list", nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error from slow service, got nil")
	}
	// Must fail within rpcTimeout (8s) + 2s margin, not after 25s.
	maxAllowed := rpcTimeout + 2*time.Second
	if elapsed > maxAllowed {
		t.Errorf("devServer.list took %s, want < %s (rpcTimeout + margin); "+
			"infra-fleet-service timeout not being enforced", elapsed, maxAllowed)
	}
}

// TestDevServerAddChannel_FailsFastWhenServiceSlow verifies the same rpcTimeout
// enforcement for devServer.add.
func TestDevServerAddChannel_FailsFastWhenServiceSlow(t *testing.T) {
	fake := &fakeInfraFleetClient{
		registerDevServerFunc: func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	r := NewRegistry()
	registerDevServerChannels(r, fake)

	args := argsJSON(t, map[string]any{"name": "slow-server", "connectionType": "relay-ssh"})

	start := time.Now()
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "devServer.add", args)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error from slow service, got nil")
	}
	if elapsed > rpcTimeout+2*time.Second {
		t.Errorf("devServer.add took %s, want < %s", elapsed, rpcTimeout+2*time.Second)
	}
}

// TestFleetHealthCheckAll_FailsFastWhenServiceSlow verifies rpcTimeout
// enforcement for fleet.health.checkAll.
func TestFleetHealthCheckAll_FailsFastWhenServiceSlow(t *testing.T) {
	fake := &fakeInfraFleetClient{
		getFleetHealthFunc: func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	r := NewRegistry()
	registerFleetChannels(r, fake)

	args := argsJSON(t, map[string]any{"serverIds": []string{"ds-1"}})

	start := time.Now()
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "fleet.health.checkAll", args)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want error from slow service, got nil")
	}
	if elapsed > rpcTimeout+2*time.Second {
		t.Errorf("fleet.health.checkAll took %s, want < %s", elapsed, rpcTimeout+2*time.Second)
	}
}

// ── TASK-INT-03-03: preflight.check tests ───────────────────────────────────

// TestPreflightCheckChannel_NoConnectionIDIsLocalOnly verifies that an empty
// connectionId returns local-only results with zero RPC calls made — no
// fake RPC funcs are set, so any call would panic on the nil-func deref.
func TestPreflightCheckChannel_NoConnectionIDIsLocalOnly(t *testing.T) {
	r := NewRegistry()
	registerPreflightChannels(r, &fakeInfraFleetClient{})

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "preflight.check", argsJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	results, ok := result.([]usecase.PreflightCheckResult)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	if len(results) != 1 || results[0].ID != "git" || results[0].Source != usecase.PreflightSourceLocal {
		t.Fatalf("expected local-only [git], got %+v", results)
	}
}

// TestPreflightCheckChannel_FleetHealthFailureProducesConnectivityWarning
// verifies a hard GetFleetHealth failure degrades to the relay-connectivity
// warning while the local git check is still present.
func TestPreflightCheckChannel_FleetHealthFailureProducesConnectivityWarning(t *testing.T) {
	fake := &fakeInfraFleetClient{
		getFleetHealthFunc: func(context.Context, *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			return nil, errors.New("connection refused")
		},
	}
	r := NewRegistry()
	registerPreflightChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "preflight.check", argsJSON(t, map[string]any{"connectionId": "conn-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	results, ok := result.([]usecase.PreflightCheckResult)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	var sawGit, sawWarning bool
	for _, c := range results {
		if c.ID == "git" {
			sawGit = true
		}
		if c.ID == "relay-connectivity" && c.Status == usecase.PreflightWarning {
			sawWarning = true
		}
	}
	if !sawGit || !sawWarning {
		t.Fatalf("expected local git check + relay-connectivity warning, got %+v", results)
	}
}

// TestPreflightCheckChannel_AuthStatusRelayFailureIsPerCheckSkip simulates a
// relay-ssh Dev Server (github.auth.status/gitlab.auth.status not
// implemented) — those 2 checks must come back skip-status while
// disk-space/port-availability still succeed, and every result must carry a
// non-empty Source.
func TestPreflightCheckChannel_AuthStatusRelayFailureIsPerCheckSkip(t *testing.T) {
	fake := &fakeInfraFleetClient{
		getFleetHealthFunc: func(context.Context, *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			return &infrafleetv1.GetFleetHealthResponse{Statuses: []*infrafleetv1.DevServerHealth{
				{DevServerId: "dev-1", DiskPercent: 10},
			}}, nil
		},
		scanWorkspacePortsFunc: func(context.Context, *infrafleetv1.ScanWorkspacePortsRequest) (*infrafleetv1.ScanWorkspacePortsResponse, error) {
			return &infrafleetv1.ScanWorkspacePortsResponse{}, nil
		},
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			return nil, errors.New("agent method not found")
		},
	}
	r := NewRegistry()
	registerPreflightChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "preflight.check", argsJSON(t, map[string]any{"connectionId": "conn-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	results, ok := result.([]usecase.PreflightCheckResult)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}

	byID := make(map[string]usecase.PreflightCheckResult, len(results))
	for _, c := range results {
		if c.Source == "" {
			t.Errorf("PreflightCheckResult %q has empty Source — every result must be tagged", c.ID)
		}
		byID[c.ID] = c
	}

	if byID["disk-space"].Status != usecase.PreflightOK {
		t.Errorf("disk-space = %+v, want ok", byID["disk-space"])
	}
	if byID["port-availability"].Status != usecase.PreflightOK {
		t.Errorf("port-availability = %+v, want ok", byID["port-availability"])
	}
	for _, id := range []string{"github-cli-auth", "gitlab-cli-auth"} {
		if byID[id].Status != usecase.PreflightSkip {
			t.Errorf("%s = %+v, want skip", id, byID[id])
		}
	}
}
