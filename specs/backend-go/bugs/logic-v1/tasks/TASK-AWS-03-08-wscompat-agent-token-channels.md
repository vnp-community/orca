# TASK-AWS-03-08: Wire `devServer.agentTokens.create/list/revoke` in `wscompat`

**From Solution:** SOL-AWS-03
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_devserver_agent_tokens.go` (new)
**Depends on:** TASK-AWS-03-07
**Status:** `[x]` DONE — channels_devserver_agent_tokens.go created, wired into RegisterRealChannels, 4 tests pass (create/list/revoke/no-devServerId + plaintext-token-never-in-list regression guard).

---

## Context

Exposes the new admin RPCs to the frontend's "DevServer Settings → Agent
Tokens tab" — same shape as `registerAccountsChannels`
(`channels_accounts.go`), decode args, attach identity, call the RPC, map
the response.

## Changes to make

Create `backend-go/services/api-gateway/internal/adapter/wscompat/channels_devserver_agent_tokens.go`:

```go
// Package wscompat — devServer.agentTokens.* channels.
//
// create/list/revoke wire BL-AWS-03's persistent agent token admin surface
// onto infra-fleet-service's CreateAgentToken/ListAgentTokens/
// RevokeAgentToken RPCs (SOL-AWS-03). Authenticated as a normal per-tenant
// admin action (session/JWT identity), NOT the ORCA_AGENT_API_SECRET gate
// TokenIssuer's bootstrap endpoint uses — see SOL-AWS-03's "reconciling
// with the existing ephemeral Registry/TokenIssuer" section for why
// conflating the two auth models would be a regression.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func registerDevServerAgentTokenChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("devServer.agentTokens.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			DevServerID string `json:"devServerId"`
			Name        string `json:"name"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.DevServerID == "" {
			return nil, fmt.Errorf("AGENT_TOKENS_NO_DEV_SERVER: devServerId is required")
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateAgentToken(rpcCtx, &infrafleetv1.CreateAgentTokenRequest{DevServerId: in.DevServerID, Name: in.Name})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id": resp.GetId(), "token": resp.GetToken(), "name": resp.GetName(),
			"createdAtUnixMs": resp.GetCreatedAtUnixMs(),
		}, nil
	})

	r.Register("devServer.agentTokens.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
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
		resp, err := client.ListAgentTokens(rpcCtx, &infrafleetv1.ListAgentTokensRequest{DevServerId: in.DevServerID})
		if err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(resp.GetTokens()))
		for _, t := range resp.GetTokens() {
			entry := map[string]any{"id": t.GetId(), "name": t.GetName(), "createdAtUnixMs": t.GetCreatedAtUnixMs()}
			if t.LastUsedAtUnixMs != nil {
				entry["lastUsedAtUnixMs"] = t.GetLastUsedAtUnixMs()
			}
			out = append(out, entry)
		}
		return map[string]any{"tokens": out}, nil
	})

	r.Register("devServer.agentTokens.revoke", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type revokeArgs struct {
			DevServerID string `json:"devServerId"`
			ID          string `json:"id"`
		}
		in, err := decodeArg[revokeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		_, err = client.RevokeAgentToken(rpcCtx, &infrafleetv1.RevokeAgentTokenRequest{DevServerId: in.DevServerID, Id: in.ID})
		return map[string]bool{"ok": err == nil}, err
	})
}
```

Add `registerDevServerAgentTokenChannels(r, infraFleetClient)` to
`RegisterRealChannels` in `channels.go` (final integration pass block, same
place `registerAccountsChannels`/`registerFleetChannels` are called —
`infraFleetClient` is already dialed there, no new client needed).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/...
```

Expected: clean build/tests. Add
`wscompat/channels_devserver_agent_tokens_test.go` per SOL-AWS-03's test
plan: one test per channel using a fake `InfraFleetServiceClient`, plus a
test asserting the plaintext token from `create` is never re-derivable from
`list`'s response shape (i.e. `list`'s map never has a `"token"` key).
