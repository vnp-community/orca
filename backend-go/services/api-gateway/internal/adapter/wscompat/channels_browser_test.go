package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeBrowserRelayClient is scoped to ResolveConnection + Relay — the only
// 2 RPCs browser.* pane-control channels call.
type fakeBrowserRelayClient struct {
	infrafleetv1.InfraFleetServiceClient

	resolveConnectionFunc func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
	relayFunc             func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeBrowserRelayClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFunc(ctx, in)
}

func (f *fakeBrowserRelayClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}

func TestBrowserChannels_RequiresWorktree_FailsFastWithoutResolving(t *testing.T) {
	called := false
	fake := &fakeBrowserRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerBrowserChannels(r, fake)

	args := argsJSON(t, map[string]any{"x": 10, "y": 20}) // no worktree
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.mouseMove", args)
	if err == nil {
		t.Fatal("expected BROWSER_NO_WORKTREE error")
	}
	if called {
		t.Error("ResolveConnection must not be called when worktree is missing")
	}
}

func TestBrowserChannels_ResolvesWorktreeThenRelaysFullParams(t *testing.T) {
	var gotResolve *infrafleetv1.ResolveConnectionRequest
	var gotRelay *infrafleetv1.RelayRequest
	fake := &fakeBrowserRelayClient{
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			gotResolve = in
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, DevServer: &infrafleetv1.DevServer{Id: "ds-1"}, ConnectionId: "conn-1"}, nil
		},
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			gotRelay = in
			return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
		},
	}
	r := NewRegistry()
	registerBrowserChannels(r, fake)

	args := argsJSON(t, map[string]any{"worktree": "wt-1", "width": 1280, "height": 720})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.viewport", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResolve.GetWorktreeId() != "wt-1" {
		t.Errorf("WorktreeId = %q, want wt-1", gotResolve.GetWorktreeId())
	}
	if gotRelay.GetMethod() != "browser.viewport" {
		t.Errorf("Method = %q, want browser.viewport", gotRelay.GetMethod())
	}
	// The Relay call must be keyed by the resolved connectionId
	// (infra.connections.id, TASK-025) — NOT the dev server's own id, a
	// different id space.
	if gotRelay.GetConnectionId() != "conn-1" {
		t.Errorf("ConnectionId = %q, want conn-1", gotRelay.GetConnectionId())
	}
	var params map[string]any
	_ = json.Unmarshal([]byte(gotRelay.GetParamsJson()), &params)
	if params["width"] != float64(1280) {
		t.Errorf("params passed through incompletely: %#v", params)
	}
	if result.(map[string]any)["ok"] != true {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestBrowserChannels_NotConnected_Errors(t *testing.T) {
	fake := &fakeBrowserRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: false}, nil
		},
	}
	r := NewRegistry()
	registerBrowserChannels(r, fake)

	args := argsJSON(t, map[string]any{"worktree": "wt-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.tabCreate", args)
	if err == nil {
		t.Fatal("expected BROWSER_NO_CONNECTION error")
	}
}

func TestBrowserChannels_AllGroupAAndBChannels_ResolveThenRelay(t *testing.T) {
	channels := []string{
		// goto/snapshot/click: TASK-036 option b's additive ops beyond
		// SOL-006's original 9 — real headless-browser navigate/inspect/
		// interact, implemented on the agent (browser-handler.ts). Same
		// resolve-then-relay skeleton, so covered by this one table.
		"browser.goto", "browser.snapshot", "browser.click",
		"browser.eval", "browser.keypress", "browser.mouseDown", "browser.mouseMove",
		"browser.mouseUp", "browser.mouseWheel", "browser.viewport", "browser.tabCreate", "browser.tabClose",
	}
	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			var gotRelay *infrafleetv1.RelayRequest
			fake := &fakeBrowserRelayClient{
				resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
					return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "conn-1"}, nil
				},
				relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
					gotRelay = in
					return &infrafleetv1.RelayResponse{ResultJson: `{}`}, nil
				},
			}
			r := NewRegistry()
			registerBrowserChannels(r, fake)

			args := argsJSON(t, map[string]any{"worktree": "wt-1"})
			if _, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel, args); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotRelay == nil {
				t.Fatal("expected Relay to be called")
			}
			if gotRelay.GetMethod() != channel {
				t.Errorf("Method = %q, want %q", gotRelay.GetMethod(), channel)
			}
			if gotRelay.GetConnectionId() != "conn-1" {
				t.Errorf("ConnectionId = %q, want conn-1", gotRelay.GetConnectionId())
			}
		})
	}
}
