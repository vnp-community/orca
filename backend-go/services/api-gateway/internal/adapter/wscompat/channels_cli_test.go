package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeCliRelayClient is a minimal test double scoped to RelayByDevServer —
// the only RPC cli.* channel handlers call, same shape as
// fakeAccountsRelayClient in channels_accounts_test.go.
type fakeCliRelayClient struct {
	infrafleetv1.InfraFleetServiceClient

	relayByDevServerFunc func(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeCliRelayClient) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayByDevServerFunc(ctx, in)
}

func TestCliChannels_RelaySuccess(t *testing.T) {
	cases := []struct {
		channel    string
		args       map[string]any
		wantMethod string
		wantParams map[string]any
	}{
		{"cli.getInstallStatus", map[string]any{"devServerId": "ds-1"}, "cli.getInstallStatus", map[string]any{}},
		{"cli.install", map[string]any{"devServerId": "ds-1"}, "cli.install", map[string]any{}},
		{"cli.remove", map[string]any{"devServerId": "ds-1"}, "cli.remove", map[string]any{}},
		{
			"cli.getWslInstallStatus",
			map[string]any{"devServerId": "ds-1", "distro": "Ubuntu"},
			"cli.getWslInstallStatus",
			map[string]any{"distro": "Ubuntu"},
		},
		{
			"cli.installWsl",
			map[string]any{"devServerId": "ds-1", "distro": "Ubuntu"},
			"cli.installWsl",
			map[string]any{"distro": "Ubuntu"},
		},
		{
			"cli.removeWsl",
			map[string]any{"devServerId": "ds-1"},
			"cli.removeWsl",
			map[string]any{"distro": nil},
		},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			var gotReq *infrafleetv1.RelayByDevServerRequest
			fake := &fakeCliRelayClient{
				relayByDevServerFunc: func(_ context.Context, in *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
					gotReq = in
					return &infrafleetv1.RelayResponse{ResultJson: `{"state":"installed","supported":true}`}, nil
				},
			}
			r := NewRegistry()
			registerCliChannels(r, fake)

			args := argsJSON(t, tc.args)
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, tc.channel, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := result.(map[string]any)
			if !ok || got["state"] != "installed" {
				t.Fatalf("unexpected result: %#v", result)
			}
			if gotReq.GetDevServerId() != "ds-1" {
				t.Errorf("DevServerId = %q, want ds-1", gotReq.GetDevServerId())
			}
			if gotReq.GetMethod() != tc.wantMethod {
				t.Errorf("Method = %q, want %q", gotReq.GetMethod(), tc.wantMethod)
			}
			var params map[string]any
			if err := json.Unmarshal([]byte(gotReq.GetParamsJson()), &params); err != nil {
				t.Fatalf("params_json not valid JSON: %v", err)
			}
			for k, want := range tc.wantParams {
				if got := params[k]; got != want {
					t.Errorf("params[%q] = %v, want %v", k, got, want)
				}
			}
		})
	}
}

func TestCliChannels_MissingDevServerID_FailsFastWithoutCallingRelay(t *testing.T) {
	for _, channel := range []string{
		"cli.getInstallStatus", "cli.install", "cli.remove",
		"cli.getWslInstallStatus", "cli.installWsl", "cli.removeWsl",
	} {
		t.Run(channel, func(t *testing.T) {
			called := false
			fake := &fakeCliRelayClient{
				relayByDevServerFunc: func(context.Context, *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
					called = true
					return nil, nil
				},
			}
			r := NewRegistry()
			registerCliChannels(r, fake)

			args := argsJSON(t, map[string]any{}) // no devServerId
			_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel, args)
			if err == nil {
				t.Fatal("expected an error for missing devServerId")
			}
			if called {
				t.Error("RelayByDevServer must not be called when devServerId is missing")
			}
		})
	}
}

func TestCliStatusChannels_NotConnected_DegradesToUnsupportedStatus(t *testing.T) {
	for _, channel := range []string{"cli.getInstallStatus", "cli.getWslInstallStatus"} {
		t.Run(channel, func(t *testing.T) {
			fake := &fakeCliRelayClient{
				relayByDevServerFunc: func(context.Context, *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
					return nil, status.Error(codes.FailedPrecondition, "dev server has no live agent session")
				},
			}
			r := NewRegistry()
			registerCliChannels(r, fake)

			args := argsJSON(t, map[string]any{"devServerId": "ds-1"})
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel, args)
			if err != nil {
				t.Fatalf("expected a degraded status, not an error: %v", err)
			}
			got, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("unexpected result type: %#v", result)
			}
			if got["supported"] != false {
				t.Errorf("supported = %v, want false", got["supported"])
			}
			if got["state"] != "unsupported" {
				t.Errorf("state = %v, want unsupported", got["state"])
			}
		})
	}
}

func TestCliMutationChannels_NotConnected_ErrorPropagatesVerbatim(t *testing.T) {
	for _, channel := range []string{"cli.install", "cli.remove", "cli.installWsl", "cli.removeWsl"} {
		t.Run(channel, func(t *testing.T) {
			wantErr := status.Error(codes.FailedPrecondition, "dev server has no live agent session")
			fake := &fakeCliRelayClient{
				relayByDevServerFunc: func(context.Context, *infrafleetv1.RelayByDevServerRequest) (*infrafleetv1.RelayResponse, error) {
					return nil, wantErr
				},
			}
			r := NewRegistry()
			registerCliChannels(r, fake)

			args := argsJSON(t, map[string]any{"devServerId": "ds-1"})
			_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel, args)
			if !errors.Is(err, wantErr) && (err == nil || err.Error() != wantErr.Error()) {
				t.Fatalf("expected RelayByDevServer's error to pass through verbatim, got: %v", err)
			}
		})
	}
}
