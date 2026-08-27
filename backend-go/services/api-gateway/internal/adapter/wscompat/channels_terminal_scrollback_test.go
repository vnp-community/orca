package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeScrollbackInfraFleetClient is this file's own InfraFleetServiceClient
// test double — kept separate from channels_test.go's fakeInfraFleetClient
// and channels_terminal_test.go's fakeTerminalInfraFleetClient, same
// convention as those files, so this test never needs to touch either.
type fakeScrollbackInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	saveFunc func(*infrafleetv1.SaveTerminalScrollbackSnapshotRequest) error
	getFunc  func(*infrafleetv1.GetTerminalScrollbackSnapshotRequest) (*infrafleetv1.GetTerminalScrollbackSnapshotResponse, error)
}

func (f *fakeScrollbackInfraFleetClient) SaveTerminalScrollbackSnapshot(_ context.Context, in *infrafleetv1.SaveTerminalScrollbackSnapshotRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.saveFunc(in)
}

func (f *fakeScrollbackInfraFleetClient) GetTerminalScrollbackSnapshot(_ context.Context, in *infrafleetv1.GetTerminalScrollbackSnapshotRequest, _ ...grpc.CallOption) (*infrafleetv1.GetTerminalScrollbackSnapshotResponse, error) {
	return f.getFunc(in)
}

// TestTerminalScrollbackSave_RoundTripsThroughFakeClient guards
// terminal.scrollback.save's args -> SaveTerminalScrollbackSnapshotRequest
// translation.
func TestTerminalScrollbackSave_RoundTripsThroughFakeClient(t *testing.T) {
	var got *infrafleetv1.SaveTerminalScrollbackSnapshotRequest
	fake := &fakeScrollbackInfraFleetClient{
		saveFunc: func(in *infrafleetv1.SaveTerminalScrollbackSnapshotRequest) error {
			got = in
			return nil
		},
	}
	r := NewRegistry()
	registerTerminalScrollbackChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "terminal.scrollback.save",
		argsJSON(t, scrollbackSaveArgs{
			WorktreeID: "wt-1", PaneKey: "pane-1", Cols: 80, Rows: 24,
			Data: "hello scrollback", LastTitle: "bash",
		}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected SaveTerminalScrollbackSnapshot to have been called")
	}
	if got.GetWorktreeId() != "wt-1" || got.GetPaneKey() != "pane-1" || got.GetCols() != 80 || got.GetRows() != 24 {
		t.Errorf("unexpected SaveTerminalScrollbackSnapshotRequest: %+v", got)
	}
	if string(got.GetData()) != "hello scrollback" || got.GetLastTitle() != "bash" {
		t.Errorf("unexpected SaveTerminalScrollbackSnapshotRequest data/lastTitle: %+v", got)
	}
	view, ok := result.(map[string]bool)
	if !ok || !view["ok"] {
		t.Errorf("expected {ok: true}, got %+v", result)
	}
}

// TestTerminalScrollbackSave_RPCFailurePropagates guards that a save RPC
// error surfaces to the caller (and {ok: false} is still the returned
// shape, matching the ok: err == nil convention).
func TestTerminalScrollbackSave_RPCFailurePropagates(t *testing.T) {
	fake := &fakeScrollbackInfraFleetClient{
		saveFunc: func(*infrafleetv1.SaveTerminalScrollbackSnapshotRequest) error {
			return errors.New("infra-fleet-service: scrollback save failed")
		},
	}
	r := NewRegistry()
	registerTerminalScrollbackChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.scrollback.save",
		argsJSON(t, scrollbackSaveArgs{WorktreeID: "wt-1", PaneKey: "pane-1"}))
	if err == nil {
		t.Fatal("expected the save RPC error to propagate")
	}
	view, ok := result.(map[string]bool)
	if !ok || view["ok"] {
		t.Errorf("expected {ok: false}, got %+v", result)
	}
}

// TestTerminalScrollbackRestore_NeverSaved_ReturnsFoundFalseNotError guards
// the never-saved-pane contract: GetTerminalScrollbackSnapshotResponse's
// Found: false must surface as {found: false}, not an error.
func TestTerminalScrollbackRestore_NeverSaved_ReturnsFoundFalseNotError(t *testing.T) {
	fake := &fakeScrollbackInfraFleetClient{
		getFunc: func(*infrafleetv1.GetTerminalScrollbackSnapshotRequest) (*infrafleetv1.GetTerminalScrollbackSnapshotResponse, error) {
			return &infrafleetv1.GetTerminalScrollbackSnapshotResponse{Found: false}, nil
		},
	}
	r := NewRegistry()
	registerTerminalScrollbackChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.scrollback.restore",
		argsJSON(t, scrollbackRestoreArgs{WorktreeID: "wt-1", PaneKey: "never-saved"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	view, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected a map[string]any result, got %+v", result)
	}
	if found, _ := view["found"].(bool); found {
		t.Errorf("expected found: false, got %+v", view)
	}
}

// TestTerminalScrollbackRestore_RoundTripsThroughFakeClient guards
// terminal.scrollback.restore's found-snapshot response translation.
func TestTerminalScrollbackRestore_RoundTripsThroughFakeClient(t *testing.T) {
	var got *infrafleetv1.GetTerminalScrollbackSnapshotRequest
	fake := &fakeScrollbackInfraFleetClient{
		getFunc: func(in *infrafleetv1.GetTerminalScrollbackSnapshotRequest) (*infrafleetv1.GetTerminalScrollbackSnapshotResponse, error) {
			got = in
			return &infrafleetv1.GetTerminalScrollbackSnapshotResponse{
				Found: true, Cols: 80, Rows: 24, Data: []byte("restored scrollback"),
				LastTitle: "bash", UpdatedAtUnixMs: 1234,
			}, nil
		},
	}
	r := NewRegistry()
	registerTerminalScrollbackChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1"}, "terminal.scrollback.restore",
		argsJSON(t, scrollbackRestoreArgs{WorktreeID: "wt-1", PaneKey: "pane-1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.GetWorktreeId() != "wt-1" || got.GetPaneKey() != "pane-1" {
		t.Errorf("unexpected GetTerminalScrollbackSnapshotRequest: %+v", got)
	}
	view, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected a map[string]any result, got %+v", result)
	}
	if found, _ := view["found"].(bool); !found {
		t.Errorf("expected found: true, got %+v", view)
	}
	if data, _ := view["data"].(string); data != "restored scrollback" {
		t.Errorf("expected data to round-trip, got %+v", view)
	}
}
