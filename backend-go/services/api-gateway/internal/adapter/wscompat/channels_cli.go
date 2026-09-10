// ── cli.* ────────────────────────────────────────────────────────────────
//
// cli.getInstallStatus/install/remove/getWslInstallStatus/installWsl/
// removeWsl register (or unregister) the `orca` shell command on the
// machine hosting a user's terminals. That machine is never api-gateway's
// own container — in server/web mode it is always a connected Dev Server —
// so, like preflight.detectAgents (registerOnboardingChannels) and
// accounts.select/remove (registerAccountsRelay), this is a pure relay:
// resolve nothing locally, forward the call to the agent over
// RelayByDevServer, decode its JSON result verbatim.
//
// Ports backend/src/main/runtime/rpc/methods/cli.ts's CLI_METHODS (legacy TS
// reference) — devServerId required, no local (backend-container) fallback,
// same reasoning as that file's own doc comment. Execution stays entirely on
// the agent (agent/src/relay/agent-cli-handler.ts already implements all 6
// methods; nothing there needs to change).
//
// Live gap this closes: frontend/src/renderer/src/web/web-preload-api.ts's
// createCliApi() stubs window.api.cli.* with a hardcoded
// "CLI registration is managed on the Orca server, not in the web browser"
// response, because a prior pass found every cli.* call over
// window.api.runtime.call threw "channel not yet implemented in backend-go"
// (see runtime-cli-client.ts's doc comment) — true until this file, since no
// cli.* channel existed here yet. Rewiring web-preload-api.ts to call these
// channels (instead of returning that stub) is a separate, frontend-side
// follow-up — not done in this pass.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// cliRelayTimeout mirrors the legacy TS backend's CLI_RELAY_TIMEOUT_MS
// (backend/src/main/runtime/rpc/methods/cli.ts) — longer than rpcTimeout
// (8s) because installing/removing a PATH entry can involve the agent
// touching disk on a possibly slow remote host, not just a Postgres read.
const cliRelayTimeout = 30 * time.Second

type cliDevServerArgs struct {
	DevServerID string `json:"devServerId"`
}

type cliWslArgs struct {
	DevServerID string  `json:"devServerId"`
	Distro      *string `json:"distro"`
}

func registerCliChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("cli.getInstallStatus", cliStatusHandler(client, "cli.getInstallStatus"))
	r.Register("cli.install", cliMutationHandler(client, "cli.install"))
	r.Register("cli.remove", cliMutationHandler(client, "cli.remove"))
	r.Register("cli.getWslInstallStatus", cliWslStatusHandler(client, "cli.getWslInstallStatus"))
	r.Register("cli.installWsl", cliWslMutationHandler(client, "cli.installWsl"))
	r.Register("cli.removeWsl", cliWslMutationHandler(client, "cli.removeWsl"))
}

// cliStatusHandler backs cli.getInstallStatus — a read, so a dev server with
// no live agent session degrades to an honest "unsupported" CliInstallStatus
// (cliNotConnectedStatus) instead of erroring, same tolerance
// onboardingDetectAgents already applies to preflight.detectAgents.
func cliStatusHandler(client infrafleetv1.InfraFleetServiceClient, agentMethod string) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[cliDevServerArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("CLI_NO_DEV_SERVER: devServerId is required")
		}
		result, err := relayCliMethod(ctx, id, client, in.DevServerID, agentMethod, "{}")
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				return cliNotConnectedStatus("Dev server is not connected."), nil
			}
			return nil, err
		}
		return result, nil
	}
}

// cliMutationHandler backs cli.install/cli.remove — unlike the status
// reads, a disconnected dev server has nothing to degrade to (there is no
// honest "installed" answer to fabricate), so any relay error, including
// FailedPrecondition, propagates as-is — same convention
// registerAccountsRelay's select/remove handlers use.
func cliMutationHandler(client infrafleetv1.InfraFleetServiceClient, agentMethod string) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[cliDevServerArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("CLI_NO_DEV_SERVER: devServerId is required")
		}
		return relayCliMethod(ctx, id, client, in.DevServerID, agentMethod, "{}")
	}
}

func cliWslStatusHandler(client infrafleetv1.InfraFleetServiceClient, agentMethod string) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[cliWslArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("CLI_NO_DEV_SERVER: devServerId is required")
		}
		paramsJSON, err := json.Marshal(map[string]any{"distro": in.Distro})
		if err != nil {
			return nil, err
		}
		result, err := relayCliMethod(ctx, id, client, in.DevServerID, agentMethod, string(paramsJSON))
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				return cliNotConnectedStatus("Dev server is not connected."), nil
			}
			return nil, err
		}
		return result, nil
	}
}

func cliWslMutationHandler(client infrafleetv1.InfraFleetServiceClient, agentMethod string) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[cliWslArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("CLI_NO_DEV_SERVER: devServerId is required")
		}
		paramsJSON, err := json.Marshal(map[string]any{"distro": in.Distro})
		if err != nil {
			return nil, err
		}
		return relayCliMethod(ctx, id, client, in.DevServerID, agentMethod, string(paramsJSON))
	}
}

// relayCliMethod is this file's one RelayByDevServer call site — every
// cli.* handler above funnels through it, mirroring
// registerAccountsRelay/fetchAccountsSnapshot's shared-helper shape.
// Attaches the caller's real tenant/user identity (not a zero value) —
// RelayByDevServer's usecase resolves the dev server through
// DevServerRepository.Get, which is tenant-scoped; a zero-value Identity
// would fail every call with "no tenant in request context".
func relayCliMethod(
	ctx context.Context,
	id Identity,
	client infrafleetv1.InfraFleetServiceClient,
	devServerID, agentMethod, paramsJSON string,
) (map[string]any, error) {
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	rpcCtx, cancel := context.WithTimeout(ctx, cliRelayTimeout)
	defer cancel()
	resp, err := client.RelayByDevServer(rpcCtx, &infrafleetv1.RelayByDevServerRequest{
		DevServerId: devServerID,
		Method:      agentMethod,
		ParamsJson:  paramsJSON,
	})
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if raw := resp.GetResultJson(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return nil, fmt.Errorf("%s: decoding relay result: %w", agentMethod, err)
		}
	}
	return result, nil
}

// cliNotConnectedStatus is the CliInstallStatus-shaped (frontend/src/shared/
// cli-install-types.ts) honest answer for "this dev server has no live agent
// session right now" — mirrors the old TS backend's own
// unsupportedReason:'launch_mode_unavailable' convention (see
// web-preload-api.ts's createCliApi doc comment on this file's top), not a
// fabricated installed/not_installed guess.
func cliNotConnectedStatus(detail string) map[string]any {
	return map[string]any{
		"platform":          nil,
		"commandName":       "",
		"commandPath":       nil,
		"pathDirectory":     nil,
		"pathConfigured":    false,
		"launcherPath":      nil,
		"installMethod":     nil,
		"supported":         false,
		"state":             "unsupported",
		"currentTarget":     nil,
		"unsupportedReason": "launch_mode_unavailable",
		"detail":            detail,
	}
}
