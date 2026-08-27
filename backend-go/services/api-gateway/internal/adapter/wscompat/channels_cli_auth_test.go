package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeCLIAuthClient is a minimal test double scoped to Relay and
// SpawnTerminalSession — the only 2 RPCs this file's channel handlers call.
type fakeCLIAuthClient struct {
	infrafleetv1.InfraFleetServiceClient

	relayFunc  func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
	spawnFunc  func(ctx context.Context, in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error)
	relayCalls int
}

func (f *fakeCLIAuthClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	f.relayCalls++
	return f.relayFunc(ctx, in)
}

func (f *fakeCLIAuthClient) SpawnTerminalSession(ctx context.Context, in *infrafleetv1.SpawnTerminalSessionRequest, _ ...grpc.CallOption) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	return f.spawnFunc(ctx, in)
}

func TestCLIAuthChannels_CheckAuthStatusRelaysToCorrectMethod(t *testing.T) {
	cases := []struct {
		channel    string
		wantMethod string
	}{
		{"github.checkAuthStatus", "github.auth.status"},
		{"gitlab.checkAuthStatus", "gitlab.auth.status"},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			var gotReq *infrafleetv1.RelayRequest
			fake := &fakeCLIAuthClient{
				relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
					gotReq = in
					return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true,"stdout":"logged in","stderr":""}`}, nil
				},
			}
			r := NewRegistry()
			registerCLIAuthChannels(r, fake)

			args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, tc.channel, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotReq.GetMethod() != tc.wantMethod {
				t.Fatalf("expected method %q, got %q", tc.wantMethod, gotReq.GetMethod())
			}
			if gotReq.GetConnectionId() != "conn-1" {
				t.Fatalf("unexpected connectionId: %q", gotReq.GetConnectionId())
			}
			if gotReq.GetParamsJson() != `{"userId":"u1"}` {
				t.Fatalf("unexpected paramsJson: %q", gotReq.GetParamsJson())
			}
			out, ok := result.(map[string]any)
			if !ok || out["ok"] != true {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestCLIAuthChannels_CheckAuthStatusMissingConnectionIDFailsFast(t *testing.T) {
	fake := &fakeCLIAuthClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			t.Fatal("Relay should not be called when connectionId is missing")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerCLIAuthChannels(r, fake)

	args := argsJSON(t, map[string]any{})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "github.checkAuthStatus", args)
	if err == nil {
		t.Fatal("expected error for missing connectionId")
	}
	if fake.relayCalls != 0 {
		t.Fatalf("expected no RPC call, got %d", fake.relayCalls)
	}
}

func TestCLIAuthChannels_CheckAuthStatusMethodNotFoundPropagatesTyped(t *testing.T) {
	relayErr := errors.New("agent method not found: github.auth.status")
	fake := &fakeCLIAuthClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			return nil, relayErr
		},
	}
	r := NewRegistry()
	registerCLIAuthChannels(r, fake)

	args := argsJSON(t, map[string]any{"connectionId": "conn-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "github.checkAuthStatus", args)
	if err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
	if !errors.Is(err, relayErr) {
		t.Fatalf("expected the exact typed error to propagate, got: %v", err)
	}
}

// TestCLIAuthLoginChannel_SpawnsWithServerSideUserID exercises
// registerCLIAuthLoginChannel directly under a non-colliding test channel
// name — see channels_cli_auth.go's package doc comment "DEVIATION"
// section for why it is not wired to "github.startAuthLogin" in production.
func TestCLIAuthLoginChannel_SpawnsWithServerSideUserID(t *testing.T) {
	var gotReq *infrafleetv1.SpawnTerminalSessionRequest
	fake := &fakeCLIAuthClient{
		spawnFunc: func(_ context.Context, in *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			gotReq = in
			return &infrafleetv1.SpawnTerminalSessionResponse{Session: &infrafleetv1.TerminalSession{PtyId: "pty-1"}}, nil
		},
	}
	r := NewRegistry()
	registerCLIAuthLoginChannel(r, fake, "test.cliAuthLogin", "gh auth login --hostname github.com --web")

	// A caller-supplied userId in args must never override the
	// server-side identity — regression guard against spoofing another
	// user's isolated config dir.
	args := argsJSON(t, map[string]any{"connectionId": "conn-1", "userId": "attacker"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "real-user"}, "test.cliAuthLogin", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetUserId() != "real-user" {
		t.Fatalf("expected UserId to come from Identity.UserID, got %q", gotReq.GetUserId())
	}
	if gotReq.GetCommand() != "gh auth login --hostname github.com --web" {
		t.Fatalf("unexpected command: %q", gotReq.GetCommand())
	}
	if gotReq.GetConnectionId() != "conn-1" {
		t.Fatalf("unexpected connectionId: %q", gotReq.GetConnectionId())
	}
}

func TestCLIAuthLoginChannel_MissingConnectionIDFailsFast(t *testing.T) {
	fake := &fakeCLIAuthClient{
		spawnFunc: func(context.Context, *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
			t.Fatal("SpawnTerminalSession should not be called when connectionId is missing")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerCLIAuthLoginChannel(r, fake, "test.cliAuthLogin", "gh auth login")

	args := argsJSON(t, map[string]any{})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "test.cliAuthLogin", args)
	if err == nil {
		t.Fatal("expected error for missing connectionId")
	}
}
