// Package wscompat — github.checkAuthStatus / gitlab.checkAuthStatus
// channels.
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
//
// DEVIATION FROM SOL-INT-01/TASK-INT-01-01: github.startAuthLogin and
// gitlab.startAuthLogin are deliberately NOT registered here.
// channels_scm.go already registers both channel names (see its
// registerSCMChannels — "github.startAuthLogin / github.revokeAuth" /
// "gitlab.startAuthLogin / gitlab.revokeAuth") as thin wrappers over
// scm-integration-service's StartOAuthFlow, a redirect-URI browser OAuth
// flow the frontend already depends on to connect a GitHub/GitLab account.
// Registry.Register overwrites on a repeated channel key (see channels.go's
// registerGitDeepChannels-vs-registerGitChannels comment for the same
// mechanism used deliberately elsewhere) — registering a second, unrelated
// "spawn `gh auth login` in a PTY" handler under the identical channel name
// would silently replace that already-working OAuth flow with a
// completely different, incompatible one. That is a naming collision this
// task's authors did not anticipate (checked against the real
// channels_scm.go, not assumed), not something safe to resolve
// unilaterally here — it needs an explicit rename (e.g.
// "github.startCliAuthLogin") or a product decision to retire the OAuth
// path before a PTY-spawning handler can safely take this name. See this
// task's Status line for the full note.
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
//
// NOT wired into registerCLIAuthChannels — see this file's package doc
// comment's "DEVIATION" section: "github.startAuthLogin"/
// "gitlab.startAuthLogin" are already taken by channels_scm.go's
// OAuth-flow handlers. Kept here, implemented and unit-tested, so a future
// rename only needs a new r.Register call site once a non-colliding
// channel name is chosen.
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
