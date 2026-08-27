package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeAccountsRelayClient is a minimal test double scoped to Relay and
// ResolveConnection — the only 2 RPCs accounts.* channel handlers call.
type fakeAccountsRelayClient struct {
	infrafleetv1.InfraFleetServiceClient

	relayFunc             func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
	resolveConnectionFunc func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
}

func (f *fakeAccountsRelayClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}

func (f *fakeAccountsRelayClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFunc(ctx, in)
}

func TestAccountsChannels_RelaySuccess(t *testing.T) {
	cases := []struct {
		channel    string
		wantMethod string
	}{
		{"accounts.selectClaude", "accounts.selectClaude"},
		{"accounts.selectCodex", "accounts.selectCodex"},
		{"accounts.removeClaude", "accounts.removeClaude"},
		{"accounts.removeCodex", "accounts.removeCodex"},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			var gotReq *infrafleetv1.RelayRequest
			fake := &fakeAccountsRelayClient{
				relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
					gotReq = in
					return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
				},
			}
			r := NewRegistry()
			registerAccountsChannels(r, fake)

			args := argsJSON(t, map[string]any{"accountId": "acct-1", "connectionId": "conn-1"})
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, tc.channel, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := result.(map[string]any)
			if !ok || got["ok"] != true {
				t.Fatalf("unexpected result: %#v", result)
			}
			if gotReq.GetConnectionId() != "conn-1" {
				t.Errorf("ConnectionId = %q, want conn-1", gotReq.GetConnectionId())
			}
			if gotReq.GetMethod() != tc.wantMethod {
				t.Errorf("Method = %q, want %q", gotReq.GetMethod(), tc.wantMethod)
			}
			var params map[string]any
			if err := json.Unmarshal([]byte(gotReq.GetParamsJson()), &params); err != nil {
				t.Fatalf("params_json not valid JSON: %v", err)
			}
			if params["accountId"] != "acct-1" {
				t.Errorf("params accountId = %v, want acct-1", params["accountId"])
			}
		})
	}
}

func TestAccountsChannels_MissingConnectionID_FailsFastWithoutCallingRelay(t *testing.T) {
	called := false
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"accountId": "acct-1"}) // no connectionId
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.selectClaude", args)
	if err == nil {
		t.Fatal("expected an error for missing connectionId")
	}
	if called {
		t.Error("Relay must not be called when connectionId is missing")
	}
}

func TestAccountsChannels_RelayErrorPassesThroughVerbatim(t *testing.T) {
	wantErr := errors.New("INFRA_CONNECTION_NOT_FOUND: no dev server owns this connectionId")
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"accountId": "acct-1", "connectionId": "conn-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.removeCodex", args)
	if !errors.Is(err, wantErr) && err.Error() != wantErr.Error() {
		t.Fatalf("expected Relay's error to pass through verbatim, got: %v", err)
	}
}

func TestAccountsSubscribe_MissingConnectionID_FailsFastWithoutCallingRelay(t *testing.T) {
	called := false
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	sh, ok := r.StreamHandlerFor("accounts.subscribe")
	if !ok {
		t.Fatal("accounts.subscribe not registered as a StreamHandler")
	}
	// No args at all — matches the real frontend call site
	// (watchProviderAccounts never sends a params object for this method).
	_, err := sh(context.Background(), Identity{TenantID: "t1"}, nil)
	if err == nil {
		t.Fatal("expected an error for missing connectionId")
	}
	if called {
		t.Error("Relay must not be called when connectionId is missing")
	}
}

func TestAccountsSubscribe_RelayErrorOnFirstFetchFailsSubscribeSynchronously(t *testing.T) {
	wantErr := errors.New("INFRA_CONNECTION_NOT_FOUND: no dev server owns this connectionId")
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	sh, _ := r.StreamHandlerFor("accounts.subscribe")
	args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	_, err := sh(context.Background(), Identity{TenantID: "t1"}, args)
	if !errors.Is(err, wantErr) && (err == nil || err.Error() != wantErr.Error()) {
		t.Fatalf("expected Relay's first-fetch error to fail the subscribe call synchronously, got: %v", err)
	}
}

func TestAccountsSubscribe_EmitsReadyThenSnapshotOnChangeOnly(t *testing.T) {
	orig := accountsSubscribePollInterval
	accountsSubscribePollInterval = 5 * time.Millisecond
	t.Cleanup(func() { accountsSubscribePollInterval = orig })

	var call atomic.Int32
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			n := call.Add(1)
			// First 3 calls (initial fetch + first 2 poll ticks) return the SAME
			// snapshot — must NOT produce a second push. The 4th call changes
			// activeAccountId — must produce exactly one "snapshot" push.
			active := "acct-1"
			if n >= 4 {
				active = "acct-2"
			}
			return &infrafleetv1.RelayResponse{ResultJson: `{"claude":{"activeAccountId":"` + active + `"}}`}, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	sh, _ := r.StreamHandlerFor("accounts.subscribe")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	events, err := sh(ctx, Identity{TenantID: "t1"}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ready := readAccountsEvent(t, events)
	if ready["type"] != "ready" {
		t.Fatalf("first event type = %v, want ready", ready["type"])
	}

	update := readAccountsEvent(t, events)
	if update["type"] != "snapshot" {
		t.Fatalf("second event type = %v, want snapshot", update["type"])
	}
	snapshot, _ := update["snapshot"].(map[string]any)
	claude, _ := snapshot["claude"].(map[string]any)
	if claude["activeAccountId"] != "acct-2" {
		t.Fatalf("snapshot activeAccountId = %v, want acct-2 (unchanged ticks must not have pushed)", claude["activeAccountId"])
	}
}

func TestAccountsResolveDevServerConnection_Connected(t *testing.T) {
	var gotReq *infrafleetv1.ResolveConnectionRequest
	fake := &fakeAccountsRelayClient{
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			gotReq = in
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "conn-1"}, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "accounts.resolveDevServerConnection", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result.(accountsResolveDevServerConnectionResult)
	if !ok {
		t.Fatalf("unexpected result type: %#v", result)
	}
	if !got.Connected || got.ConnectionID != "conn-1" {
		t.Errorf("result = %+v, want Connected=true ConnectionID=conn-1", got)
	}
	if gotReq.GetDevServerId() != "ds-1" {
		t.Errorf("DevServerId = %q, want ds-1", gotReq.GetDevServerId())
	}
}

func TestAccountsResolveDevServerConnection_NotConnected_NotAnError(t *testing.T) {
	fake := &fakeAccountsRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: false}, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds-offline"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.resolveDevServerConnection", args)
	if err != nil {
		t.Fatalf("a disconnected dev server must be a displayable result, not an error: %v", err)
	}
	got, ok := result.(accountsResolveDevServerConnectionResult)
	if !ok {
		t.Fatalf("unexpected result type: %#v", result)
	}
	if got.Connected || got.ConnectionID != "" {
		t.Errorf("result = %+v, want Connected=false ConnectionID=\"\"", got)
	}
}

func TestAccountsResolveDevServerConnection_MissingDevServerID_FailsFastWithoutCallingResolve(t *testing.T) {
	called := false
	fake := &fakeAccountsRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.resolveDevServerConnection", args)
	if err == nil {
		t.Fatal("expected an error for missing devServerId")
	}
	if called {
		t.Error("ResolveConnection must not be called when devServerId is missing")
	}
}

func TestAccountsResolveDevServerConnection_UnknownDevServerID_ResolveErrorPassesThrough(t *testing.T) {
	wantErr := errors.New("INFRA_DEV_SERVER_NOT_FOUND: no such dev server")
	fake := &fakeAccountsRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds-unknown"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.resolveDevServerConnection", args)
	if !errors.Is(err, wantErr) && (err == nil || err.Error() != wantErr.Error()) {
		t.Fatalf("expected ResolveConnection's error to pass through verbatim, got: %v", err)
	}
}

func readAccountsEvent(t *testing.T, events <-chan PushEvent) map[string]any {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("events channel closed before expected event arrived")
		}
		if ev.Channel != "accounts.event" || len(ev.Args) != 1 {
			t.Fatalf("unexpected PushEvent shape: %+v", ev)
		}
		payload, ok := ev.Args[0].(map[string]any)
		if !ok {
			t.Fatalf("event payload is not a map[string]any: %#v", ev.Args[0])
		}
		return payload
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for accounts.subscribe push event")
		return nil
	}
}
