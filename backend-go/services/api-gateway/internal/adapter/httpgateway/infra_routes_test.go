package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeInfraFleetServiceClient implements infrafleetv1.InfraFleetServiceClient
// with one settable hook per RPC, so individual tests can assert on the
// request they receive and control the response/error returned — same
// shape as the fakes usage_routes tests would use, hand-rolled here since
// there's no generated mock for this client.
type fakeInfraFleetServiceClient struct {
	registerDevServerFn  func(*infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error)
	resolveConnectionFn  func(*infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
	createSshTargetFn    func(*infrafleetv1.CreateSshTargetRequest) (*infrafleetv1.CreateSshTargetResponse, error)
	getFleetHealthFn     func(*infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error)
	scanWorkspacePortsFn func(*infrafleetv1.ScanWorkspacePortsRequest) (*infrafleetv1.ScanWorkspacePortsResponse, error)
	listDevServersFn     func(*infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error)
	createConnectionFn   func(*infrafleetv1.CreateConnectionRequest) (*infrafleetv1.CreateConnectionResponse, error)
	relayFn              func(*infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)

	listSshTargetsFn      func(*infrafleetv1.ListSshTargetsRequest) (*infrafleetv1.ListSshTargetsResponse, error)
	getSshStateFn         func(*infrafleetv1.GetSshStateRequest) (*infrafleetv1.GetSshStateResponse, error)
	establishConnectionFn func(*infrafleetv1.EstablishConnectionRequest) (*infrafleetv1.Connection, error)
	killWorkspacePortFn   func(*infrafleetv1.KillWorkspacePortRequest) (*infrafleetv1.KillWorkspacePortResponse, error)
}

func (f *fakeInfraFleetServiceClient) RegisterDevServer(_ context.Context, in *infrafleetv1.RegisterDevServerRequest, _ ...grpc.CallOption) (*infrafleetv1.RegisterDevServerResponse, error) {
	return f.registerDevServerFn(in)
}

func (f *fakeInfraFleetServiceClient) ResolveConnection(_ context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFn(in)
}

func (f *fakeInfraFleetServiceClient) CreateSshTarget(_ context.Context, in *infrafleetv1.CreateSshTargetRequest, _ ...grpc.CallOption) (*infrafleetv1.CreateSshTargetResponse, error) {
	return f.createSshTargetFn(in)
}

func (f *fakeInfraFleetServiceClient) GetFleetHealth(_ context.Context, in *infrafleetv1.GetFleetHealthRequest, _ ...grpc.CallOption) (*infrafleetv1.GetFleetHealthResponse, error) {
	return f.getFleetHealthFn(in)
}

func (f *fakeInfraFleetServiceClient) ScanWorkspacePorts(_ context.Context, in *infrafleetv1.ScanWorkspacePortsRequest, _ ...grpc.CallOption) (*infrafleetv1.ScanWorkspacePortsResponse, error) {
	return f.scanWorkspacePortsFn(in)
}

func (f *fakeInfraFleetServiceClient) ListDevServers(_ context.Context, in *infrafleetv1.ListDevServersRequest, _ ...grpc.CallOption) (*infrafleetv1.ListDevServersResponse, error) {
	return f.listDevServersFn(in)
}

func (f *fakeInfraFleetServiceClient) CreateConnection(_ context.Context, in *infrafleetv1.CreateConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.CreateConnectionResponse, error) {
	return f.createConnectionFn(in)
}

func (f *fakeInfraFleetServiceClient) Relay(_ context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFn(in)
}

func (f *fakeInfraFleetServiceClient) ListSshTargets(_ context.Context, in *infrafleetv1.ListSshTargetsRequest, _ ...grpc.CallOption) (*infrafleetv1.ListSshTargetsResponse, error) {
	return f.listSshTargetsFn(in)
}

func (f *fakeInfraFleetServiceClient) GetSshState(_ context.Context, in *infrafleetv1.GetSshStateRequest, _ ...grpc.CallOption) (*infrafleetv1.GetSshStateResponse, error) {
	return f.getSshStateFn(in)
}

func (f *fakeInfraFleetServiceClient) EstablishConnection(_ context.Context, in *infrafleetv1.EstablishConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.Connection, error) {
	return f.establishConnectionFn(in)
}

func (f *fakeInfraFleetServiceClient) KillWorkspacePort(_ context.Context, in *infrafleetv1.KillWorkspacePortRequest, _ ...grpc.CallOption) (*infrafleetv1.KillWorkspacePortResponse, error) {
	return f.killWorkspacePortFn(in)
}

// Terminal/PTY RPCs (TASK-180..185): no httpgateway route exercises these
// (they're wired through wscompat's channels_terminal.go instead, with its
// own fake client — see channels_terminal_test.go), so these exist only to
// satisfy the infrafleetv1.InfraFleetServiceClient interface this fake must
// implement in full; every one is an unconditional Unimplemented stub.
func (f *fakeInfraFleetServiceClient) SpawnTerminalSession(context.Context, *infrafleetv1.SpawnTerminalSessionRequest, ...grpc.CallOption) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) ResizeTerminalSession(context.Context, *infrafleetv1.ResizeTerminalSessionRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) KillTerminalSession(context.Context, *infrafleetv1.KillTerminalSessionRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) StopTerminalProcess(context.Context, *infrafleetv1.StopTerminalProcessRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) ListTerminalSessions(context.Context, *infrafleetv1.ListTerminalSessionsRequest, ...grpc.CallOption) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) WaitTerminalSession(context.Context, *infrafleetv1.WaitTerminalSessionRequest, ...grpc.CallOption) (*infrafleetv1.WaitTerminalSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) FocusTerminalSession(context.Context, *infrafleetv1.FocusTerminalSessionRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) GetTerminalAgentStatus(context.Context, *infrafleetv1.GetTerminalAgentStatusRequest, ...grpc.CallOption) (*infrafleetv1.GetTerminalAgentStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) InspectTerminalProcess(context.Context, *infrafleetv1.InspectTerminalProcessRequest, ...grpc.CallOption) (*infrafleetv1.InspectTerminalProcessResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) AttachPty(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[infrafleetv1.PtyClientFrame, infrafleetv1.PtyServerFrame], error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

// ListBrowserProfiles/CreateBrowserProfile/DeleteBrowserProfile: no
// httpgateway route exercises these (they're wired through wscompat's
// channels_browser.go instead) — same unconditional Unimplemented-stub
// convention as this file's terminal RPCs above.
func (f *fakeInfraFleetServiceClient) ListBrowserProfiles(context.Context, *infrafleetv1.ListBrowserProfilesRequest, ...grpc.CallOption) (*infrafleetv1.ListBrowserProfilesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) CreateBrowserProfile(context.Context, *infrafleetv1.CreateBrowserProfileRequest, ...grpc.CallOption) (*infrafleetv1.CreateBrowserProfileResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

// ListEmulatorDevices..GetHostCapabilities: no httpgateway route exercises
// these (TASK-048/TASK-070 relay through wscompat's
// channels_emulator_folderworkspace_host.go instead) — same
// unconditional Unimplemented-stub convention as this file's terminal/
// browser-profile RPCs above.
func (f *fakeInfraFleetServiceClient) ListEmulatorDevices(context.Context, *infrafleetv1.ListEmulatorDevicesRequest, ...grpc.CallOption) (*infrafleetv1.ListEmulatorDevicesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) GetEmulatorAvailability(context.Context, *infrafleetv1.GetEmulatorAvailabilityRequest, ...grpc.CallOption) (*infrafleetv1.GetEmulatorAvailabilityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) AttachEmulatorSession(context.Context, *infrafleetv1.AttachEmulatorSessionRequest, ...grpc.CallOption) (*infrafleetv1.EmulatorSession, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) SendEmulatorTap(context.Context, *infrafleetv1.SendEmulatorTapRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) SendEmulatorGesture(context.Context, *infrafleetv1.SendEmulatorGestureRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) SendEmulatorButton(context.Context, *infrafleetv1.SendEmulatorButtonRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) RotateEmulator(context.Context, *infrafleetv1.RotateEmulatorRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) ShutdownEmulator(context.Context, *infrafleetv1.ShutdownEmulatorRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) GetHostCapabilities(context.Context, *infrafleetv1.GetHostCapabilitiesRequest, ...grpc.CallOption) (*infrafleetv1.GetHostCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) DeleteBrowserProfile(context.Context, *infrafleetv1.DeleteBrowserProfileRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) ImportFleetInventory(context.Context, *infrafleetv1.ImportFleetInventoryRequest, ...grpc.CallOption) (*infrafleetv1.ImportFleetInventoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) BulkProvisionFleet(context.Context, *infrafleetv1.BulkProvisionFleetRequest, ...grpc.CallOption) (*infrafleetv1.BulkProvisionFleetResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) DetectDevServerAgents(context.Context, *infrafleetv1.DetectDevServerAgentsRequest, ...grpc.CallOption) (*infrafleetv1.DetectDevServerAgentsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

func (f *fakeInfraFleetServiceClient) CheckDevServerPreflight(context.Context, *infrafleetv1.CheckDevServerPreflightRequest, ...grpc.CallOption) (*infrafleetv1.CheckDevServerPreflightResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by infra_routes_test.go")
}

// testInfraRouter mounts mountInfraRoutes alone on a fresh chi router — no
// authMiddleware, since these tests inject identity into the request
// context directly the way authMiddleware would (withIdentity), mirroring
// how router_test.go exercises auth separately from route behavior.
func testInfraRouter(client infrafleetv1.InfraFleetServiceClient) http.Handler {
	r := chi.NewRouter()
	mountInfraRoutes(r, client)
	return r
}

func newInfraRequest(t *testing.T, method, path string, identity usecase.Identity, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encoding request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(withIdentity(req.Context(), identity))
}

func TestMountInfraRoutes_RegisterDevServer_SuccessRoundTrip(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}

	var gotReq *infrafleetv1.RegisterDevServerRequest
	client := &fakeInfraFleetServiceClient{
		registerDevServerFn: func(req *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			gotReq = req
			return &infrafleetv1.RegisterDevServerResponse{
				DevServer: &infrafleetv1.DevServer{
					Id:       "dev-server-1",
					TenantId: req.GetTenantId(),
					Host:     req.GetHost(),
					Mode:     req.GetMode(),
				},
			}, nil
		},
	}

	router := testInfraRouter(client)

	body := registerDevServerRequestBody{
		Host:        "10.0.0.5",
		Mode:        "relay_ssh",
		SshTargetID: "ssh-target-1",
	}
	req := newInfraRequest(t, http.MethodPost, "/v1/infra/dev-servers", identity, body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if gotReq == nil {
		t.Fatal("client RPC was not called")
	}
	if gotReq.GetHost() != "10.0.0.5" {
		t.Fatalf("Host = %q, want %q", gotReq.GetHost(), "10.0.0.5")
	}
	if gotReq.GetMode() != infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		t.Fatalf("Mode = %v, want CONNECTION_MODE_RELAY_SSH", gotReq.GetMode())
	}
	if gotReq.GetSshTargetId() != "ssh-target-1" {
		t.Fatalf("SshTargetId = %q, want %q", gotReq.GetSshTargetId(), "ssh-target-1")
	}

	var got infrafleetv1.DevServer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if got.Id != "dev-server-1" {
		t.Fatalf("response DevServer.Id = %q, want %q", got.Id, "dev-server-1")
	}
}

// TestMountInfraRoutes_RegisterDevServer_TenantIDComesFromIdentity proves
// tenant_id is always sourced from the validated Identity, never from the
// JSON request body — the request body shape for POST /v1/infra/dev-servers
// (registerDevServerRequestBody) doesn't even have a tenant_id field, so a
// caller attempting to smuggle one in via extra JSON keys must be ignored.
func TestMountInfraRoutes_RegisterDevServer_TenantIDComesFromIdentity(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-from-identity", UserID: "user-1"}

	var gotReq *infrafleetv1.RegisterDevServerRequest
	client := &fakeInfraFleetServiceClient{
		registerDevServerFn: func(req *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error) {
			gotReq = req
			return &infrafleetv1.RegisterDevServerResponse{DevServer: &infrafleetv1.DevServer{Id: "dev-server-1"}}, nil
		},
	}

	router := testInfraRouter(client)

	// Attempt to smuggle a different tenant_id in the raw JSON body — the
	// typed registerDevServerRequestBody has no tenant_id field, so
	// json.Decode silently drops it, but assert the end-to-end behavior
	// anyway: the gRPC request must carry identity's tenant, not this one.
	raw := map[string]string{
		"tenant_id": "attacker-supplied-tenant",
		"host":      "10.0.0.9",
		"mode":      "relay_ssh",
	}
	req := newInfraRequest(t, http.MethodPost, "/v1/infra/dev-servers", identity, raw)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if gotReq == nil {
		t.Fatal("client RPC was not called")
	}
	if gotReq.GetTenantId() != "tenant-from-identity" {
		t.Fatalf("TenantId = %q, want %q (identity's tenant, not the body's)", gotReq.GetTenantId(), "tenant-from-identity")
	}
}

func TestMountInfraRoutes_GetFleetHealth_GRPCErrorMapsToHTTPStatus(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}

	client := &fakeInfraFleetServiceClient{
		getFleetHealthFn: func(*infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "tenant not allowed")
		},
	}

	router := testInfraRouter(client)

	req := newInfraRequest(t, http.MethodGet, "/v1/infra/health", identity, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error body: %v", err)
	}
	if body.Error.Code != codes.PermissionDenied.String() {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, codes.PermissionDenied.String())
	}
	if body.Error.Message != "tenant not allowed" {
		t.Fatalf("error.message = %q, want %q", body.Error.Message, "tenant not allowed")
	}
}

func TestMountInfraRoutes_Relay_SuccessRoundTrip(t *testing.T) {
	identity := usecase.Identity{TenantID: "tenant-1", UserID: "user-1"}

	client := &fakeInfraFleetServiceClient{
		relayFn: func(req *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			if req.GetConnectionId() != "conn-1" {
				t.Fatalf("ConnectionId = %q, want %q", req.GetConnectionId(), "conn-1")
			}
			if req.GetMethod() != "devServer.list" {
				t.Fatalf("Method = %q, want %q", req.GetMethod(), "devServer.list")
			}
			return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
		},
	}

	router := testInfraRouter(client)

	body := relayRequestBody{ConnectionID: "conn-1", Method: "devServer.list", ParamsJSON: "{}"}
	req := newInfraRequest(t, http.MethodPost, "/v1/infra/relay", identity, body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got infrafleetv1.RelayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if got.ResultJson != `{"ok":true}` {
		t.Fatalf("ResultJson = %q, want %q", got.ResultJson, `{"ok":true}`)
	}
}
