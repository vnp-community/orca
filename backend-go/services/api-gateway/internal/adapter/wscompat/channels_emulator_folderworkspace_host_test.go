package wscompat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeEmulatorHostClient is a minimal test double scoped to the emulator.*/
// host.* RPCs this file's channel handlers call (TASK-048/TASK-070).
type fakeEmulatorHostClient struct {
	infrafleetv1.InfraFleetServiceClient

	listEmulatorDevicesFunc     func(ctx context.Context, in *infrafleetv1.ListEmulatorDevicesRequest) (*infrafleetv1.ListEmulatorDevicesResponse, error)
	getEmulatorAvailabilityFunc func(ctx context.Context, in *infrafleetv1.GetEmulatorAvailabilityRequest) (*infrafleetv1.GetEmulatorAvailabilityResponse, error)
	attachEmulatorSessionFunc   func(ctx context.Context, in *infrafleetv1.AttachEmulatorSessionRequest) (*infrafleetv1.EmulatorSession, error)
	getHostCapabilitiesFunc     func(ctx context.Context, in *infrafleetv1.GetHostCapabilitiesRequest) (*infrafleetv1.GetHostCapabilitiesResponse, error)

	lastConnectionID string
}

func (f *fakeEmulatorHostClient) ListEmulatorDevices(ctx context.Context, in *infrafleetv1.ListEmulatorDevicesRequest, _ ...grpc.CallOption) (*infrafleetv1.ListEmulatorDevicesResponse, error) {
	f.lastConnectionID = in.GetConnectionId()
	return f.listEmulatorDevicesFunc(ctx, in)
}

func (f *fakeEmulatorHostClient) GetEmulatorAvailability(ctx context.Context, in *infrafleetv1.GetEmulatorAvailabilityRequest, _ ...grpc.CallOption) (*infrafleetv1.GetEmulatorAvailabilityResponse, error) {
	f.lastConnectionID = in.GetConnectionId()
	return f.getEmulatorAvailabilityFunc(ctx, in)
}

func (f *fakeEmulatorHostClient) AttachEmulatorSession(ctx context.Context, in *infrafleetv1.AttachEmulatorSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.EmulatorSession, error) {
	f.lastConnectionID = in.GetConnectionId()
	return f.attachEmulatorSessionFunc(ctx, in)
}

func (f *fakeEmulatorHostClient) GetHostCapabilities(ctx context.Context, in *infrafleetv1.GetHostCapabilitiesRequest, _ ...grpc.CallOption) (*infrafleetv1.GetHostCapabilitiesResponse, error) {
	f.lastConnectionID = in.GetConnectionId()
	return f.getHostCapabilitiesFunc(ctx, in)
}

// ── emulator.* (TASK-046/TASK-048) ────────────────────────────────────────

func TestRegisterEmulatorChannels_NoConnectionID_ReturnsHonestNotSupportedError(t *testing.T) {
	r := NewRegistry()
	registerEmulatorChannels(r, &fakeEmulatorHostClient{})

	channels := []string{
		"emulator.attach", "emulator.button",
		"emulator.gesture", "emulator.listDevices", "emulator.rotate",
		"emulator.shutdown", "emulator.tap",
	}

	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, channel, nil)
			if result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if !errors.Is(err, errEmulatorNotSupported) {
				t.Errorf("expected errEmulatorNotSupported, got %v", err)
			}
			// Regression guard: must not fall through to
			// notImplementedHandler's generic "not yet implemented" message.
			if err != nil && strings.Contains(err.Error(), "is not yet implemented in backend-go") {
				t.Errorf("channel %q fell through to notImplementedHandler, want a registered honest-stub handler", channel)
			}
		})
	}
}

// TASK-048 regression test: a connectionId present in the request must
// prefer the relay path — proving RegisterRealChannels' "relay when
// connection_id is present" rule for a namespace with NO local fallback
// (unlike host.* below).
func TestRegisterEmulatorChannels_WithConnectionID_Relays(t *testing.T) {
	fake := &fakeEmulatorHostClient{
		listEmulatorDevicesFunc: func(ctx context.Context, in *infrafleetv1.ListEmulatorDevicesRequest) (*infrafleetv1.ListEmulatorDevicesResponse, error) {
			return &infrafleetv1.ListEmulatorDevicesResponse{Devices: []*infrafleetv1.EmulatorDevice{{Id: "emulator-5554"}}}, nil
		},
	}
	r := NewRegistry()
	registerEmulatorChannels(r, fake)

	args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "emulator.listDevices", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastConnectionID != "conn-1" {
		t.Errorf("expected connectionId to be forwarded, got %q", fake.lastConnectionID)
	}
	devices, ok := result.([]*infrafleetv1.EmulatorDevice)
	if !ok || len(devices) != 1 || devices[0].GetId() != "emulator-5554" {
		t.Errorf("unexpected result: %#v", result)
	}
}

// GetEmulatorAvailability has no connectionId requirement even in
// wscompat, unlike every other emulator.* channel — see
// registerEmulatorChannels's doc comment.
func TestRegisterEmulatorChannels_Availability_AlwaysRelaysEvenWithoutConnectionID(t *testing.T) {
	fake := &fakeEmulatorHostClient{
		getEmulatorAvailabilityFunc: func(ctx context.Context, in *infrafleetv1.GetEmulatorAvailabilityRequest) (*infrafleetv1.GetEmulatorAvailabilityResponse, error) {
			return &infrafleetv1.GetEmulatorAvailabilityResponse{Available: false, Reason: "no active dev server connection"}, nil
		},
	}
	r := NewRegistry()
	registerEmulatorChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "emulator.availability", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok || m["available"] != false {
		t.Errorf("unexpected result: %#v", result)
	}
}

// ── host.* (TASK-068/TASK-070) ──────────────────────────────────────────────

func TestRegisterHostChannels_NoConnectionID_HonestLocalAnswers(t *testing.T) {
	r := NewRegistry()
	registerHostChannels(r, &fakeEmulatorHostClient{})

	tests := []struct {
		channel string
		want    any
	}{
		{"host.wsl.isAvailable", map[string]bool{"available": false}},
		{"host.wsl.listDistros", []string{}},
		{"host.pwsh.isAvailable", map[string]bool{"available": false}},
		{"host.gitBash.isAvailable", map[string]bool{"available": false}},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, tt.channel, nil)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			switch want := tt.want.(type) {
			case map[string]bool:
				got, ok := result.(map[string]bool)
				if !ok || got["available"] != want["available"] {
					t.Errorf("channel %q: got %#v, want %#v", tt.channel, result, want)
				}
			case []string:
				got, ok := result.([]string)
				if !ok || len(got) != len(want) {
					t.Errorf("channel %q: got %#v, want %#v", tt.channel, result, want)
				}
			}
		})
	}
}

// TASK-070 regression test: a connectionId present in the request must
// prefer the relay path.
func TestRegisterHostChannels_WithConnectionID_Relays(t *testing.T) {
	fake := &fakeEmulatorHostClient{
		getHostCapabilitiesFunc: func(ctx context.Context, in *infrafleetv1.GetHostCapabilitiesRequest) (*infrafleetv1.GetHostCapabilitiesResponse, error) {
			return &infrafleetv1.GetHostCapabilitiesResponse{
				WslAvailable: true, WslDistros: []string{"Ubuntu"}, PwshAvailable: true, GitBashAvailable: false,
			}, nil
		},
	}
	r := NewRegistry()
	registerHostChannels(r, fake)

	args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "host.wsl.isAvailable", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastConnectionID != "conn-1" {
		t.Errorf("expected connectionId to be forwarded, got %q", fake.lastConnectionID)
	}
	m, ok := result.(map[string]bool)
	if !ok || !m["available"] {
		t.Errorf("unexpected result: %#v", result)
	}

	distrosArgs := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	distrosResult, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "host.wsl.listDistros", distrosArgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	distros, ok := distrosResult.([]string)
	if !ok || len(distros) != 1 || distros[0] != "Ubuntu" {
		t.Errorf("unexpected result: %#v", distrosResult)
	}
}

// A real "method not found" translated into apperrors.KindFailedPrecondition
// by infra-fleet-service surfaces here as an ordinary gRPC error — this file
// deliberately does not add a second layer of translation on top.
func TestRegisterHostChannels_RelayError_Propagates(t *testing.T) {
	wantErr := errors.New("rpc error: code = FailedPrecondition desc = INFRA_HOST_CAPABILITIES_UNSUPPORTED")
	fake := &fakeEmulatorHostClient{
		getHostCapabilitiesFunc: func(ctx context.Context, in *infrafleetv1.GetHostCapabilitiesRequest) (*infrafleetv1.GetHostCapabilitiesResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerHostChannels(r, fake)

	args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "host.wsl.isAvailable", args)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want error %v, got %v", wantErr, err)
	}
}

// ── folderWorkspace.* (TASK-067's wscompat-channel portion) ────────────────

// fakeProjectClient is a minimal test double for
// projectv1.ProjectServiceClient — embeds the (nil) interface so it
// satisfies every method, and overrides only the five this file's channel
// handlers actually call.
type fakeProjectClient struct {
	projectv1.ProjectServiceClient

	createFolderWorkspaceFunc      func(ctx context.Context, in *projectv1.CreateFolderWorkspaceRequest) (*projectv1.CreateFolderWorkspaceResponse, error)
	updateFolderWorkspaceFunc      func(ctx context.Context, in *projectv1.UpdateFolderWorkspaceRequest) (*projectv1.UpdateFolderWorkspaceResponse, error)
	deleteFolderWorkspaceFunc      func(ctx context.Context, in *projectv1.DeleteFolderWorkspaceRequest) (*projectv1.DeleteFolderWorkspaceResponse, error)
	listFolderWorkspacesFunc       func(ctx context.Context, in *projectv1.ListFolderWorkspacesRequest) (*projectv1.ListFolderWorkspacesResponse, error)
	getFolderWorkspacePathStatFunc func(ctx context.Context, in *projectv1.GetFolderWorkspacePathStatusRequest) (*projectv1.GetFolderWorkspacePathStatusResponse, error)

	lastCreateReq *projectv1.CreateFolderWorkspaceRequest
	lastUpdateReq *projectv1.UpdateFolderWorkspaceRequest
	lastDeleteReq *projectv1.DeleteFolderWorkspaceRequest
	lastStatusReq *projectv1.GetFolderWorkspacePathStatusRequest
	callCount     int
}

func (f *fakeProjectClient) CreateFolderWorkspace(ctx context.Context, in *projectv1.CreateFolderWorkspaceRequest, _ ...grpc.CallOption) (*projectv1.CreateFolderWorkspaceResponse, error) {
	f.callCount++
	f.lastCreateReq = in
	return f.createFolderWorkspaceFunc(ctx, in)
}

func (f *fakeProjectClient) UpdateFolderWorkspace(ctx context.Context, in *projectv1.UpdateFolderWorkspaceRequest, _ ...grpc.CallOption) (*projectv1.UpdateFolderWorkspaceResponse, error) {
	f.callCount++
	f.lastUpdateReq = in
	return f.updateFolderWorkspaceFunc(ctx, in)
}

func (f *fakeProjectClient) DeleteFolderWorkspace(ctx context.Context, in *projectv1.DeleteFolderWorkspaceRequest, _ ...grpc.CallOption) (*projectv1.DeleteFolderWorkspaceResponse, error) {
	f.callCount++
	f.lastDeleteReq = in
	return f.deleteFolderWorkspaceFunc(ctx, in)
}

func (f *fakeProjectClient) ListFolderWorkspaces(ctx context.Context, in *projectv1.ListFolderWorkspacesRequest, _ ...grpc.CallOption) (*projectv1.ListFolderWorkspacesResponse, error) {
	f.callCount++
	return f.listFolderWorkspacesFunc(ctx, in)
}

func (f *fakeProjectClient) GetFolderWorkspacePathStatus(ctx context.Context, in *projectv1.GetFolderWorkspacePathStatusRequest, _ ...grpc.CallOption) (*projectv1.GetFolderWorkspacePathStatusResponse, error) {
	f.callCount++
	f.lastStatusReq = in
	return f.getFolderWorkspacePathStatFunc(ctx, in)
}

func TestFolderWorkspaceCreateChannel_Success(t *testing.T) {
	fake := &fakeProjectClient{
		createFolderWorkspaceFunc: func(ctx context.Context, in *projectv1.CreateFolderWorkspaceRequest) (*projectv1.CreateFolderWorkspaceResponse, error) {
			return &projectv1.CreateFolderWorkspaceResponse{
				FolderWorkspace: &projectv1.FolderWorkspace{
					Id: "fw-1", DevServerId: in.DevServerId, Path: in.Path, Name: in.Name, ProjectGroupId: in.ProjectGroupId,
				},
			}, nil
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	// Why projectGroupId is asserted here: migration 0012 added real
	// persistence for it — this channel used to silently drop it (no field
	// in the decode struct at all) before that.
	args := argsJSON(t, map[string]any{
		"devServerId": "d1", "path": "/home/x", "name": "x", "projectGroupId": "group-1",
	})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "folderWorkspace.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.callCount != 1 {
		t.Fatalf("expected exactly 1 call, got %d", fake.callCount)
	}
	if fake.lastCreateReq.DevServerId != "d1" || fake.lastCreateReq.Path != "/home/x" || fake.lastCreateReq.Name != "x" ||
		fake.lastCreateReq.ProjectGroupId != "group-1" {
		t.Errorf("unexpected request fields: %+v", fake.lastCreateReq)
	}
	fw, ok := result.(folderWorkspaceView)
	if !ok || fw.ID != "fw-1" || fw.ProjectGroupID != "group-1" {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestFolderWorkspaceUpdateChannel_Success(t *testing.T) {
	fake := &fakeProjectClient{
		updateFolderWorkspaceFunc: func(ctx context.Context, in *projectv1.UpdateFolderWorkspaceRequest) (*projectv1.UpdateFolderWorkspaceResponse, error) {
			return &projectv1.UpdateFolderWorkspaceResponse{FolderWorkspace: &projectv1.FolderWorkspace{Id: in.Id, Name: in.Name}}, nil
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "fw-1", "name": "renamed"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "folderWorkspace.update", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.callCount != 1 || fake.lastUpdateReq.Id != "fw-1" || fake.lastUpdateReq.Name != "renamed" {
		t.Errorf("unexpected request: %+v (calls=%d)", fake.lastUpdateReq, fake.callCount)
	}
	fw, ok := result.(folderWorkspaceView)
	if !ok || fw.Name != "renamed" {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestFolderWorkspaceDeleteChannel_Success(t *testing.T) {
	fake := &fakeProjectClient{
		deleteFolderWorkspaceFunc: func(ctx context.Context, in *projectv1.DeleteFolderWorkspaceRequest) (*projectv1.DeleteFolderWorkspaceResponse, error) {
			return &projectv1.DeleteFolderWorkspaceResponse{}, nil
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "fw-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "folderWorkspace.delete", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.callCount != 1 || fake.lastDeleteReq.Id != "fw-1" {
		t.Errorf("unexpected request: %+v (calls=%d)", fake.lastDeleteReq, fake.callCount)
	}
	ok, _ := result.(map[string]bool)
	if !ok["ok"] {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestFolderWorkspaceListChannel_Success(t *testing.T) {
	fake := &fakeProjectClient{
		listFolderWorkspacesFunc: func(ctx context.Context, in *projectv1.ListFolderWorkspacesRequest) (*projectv1.ListFolderWorkspacesResponse, error) {
			return &projectv1.ListFolderWorkspacesResponse{
				FolderWorkspaces: []*projectv1.FolderWorkspace{{Id: "fw-1"}, {Id: "fw-2"}},
			}, nil
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "folderWorkspace.list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.callCount != 1 {
		t.Errorf("expected exactly 1 call, got %d", fake.callCount)
	}
	resp, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", result)
	}
	list, ok := resp["folderWorkspaces"].([]folderWorkspaceView)
	if !ok || len(list) != 2 {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestFolderWorkspaceGetPathStatusChannel_Success(t *testing.T) {
	fake := &fakeProjectClient{
		getFolderWorkspacePathStatFunc: func(ctx context.Context, in *projectv1.GetFolderWorkspacePathStatusRequest) (*projectv1.GetFolderWorkspacePathStatusResponse, error) {
			return &projectv1.GetFolderWorkspacePathStatusResponse{Status: "PATH_STATUS_AVAILABLE"}, nil
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "d1", "path": "/home/new"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "folderWorkspace.getPathStatus", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.callCount != 1 || fake.lastStatusReq.DevServerId != "d1" || fake.lastStatusReq.Path != "/home/new" {
		t.Errorf("unexpected request: %+v (calls=%d)", fake.lastStatusReq, fake.callCount)
	}
	resp, ok := result.(map[string]any)
	if !ok || resp["status"] != "PATH_STATUS_AVAILABLE" {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestFolderWorkspaceCreateChannel_PropagatesError(t *testing.T) {
	wantErr := errors.New("project-service unavailable")
	fake := &fakeProjectClient{
		createFolderWorkspaceFunc: func(ctx context.Context, in *projectv1.CreateFolderWorkspaceRequest) (*projectv1.CreateFolderWorkspaceResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "d1", "path": "/home/x"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "folderWorkspace.create", args)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want error %v, got %v", wantErr, err)
	}
}
