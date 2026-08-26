// Package wscompat — browser.* pane-control channels (SOL-006 Groups A/B).
// See specs/backend-go/bugs/missing-v1/solutions/SOL-006-browser-channels.md.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerBrowserChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	// Group A + B — one relay handler per channel, all sharing
	// registerBrowserRelay's resolve-then-relay logic.
	for _, op := range []string{
		"eval", "keypress", "mouseDown", "mouseMove", "mouseUp", "mouseWheel",
		"viewport", "tabCreate", "tabClose",
	} {
		registerBrowserRelay(r, client, "browser."+op, "browser."+op)
	}
}

// registerBrowserRelay is the single representative sketch for all 9
// Group A/B channels — each op's params shape differs (viewport carries
// width/height, mouseMove carries x/y, etc.) but the resolve-then-relay
// skeleton is identical, so params are passed through opaquely rather than
// typed per-op, mirroring RelayRequest.params_json's own design choice.
func registerBrowserRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("BROWSER_MISSING_ARGS: %s requires a params object", channel)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args[0], &raw); err != nil {
			return nil, err
		}
		var worktreeID string
		if wt, ok := raw["worktree"]; ok {
			_ = json.Unmarshal(wt, &worktreeID)
		}
		if worktreeID == "" {
			return nil, fmt.Errorf("BROWSER_NO_WORKTREE: %s requires a worktree selector", channel)
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})

		resolved, err := client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{WorktreeId: worktreeID})
		if err != nil {
			return nil, err
		}
		if !resolved.GetConnected() {
			return nil, fmt.Errorf("BROWSER_NO_CONNECTION: worktree %s has no bound dev server", worktreeID)
		}

		resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{
			// The Relay RPC resolves connection_id against infra.connections.id,
			// a different id space than DevServer.Id — use the resolved
			// connectionId (TASK-025), not the dev server's own id.
			ConnectionId: resolved.GetConnectionId(),
			Method:       agentMethod,
			ParamsJson:   string(args[0]),
		})
		if err != nil {
			return nil, err
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(resp.GetResultJson()), &result); err != nil {
			return nil, err
		}
		return result, nil
	})
}
