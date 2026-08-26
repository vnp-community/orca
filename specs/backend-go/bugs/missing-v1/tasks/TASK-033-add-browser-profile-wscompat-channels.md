# TASK-033: Add `browser.profile*` wscompat channels (metadata CRUD + relayed profile ops)

**From Solution:** SOL-006 (Group C)
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_browser_profiles.go` (new), `channels.go`
**Depends on:** TASK-025, TASK-032
**Status:** `[ ]` TODO

---

## Context

6 channels: `browser.profileList`/`profileCreate`/`profileDelete` are
**functional now** — pure Postgres CRUD via TASK-032's new
`InfraFleetService` RPCs, no agent involvement, no `agent/`-side blocker.
`browser.profileClearDefaultCookies`/`profileDetectBrowsers`/
`profileImportFromBrowser` act on the dev server's actual filesystem/
installed-browser state, so they relay via `Relay` — keyed by
`dev_server_id` directly (no worktree involved, per SOL-006: "a profile is
a dev-server-level resource, not a worktree-level one"), using TASK-025's
`ResolveConnectionRequest.dev_server_id` field to first resolve the active
`connectionId`. **These 3 are inert** until the agent implements the
corresponding JSON-RPC methods (same out-of-scope flag as TASK-023/
TASK-036 — tracked in TASK-036, not duplicated here).

---

## Changes to make

### New file: `services/api-gateway/internal/adapter/wscompat/channels_browser_profiles.go`

```go
// Package wscompat — browser.profile* channels (SOL-006 Group C). See
// specs/backend-go/bugs/missing-v1/solutions/SOL-006-browser-channels.md.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerBrowserProfileChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("browser.profileList", handleBrowserProfileList(client))
	r.Register("browser.profileCreate", handleBrowserProfileCreate(client))
	r.Register("browser.profileDelete", handleBrowserProfileDelete(client))

	// Relayed to the agent, keyed by dev_server_id directly (no worktree
	// involved — a profile is dev-server-scoped). INERT until the agent
	// implements these 3 methods — see TASK-036.
	for _, op := range []string{"profileClearDefaultCookies", "profileDetectBrowsers", "profileImportFromBrowser"} {
		registerBrowserProfileRelay(r, client, "browser."+op, "browser."+op)
	}
}

func browserProfileView(p *infrafleetv1.BrowserProfile) map[string]any {
	return map[string]any{
		"id":            p.GetId(),
		"devServerId":   p.GetDevServerId(),
		"name":          p.GetName(),
		"sourceBrowser": p.GetSourceBrowser(),
		"isDefault":     p.GetIsDefault(),
	}
}

func handleBrowserProfileList(client infrafleetv1.InfraFleetServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			DevServerID string `json:"devServerId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListBrowserProfiles(rpcCtx, &infrafleetv1.ListBrowserProfilesRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		views := make([]map[string]any, 0, len(resp.GetProfiles()))
		for _, p := range resp.GetProfiles() {
			views = append(views, browserProfileView(p))
		}
		return views, nil
	}
}

func handleBrowserProfileCreate(client infrafleetv1.InfraFleetServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID   string `json:"devServerId"`
			Name          string `json:"name"`
			SourceBrowser string `json:"sourceBrowser"`
			IsDefault     bool   `json:"isDefault"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateBrowserProfile(rpcCtx, &infrafleetv1.CreateBrowserProfileRequest{
			DevServerId: in.DevServerID, Name: in.Name, SourceBrowser: in.SourceBrowser, IsDefault: in.IsDefault,
		})
		if err != nil {
			return nil, err
		}
		return browserProfileView(resp.GetProfile()), nil
	}
}

func handleBrowserProfileDelete(client infrafleetv1.InfraFleetServiceClient) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type deleteArgs struct {
			ID string `json:"id"`
		}
		in, err := decodeArg[deleteArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		if _, err := client.DeleteBrowserProfile(rpcCtx, &infrafleetv1.DeleteBrowserProfileRequest{Id: in.ID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}

// registerBrowserProfileRelay handles the 3 live-agent profile operations —
// resolve dev_server_id -> connectionId (TASK-025), then Relay. Same
// resolve-then-relay skeleton as registerBrowserRelay (TASK-034), kept as
// a separate function since this one keys by dev_server_id, not worktree.
func registerBrowserProfileRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type relayArgs struct {
			DevServerID string `json:"devServerId"`
		}
		in, err := decodeArg[relayArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("BROWSER_NO_DEV_SERVER: %s requires devServerId", channel)
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()

		resolved, err := client.ResolveConnection(rpcCtx, &infrafleetv1.ResolveConnectionRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		if !resolved.GetConnected() {
			return nil, fmt.Errorf("BROWSER_NO_CONNECTION: dev server %s has no active connection", in.DevServerID)
		}

		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
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

### `channels.go` — register the new channels

Add `registerBrowserProfileChannels(r, infraFleetClient)` to
`RegisterRealChannels`'s body, alongside `registerAccountsChannels`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

Expected: clean build.
