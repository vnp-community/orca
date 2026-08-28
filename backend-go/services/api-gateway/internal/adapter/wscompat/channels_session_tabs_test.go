package wscompat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeSessionTabsInfraFleetClient is a minimal test double — embeds the
// (nil) interface, per fakeInfraFleetClient's precedent in channels_test.go,
// and overrides only the two methods this file's channel handlers call.
type fakeSessionTabsInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	listTerminalSessionsFunc func(ctx context.Context, in *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error)
	resolveConnectionFunc    func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
}

func (f *fakeSessionTabsInfraFleetClient) ListTerminalSessions(ctx context.Context, in *infrafleetv1.ListTerminalSessionsRequest, _ ...grpc.CallOption) (*infrafleetv1.ListTerminalSessionsResponse, error) {
	return f.listTerminalSessionsFunc(ctx, in)
}

func (f *fakeSessionTabsInfraFleetClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFunc(ctx, in)
}

func TestSessionTabsListAllChannel_GroupsSessionsByResolvedWorktree(t *testing.T) {
	fake := &fakeSessionTabsInfraFleetClient{
		listTerminalSessionsFunc: func(_ context.Context, in *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			if in.GetConnectionId() != "" {
				t.Errorf("expected an empty connection_id (all sessions for tenant), got %q", in.GetConnectionId())
			}
			return &infrafleetv1.ListTerminalSessionsResponse{
				Sessions: []*infrafleetv1.TerminalSession{
					{PtyId: "pty-1", ConnectionId: "conn-1", Cwd: "/repo-a"},
					{PtyId: "pty-2", ConnectionId: "conn-1", Cwd: "/repo-a"},
					{PtyId: "pty-3", ConnectionId: "conn-2", Cwd: "/repo-b"},
				},
			}, nil
		},
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			switch in.GetConnectionId() {
			case "conn-1":
				return &infrafleetv1.ResolveConnectionResponse{WorktreeId: "wt-a"}, nil
			case "conn-2":
				return &infrafleetv1.ResolveConnectionResponse{WorktreeId: "wt-b"}, nil
			}
			t.Fatalf("unexpected ResolveConnection call for %q", in.GetConnectionId())
			return nil, nil
		},
	}
	r := NewRegistry()
	registerSessionTabsChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "session.tabs.listAll", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected a map[string]any result, got %T", result)
	}
	snapshots, ok := out["snapshots"].([]sessionTabsSnapshot)
	if !ok {
		t.Fatalf("expected snapshots to be []sessionTabsSnapshot, got %T", out["snapshots"])
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 worktree snapshots, got %+v", snapshots)
	}
	if snapshots[0].WorktreeID != "wt-a" || len(snapshots[0].Tabs) != 2 {
		t.Errorf("expected wt-a with 2 tabs, got %+v", snapshots[0])
	}
	if snapshots[1].WorktreeID != "wt-b" || len(snapshots[1].Tabs) != 1 {
		t.Errorf("expected wt-b with 1 tab, got %+v", snapshots[1])
	}
}

func TestSessionTabsListAllChannel_DropsSessionsWithNoResolvedWorktree(t *testing.T) {
	fake := &fakeSessionTabsInfraFleetClient{
		listTerminalSessionsFunc: func(_ context.Context, _ *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			return &infrafleetv1.ListTerminalSessionsResponse{
				Sessions: []*infrafleetv1.TerminalSession{{PtyId: "pty-1", ConnectionId: "conn-local"}},
			}, nil
		},
		resolveConnectionFunc: func(_ context.Context, _ *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{WorktreeId: ""}, nil // local/non-worktree execution
		},
	}
	r := NewRegistry()
	registerSessionTabsChannels(r, fake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1"}, "session.tabs.listAll", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	snapshots := result.(map[string]any)["snapshots"].([]sessionTabsSnapshot)
	if len(snapshots) != 0 {
		t.Errorf("expected sessions with no resolved worktree to be dropped, got %+v", snapshots)
	}
}

func TestSessionTabsSubscribeAllChannel_EmitsSnapshotsThenUpdatedOnChange(t *testing.T) {
	orig := sessionTabsSubscribePollInterval
	sessionTabsSubscribePollInterval = 5 * time.Millisecond
	t.Cleanup(func() { sessionTabsSubscribePollInterval = orig })

	var call atomic.Int32
	fake := &fakeSessionTabsInfraFleetClient{
		listTerminalSessionsFunc: func(_ context.Context, _ *infrafleetv1.ListTerminalSessionsRequest) (*infrafleetv1.ListTerminalSessionsResponse, error) {
			n := call.Add(1)
			// First 3 calls (initial fetch + first 2 poll ticks) return ONE
			// session — must not produce a second push. The 4th call adds a
			// second session on the same worktree — must produce exactly one
			// "updated" push.
			sessions := []*infrafleetv1.TerminalSession{{PtyId: "pty-1", ConnectionId: "conn-1"}}
			if n >= 4 {
				sessions = append(sessions, &infrafleetv1.TerminalSession{PtyId: "pty-2", ConnectionId: "conn-1"})
			}
			return &infrafleetv1.ListTerminalSessionsResponse{Sessions: sessions}, nil
		},
		resolveConnectionFunc: func(_ context.Context, _ *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{WorktreeId: "wt-a"}, nil
		},
	}
	r := NewRegistry()
	registerSessionTabsChannels(r, fake)

	sh, _ := r.StreamHandlerFor("session.tabs.subscribeAll")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := sh(ctx, Identity{TenantID: "tenant-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first := readSessionTabsEvent(t, events)
	if first["type"] != "snapshots" {
		t.Fatalf("first event type = %v, want snapshots", first["type"])
	}

	update := readSessionTabsEvent(t, events)
	if update["type"] != "updated" {
		t.Fatalf("second event type = %v, want updated", update["type"])
	}
	if update["worktreeId"] != "wt-a" {
		t.Fatalf("update worktreeId = %v, want wt-a", update["worktreeId"])
	}
	tabs, ok := update["tabs"].([]terminalSessionView)
	if !ok || len(tabs) != 2 {
		t.Fatalf("expected 2 tabs in the updated snapshot (unchanged ticks must not have pushed), got %#v", update["tabs"])
	}
}

func readSessionTabsEvent(t *testing.T, events <-chan PushEvent) map[string]any {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("events channel closed before expected event arrived")
		}
		if ev.Channel != "session.tabs.event" || len(ev.Args) != 1 {
			t.Fatalf("unexpected PushEvent shape: %+v", ev)
		}
		payload, ok := ev.Args[0].(map[string]any)
		if !ok {
			t.Fatalf("event payload is not a map[string]any: %#v", ev.Args[0])
		}
		return payload
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a session.tabs event")
	}
	return nil
}
