# TASK-021: Add `accounts.*` wscompat relay channels

**From Solution:** SOL-004
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_accounts.go` (new), `channels.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`accounts.selectClaude`/`accounts.selectCodex`/`accounts.removeClaude`/
`accounts.removeCodex` currently fall through to `notImplementedHandler`.
Per SOL-004, this is a pure `wscompat`-layer wiring gap — no new
`InfraFleetService` RPC, no new usecase, no new Postgres table is needed.
All 4 channels relay through `infra-fleet-service`'s **existing** generic
`Relay` RPC (`infrafleet.proto:103-116`, `usecase/relay.go`), which already
resolves a `connectionId` to a dev server and forwards `method`+`params` to
the Dev Server Agent's JSON-RPC dispatcher.

`main.go` already dials `infraFleetClient` and already passes it into
`wscompat.RegisterRealChannels` (see `cmd/server/main.go:241`) — no
signature change is needed for this task, unlike TASK-005/TASK-006 in this
same tasks/ directory which had to add a new parameter.

**Known limitation, not fixed by this task (see TASK-023):** every relayed
call requires a `connectionId`, but the frontend's documented call-site
params for these 4 methods are bare `{ accountId }` — no connection
identifier. The handler below fails fast with `ACCOUNTS_NO_CONNECTION`
when `connectionId` is missing, rather than guessing. TASK-023 documents
this gap and the separate agent-side gap (the 4 relayed JSON-RPC methods
don't exist on the Dev Server Agent yet) — both are out of scope for
`backend-go` code changes and are tracked there, not fixed here.

---

## Changes to make

### New file: `services/api-gateway/internal/adapter/wscompat/channels_accounts.go`

```go
// Package wscompat — accounts.* channels.
//
// accounts.selectClaude/selectCodex/removeClaude/removeCodex relay through
// infra-fleet-service's existing generic Relay RPC — see SOL-004
// (specs/backend-go/bugs/missing-v1/solutions/SOL-004-accounts-channels.md)
// for why this is not a new service or new backend-side storage: reading/
// writing the Claude/Codex CLI's login config is filesystem-shaped work on
// the target dev server host, the same class of thing devServer.*/fleet.*
// already relay for.
//
// INERT UNTIL AGENT-SIDE WORK LANDS: the Dev Server Agent method names
// below (accounts.selectClaude, etc.) do not exist on the agent's JSON-RPC
// dispatcher yet — see TASK-023 (specs/backend-go/bugs/missing-v1/tasks/
// TASK-023-document-accounts-agent-gap.md). This file's plumbing is
// correct and buildable on its own merits; every call will fail with a
// "method not found" error from the agent until that companion work ships.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// registerAccountsChannels wires accounts.* to infra-fleet-service's
// existing generic Relay RPC. See this file's package doc comment (SOL-004)
// for why no new proto/usecase code is needed on infra-fleet-service's side.
func registerAccountsChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerAccountsRelay(r, client, "accounts.selectClaude", "accounts.selectClaude")
	registerAccountsRelay(r, client, "accounts.selectCodex", "accounts.selectCodex")
	registerAccountsRelay(r, client, "accounts.removeClaude", "accounts.removeClaude")
	registerAccountsRelay(r, client, "accounts.removeCodex", "accounts.removeCodex")
}

// accountsRelayArgs is shared by all 4 channels — accountId plus the
// connectionId prerequisite (see this file's package doc comment and
// TASK-023's "Open prerequisite" note).
type accountsRelayArgs struct {
	AccountID    string `json:"accountId"`
	ConnectionID string `json:"connectionId"`
}

// registerAccountsRelay is the single representative implementation shared
// by all 4 channels (select/remove x Claude/Codex are identical in shape —
// only the channel name and the relayed agent method name differ).
func registerAccountsRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[accountsRelayArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			// See TASK-023 — accounts.* has no connectionId in today's
			// documented frontend params; fail loudly rather than guessing
			// (e.g. "the tenant's only connection" would silently break
			// multi-environment tenants).
			return nil, fmt.Errorf("ACCOUNTS_NO_CONNECTION: connectionId is required until the frontend contract adds it")
		}
		paramsJSON, err := json.Marshal(map[string]any{"accountId": in.AccountID})
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
			ConnectionId: in.ConnectionID,
			Method:       agentMethod,
			ParamsJson:   string(paramsJSON),
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

Find `RegisterRealChannels`'s body:

```go
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Add `registerAccountsChannels(r, infraFleetClient)` alongside
`registerDevServerChannels`/`registerFleetChannels` (same client, same
`AttachIdentity` requirement):

```go
	registerAnnotationChannels(r, annotationClient)
	registerTaskChannels(r, taskClient)
	registerGitChannels(r, gitClient)
	registerAutomationChannels(r, automationClient)
	registerPreflightChannels(r)
	registerDevServerChannels(r, infraFleetClient)
	registerFleetChannels(r, infraFleetClient)
	registerAccountsChannels(r, infraFleetClient) // NEW — accounts.* (SOL-004 / TASK-021)
	registerCrashReportChannels(r)
	registerRateLimitChannels(r, rateLimits)
}
```

Also update the package doc comment's channel count at the top of
`channels.go` (currently "Channel count: 13 wired for real...") to add the
4 new channels — bump to 17 and append
`accounts.{selectClaude,selectCodex,removeClaude,removeCodex} (4)`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./internal/adapter/wscompat/...
go vet ./internal/adapter/wscompat/...
```

Expected: clean build. No signature changes elsewhere, so `main.go` still
builds unchanged.
