# TASK-034: Add `browser.*` pane-control relay channels (Groups A & B)

**From Solution:** SOL-006 (Groups A & B)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_browser.go` (new), `channels.go`
**Depends on:** TASK-025
**Status:** `[x]` DONE (verified — `go build`/`go vet` clean for `services/api-gateway/internal/adapter/wscompat/...`; `go test ./services/api-gateway/internal/adapter/wscompat/... -run Browser -v` passes, including `TestBrowserChannels_RequiresWorktree_FailsFastWithoutResolving`/`TestBrowserChannels_ResolvesWorktreeThenRelaysFullParams`/`TestBrowserChannels_NotConnected_Errors`/`TestBrowserChannels_AllGroupAAndBChannels_ResolveThenRelay` across all 9 Group A/B channels. Per orchestration corrections: no `rpcTimeout`; `registerBrowserRelay` uses `resolved.GetConnectionId()` (the TASK-025 `ResolveConnectionResponse.connection_id` field), not `resolved.GetDevServer().GetId()`, for `RelayRequest.ConnectionId` — this resolves the sketch's own flagged uncertainty. `registerBrowserChannels` is defined but NOT wired into `channels.go` — separate final-integration pass. `browser.screencast` remains out of scope, as noted.)

---

## Context

9 channels — Group A (live input/eval: `eval`, `keypress`, `mouseDown`,
`mouseMove`, `mouseUp`, `mouseWheel`, `viewport`) and Group B (tab
lifecycle: `tabCreate`, `tabClose`) — all relay via one generic
`Relay(connectionId, "browser.<op>", params)` call, no per-op proto
message (mirrors `RelayRequest.params_json`'s own "no per-method typed
message" design). Every real call site passes a `worktree` selector, not a
`connectionId` — resolved via TASK-025's
`ResolveConnectionRequest.worktree_id` field.

**INERT UNTIL AGENT-SIDE WORK LANDS** — see TASK-036: none of these 9
methods exist on the Dev Server Agent's JSON-RPC dispatcher today, and per
SOL-006 this is a substantially larger ask than SOL-004's `accounts.*` gap
(launching and controlling a full browser process, not just fs read/write).
This task's plumbing is correct and buildable on its own merits regardless.

---

## Changes to make

### New file: `services/api-gateway/internal/adapter/wscompat/channels_browser.go`

```go
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
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		resolved, err := client.ResolveConnection(rpcCtx, &infrafleetv1.ResolveConnectionRequest{WorktreeId: worktreeID})
		if err != nil {
			return nil, err
		}
		if !resolved.GetConnected() {
			return nil, fmt.Errorf("BROWSER_NO_CONNECTION: worktree %s has no bound dev server", worktreeID)
		}

		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
			// See TASK-028's identical flagged note: verify at
			// implementation time that DevServer.Id is what Relay expects
			// as connection_id for a worktree-resolved connection, not a
			// separate echoed connection_id field.
			ConnectionId: resolved.GetDevServer().GetId(),
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
```

`browser.viewport`'s `worktree`/`page`/`width`/`height` params pass through
unchanged inside `params_json` — the agent's new `browser.viewport` method
(once it exists) reads them directly.

**Not implemented here, flagged for later**: `browser.screencast` (not in
BUG-006's 15-method list; needs a dedicated server-streaming RPC once the
route is resolved, the same "resolve once via a unary call, then open a
dedicated streaming path" shape `infra-fleet-service.md` §7 documents for
terminal I/O — SOL-006's own note, not designed in this task).

### `channels.go` — register the new channels

Add `registerBrowserChannels(r, infraFleetClient)` to
`RegisterRealChannels`'s body, alongside `registerBrowserProfileChannels`
(TASK-033).

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

Expected: clean build.
