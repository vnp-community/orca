package wscompat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// ── emulator.* (TASK-047) ─────────────────────────────────────────────────

func TestRegisterEmulatorChannels_AllReturnHonestNotSupportedError(t *testing.T) {
	r := NewRegistry()
	registerEmulatorChannels(r)

	channels := []string{
		"emulator.attach", "emulator.availability", "emulator.button",
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

// ── host.* (TASK-069) ──────────────────────────────────────────────────────

func TestRegisterHostChannels_HonestLocalAnswers(t *testing.T) {
	r := NewRegistry()
	registerHostChannels(r)

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
				FolderWorkspace: &projectv1.FolderWorkspace{Id: "fw-1", DevServerId: in.DevServerId, Path: in.Path, Name: in.Name},
			}, nil
		},
	}
	r := NewRegistry()
	registerFolderWorkspaceChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "d1", "path": "/home/x", "name": "x"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "folderWorkspace.create", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.callCount != 1 {
		t.Fatalf("expected exactly 1 call, got %d", fake.callCount)
	}
	if fake.lastCreateReq.DevServerId != "d1" || fake.lastCreateReq.Path != "/home/x" || fake.lastCreateReq.Name != "x" {
		t.Errorf("unexpected request fields: %+v", fake.lastCreateReq)
	}
	fw, ok := result.(*projectv1.FolderWorkspace)
	if !ok || fw.GetId() != "fw-1" {
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
	fw, ok := result.(*projectv1.FolderWorkspace)
	if !ok || fw.GetName() != "renamed" {
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
	list, ok := result.([]*projectv1.FolderWorkspace)
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
	resp, ok := result.(*projectv1.GetFolderWorkspacePathStatusResponse)
	if !ok || resp.GetStatus() != "PATH_STATUS_AVAILABLE" {
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
