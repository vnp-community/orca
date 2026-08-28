# TASK-INT-01-01: Wire `github.checkAuthStatus`/`gitlab.checkAuthStatus`/`*.startAuthLogin` in `wscompat`

**From Solution:** SOL-INT-01
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli_auth.go` (new)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`preflight.check` is a hardcoded local literal (`channels.go:566-573`) and
nothing in `wscompat` calls `InfraFleetService.Relay`'s already-real
`github.auth.status`/`gitlab.auth.status` agent methods
(`agent/src/relay/agent-rpc-dispatch.ts:1029-1047`). This adds the missing
wiring only — same shape as `registerAccountsChannels`
(`channels_accounts.go`), a fifth caller of the existing generic `Relay`
RPC.

**Correction to SOL-INT-01's code sample**: the solution's
`registerCLIAuthLoginChannel` assumes `SpawnTerminalSessionRequest` already
has `Command`/`UserId` fields — checked against the real proto
(`infrafleet.proto:310-316`), it only has `connection_id`/`cwd`/`shell`/
`cols`/`rows`. This task adds the two missing fields as an additive proto
change (small, low-risk — see "Changes to make" below) rather than silently
assuming they exist.

## Changes to make

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, extend
`SpawnTerminalSessionRequest` (`:310-316`):

```protobuf
message SpawnTerminalSessionRequest {
  string connection_id = 1;  // empty = host-local; rejected in server-deployment mode, see usecase.SpawnTerminalSession
  string cwd = 2;
  string shell = 3;          // optional; agent applies its own default if empty
  int32  cols = 4;
  int32  rows = 5;
  // command, when set, is the initial command line the spawned shell runs
  // (agent's pty.create `command` param) instead of an interactive shell
  // prompt — added for github.startAuthLogin/gitlab.startAuthLogin
  // (TASK-INT-01-01), which spawn `gh auth login`/`glab auth login`
  // directly rather than typing it into an interactive shell.
  string command = 6;
  // user_id engages pty-handler.ts's per-user GH_CONFIG_DIR/GLAB_CONFIG_DIR
  // env isolation for a `gh `/`glab `-prefixed command — see
  // pty-handler.ts:680-699. Always set from the caller's authenticated
  // identity server-side (wscompat), never trusted from client args.
  string user_id = 7;
}
```

Regenerate stubs (`buf generate proto`) before implementing the channels
below. Then add
`backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go`'s
existing input struct + `adapter/grpc/server.go`'s `SpawnTerminalSession`
handler + `devserveragent`'s `SpawnPtyInput`/`pty.create` call site to
thread `Command`/`UserID` through to the agent's `pty.create` params (the
agent-side `pty-handler.ts:680-699` command-prefix match already exists —
this is wiring the two new fields from the gRPC request down to that
existing agent call, not new agent-side work).

Create `backend-go/services/api-gateway/internal/adapter/wscompat/channels_cli_auth.go`:

```go
// Package wscompat — github.checkAuthStatus / gitlab.checkAuthStatus /
// github.startAuthLogin / gitlab.startAuthLogin channels.
//
// Relay through infra-fleet-service's existing generic Relay RPC — see
// SOL-INT-01. AGENT-SIDE WORK ALREADY LANDED (unlike accounts.*):
// agent/src/relay/agent-rpc-dispatch.ts already implements
// github.auth.status/gitlab.auth.status with real per-user GH_CONFIG_DIR/
// GLAB_CONFIG_DIR isolation (external-api-connector.ts's buildGhEnv/
// buildGlabEnv); this file only adds the missing wscompat registration.
// Requires connectionId, same open prerequisite as accounts.* (see
// cliAuthArgs's doc comment).
//
// relay-ssh Dev Servers don't implement github.auth.status/
// gitlab.auth.status yet (Part A/Part B divergence, see
// infra-fleet-service.md §10) — a Relay call against one of those returns
// domain.ErrAgentMethodNotFound-shaped error, surfaced here as a typed
// error rather than swallowed; SOL-INT-03's merge layer degrades this
// honestly into a "not available on this connection mode" skip result.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

type cliAuthArgs struct {
	ConnectionID string `json:"connectionId"`
}

func registerCLIAuthChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerCLIAuthStatusRelay(r, client, "github.checkAuthStatus", "github.auth.status")
	registerCLIAuthStatusRelay(r, client, "gitlab.checkAuthStatus", "gitlab.auth.status")
	registerCLIAuthLoginChannel(r, client, "github.startAuthLogin", "gh auth login --hostname github.com --web")
	registerCLIAuthLoginChannel(r, client, "gitlab.startAuthLogin", "glab auth login --hostname gitlab.com --web")
}

// registerCLIAuthStatusRelay is the shared shape for both status checks —
// same pattern as channels_accounts.go's registerAccountsRelay.
func registerCLIAuthStatusRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[cliAuthArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			return nil, fmt.Errorf("CLI_AUTH_NO_CONNECTION: connectionId is required")
		}
		paramsJSON, err := json.Marshal(map[string]any{"userId": id.UserID})
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
			ConnectionId: in.ConnectionID, Method: agentMethod, ParamsJson: string(paramsJSON),
		})
		if err != nil {
			return nil, err // ErrAgentMethodNotFound (relay-ssh) surfaces here, mapped by SOL-INT-03's merge layer, not swallowed here
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(resp.GetResultJson()), &out); err != nil {
			return nil, err
		}
		return out, nil // {ok, stdout, stderr} verbatim from external-api-connector.ts
	})
}

// registerCLIAuthLoginChannel spawns an interactive `gh`/`glab auth login`
// PTY via the already-existing SpawnTerminalSession RPC — not a new agent
// RPC. UserId is forwarded so pty-handler.ts's existing command-prefix
// match (`gh `/`glab `) engages the same per-user env isolation the status
// checks use.
func registerCLIAuthLoginChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, command string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[cliAuthArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			return nil, fmt.Errorf("CLI_AUTH_NO_CONNECTION: connectionId is required")
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.SpawnTerminalSession(rpcCtx, &infrafleetv1.SpawnTerminalSessionRequest{
			ConnectionId: in.ConnectionID,
			Command:      command,
			UserId:       id.UserID, // engages pty-handler.ts's buildGhEnv/buildGlabEnv branch
		})
		if err != nil {
			return nil, err
		}
		return resp, nil // {ptyId, ...} — frontend attaches over the existing terminal WS stream
	})
}
```

Add `registerCLIAuthChannels(r, infraFleetClient)` to `RegisterRealChannels`
in `channels.go`'s final integration pass block (same place
`registerAccountsChannels` is called — `infraFleetClient` is already
dialed there).

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./...
go test ./services/api-gateway/internal/adapter/wscompat/... ./services/infra-fleet-service/...
```

Expected: clean build/tests; `buf breaking` reports only additions. Add
`wscompat/channels_cli_auth_test.go` per SOL-INT-01's test plan:
`github.checkAuthStatus`/`gitlab.checkAuthStatus` relay to the correct
agent method name with `{"userId": ...}` params (fake
`InfraFleetServiceClient`, mirrors `channels_accounts_test.go`'s
`fakeAccountsRelayClient` pattern); missing `connectionId` fails fast with
no RPC call; `github.startAuthLogin` calls `SpawnTerminalSession` with
`UserId` set from `Identity.UserID`, not from client-supplied args
(regression guard against a caller spoofing another user's isolated config
dir); a `Relay` call returning a `domain.ErrAgentMethodNotFound`-shaped
error propagates as a typed error, not a panic or a false `{ok:false}`.
