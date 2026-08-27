# SOL-004: Relay `accounts.*` through `infra-fleet-service`'s existing `Relay` RPC — no new backend-side storage

**Resolves:** [BUG-004](../BUG-004-accounts-channels-not-implemented.md)
**Service:** `infra-fleet-service` (no proto change — reuses the existing generic `Relay` RPC) + `api-gateway` (new `wscompat` channels)
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts_test.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (add `registerAccountsChannels` to `RegisterRealChannels`)
- `frontend/src/renderer/src/runtime/runtime-provider-accounts-client.ts` (param addition — see "Open prerequisite" below; frontend change, flagged not implemented here)
- `agent/` (new JSON-RPC methods — explicitly out of scope for this proposal, flagged below)
**Status:** 📋 Proposed — not yet implemented

---

## Service-fit decision — this is not a backend-go data-owning concern

BUG-004's own dispatch-model quote is the load-bearing fact: in the old TS
backend this was "pure backend-host filesystem I/O against the Claude/Codex
CLI's own config/credential files — no Postgres table, no relay to the Dev
Server Agent, no cross-service call." Taken literally as "add this to some
Go service's Postgres schema," that instruction doesn't survive contact with
`02-microservices-decomposition.md`'s own architecture: a horizontally
scaled, stateless Go service pod has no durable local filesystem, and even
if it did, `~/.claude`/`~/.codex` config files belong to **whichever host
actually runs the Claude/Codex CLI processes**, not to whichever pod happens
to answer this gRPC call. That is exactly the same reasoning
`02-microservices-decomposition.md`'s "What's deliberately not a separate
service" section already applies to `ai-vault.*` ("reads the backend host's
own filesystem for locally-installed CLI session history, which doesn't
make sense in a stateless multi-replica Go service").

But `accounts.*` is **not** the same shape as the two things that section
marks fully out-of-scope (browser/computer/emulator automation, `ai-vault.*`
transcript scanning) — those automate *the backend process's own host*, a
concept with no place in a stateless fleet at all. `accounts.*` is scoped
differently: per BUG-004, every one of the 4 calls fires only when
`getActiveRuntimeTarget(settings).kind === 'environment'` — i.e. only for a
backend-go-connected dev server. That is precisely the shape
`infra-fleet-service.md` §2's table already describes for PTY/git/fs: "routes
the request to the right connection, does not touch the bytes [itself] /
Yes (execution happens on the Dev Server Agent)." Reading/writing the
Claude/Codex CLI's login config is filesystem-shaped work on the *target*
host, indistinguishable in kind from a `files.*` read/write — it just
happens to target a specific well-known config path instead of an
arbitrary one.

**Conclusion:** this is not a new service, and it is not backend-side
storage in any Go service's Postgres. It is a relay through
`infra-fleet-service`'s existing Dev Server Agent connection, using the
**generic `Relay` RPC that already exists** in `infrafleet.proto` today
(`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:103-116`) —
purpose-built for exactly this: "the generic connectionId+method+params
passthrough onto the Dev Server Agent execution plane... so cross-service
callers (git-gateway-service's `RelayExecutor`, workflow-service's step
executors, **api-gateway's `wscompat`**) can reach a dev server without each
reimplementing resolve-then-exec"
(`backend-go/services/infra-fleet-service/internal/usecase/relay.go:19-27`'s
doc comment names `wscompat` explicitly as an intended caller). No new
`InfraFleetService` RPC, no new usecase, no new Postgres table — the design
already exists; this bug is a pure `wscompat`-layer wiring gap over it.

---

## Design — `wscompat` channel wiring

`Relay`'s usecase (`relay.go:37-62`) requires a non-empty `ConnectionID` and
`Method`, resolves the connection via `ConnectionResolver`, and calls
`DevServerAgentClient.Exec(ctx, devServer, method, params)` — returning
`INFRA_CONNECTION_NOT_FOUND` if nothing owns the id, never a silent
empty/zero result (`relay.go:53-55`, matching `ScanWorkspacePorts`'s "always
relay, never silently swallow" rule that same doc comment cites as
precedent). This is exactly the shape `accounts.*` needs: 4 handlers, each a
one-line call into `client.Relay`, no bespoke resolution logic to write.

```go
// channels_accounts.go
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
// existing generic Relay RPC — see this file's package-level doc comment
// (SOL-004) for why no new proto/usecase code is needed on
// infra-fleet-service's side. The Dev Server Agent method names below
// (accounts.selectClaude, etc.) are new JSON-RPC methods the agent itself
// must implement — see the "Agent-side companion work" section; this
// handler only relays, it does not implement CLI-config file I/O anywhere
// in backend-go.
func registerAccountsChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	registerAccountsRelay(r, client, "accounts.selectClaude", "accounts.selectClaude")
	registerAccountsRelay(r, client, "accounts.selectCodex", "accounts.selectCodex")
	registerAccountsRelay(r, client, "accounts.removeClaude", "accounts.removeClaude")
	registerAccountsRelay(r, client, "accounts.removeCodex", "accounts.removeCodex")
}

// accountsRelayArgs is shared by all 4 channels — accountId plus the
// connectionId prerequisite (see "Open prerequisite" below).
type accountsRelayArgs struct {
	AccountID    string `json:"accountId"`
	ConnectionID string `json:"connectionId"`
}

// registerAccountsRelay is one representative implementation shared by all
// 4 channels (select/remove x Claude/Codex are identical in shape — only
// the channel name and the relayed agent method name differ), following
// this task's "group methods that share one pattern, one sketch" guidance.
func registerAccountsRelay(r *Registry, client infrafleetv1.InfraFleetServiceClient, channel, agentMethod string) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[accountsRelayArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			// See "Open prerequisite" — accounts.* has no connectionId in
			// today's documented params; fail loudly rather than guessing.
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

Add `registerAccountsChannels(r, infraFleetClient)` to `RegisterRealChannels`
(`channels.go:64-82`), alongside `registerDevServerChannels`/
`registerFleetChannels` — same client, same identity-attachment requirement
(per `channels.go:279-287`'s doc comment: infra-fleet-service needs tenant
metadata on the outbound gRPC context, unlike most other channels in that
file).

### Open prerequisite — `accounts.*` has no `connectionId` in its documented params today

`Relay` requires a `connectionId` (`relay.go:42-43`), but BUG-004's call-site
table lists every one of the 4 methods' params as bare `{ accountId }` —
no environment/connection identifier. This is a real gap in the frontend
contract, not something `wscompat` can paper over: `Identity`
(`registry.go:12-15`) carries only `TenantID`/`UserID`, no session-scoped
connection, and none of `channels.go`'s existing patterns resolve a
`connectionId` implicitly (compare `git.status`/`git.diff`, which pass a
`worktreeId` and let `git-gateway-service` resolve the connection
internally — `channels.go:222-251`). Since `runtime-provider-accounts-client.ts`
already calls `getActiveRuntimeTarget(settings)` client-side before making
any of these 4 calls (BUG-004's dispatch-model section), the natural fix is
a small frontend addition — thread that already-resolved environment's
`connectionId` into the RPC params alongside `accountId` — not a
backend-only change. Flagging this explicitly rather than inventing a
connectionId-resolution mechanism api-gateway has no information to perform
correctly (guessing "the tenant's only connection" would silently break
multi-environment tenants).

---

## Agent-side companion work (flagged out of scope for this proposal)

`Relay` only forwards `method`/`params` to whatever the Dev Server Agent's
JSON-RPC dispatcher already implements (`devServerAgentClient.Exec`,
`relay.go:57`) — it does not itself know how to read a Claude/Codex CLI
config file. `accounts.selectClaude`/`accounts.selectCodex`/
`accounts.removeClaude`/`accounts.removeCodex` need to exist as real
JSON-RPC methods on the agent (`agent/`) before this relay wiring does
anything but return a "method not found" error from the agent's own
dispatcher. Per `08-inter-service-communication.md`'s "Talking to the Dev
Server Agent" section, `agent/` changes are **explicitly out of scope** for
"the Go rewrite of `backend/`" — this proposal's `backend-go`-side plumbing
(the `wscompat` handlers above) is complete and correct on its own merits,
but is inert until a companion `agent/` change ships these 4 methods. Unlike
BUG-006's browser-pane gap (see SOL-006), this companion work is small: fs
read/write against a known config path, well within the agent's existing
filesystem capability, not a new execution-plane capability class — worth
noting as a much lower-risk agent-side ask than SOL-006's.

The companion `accounts.subscribe` streaming method (BUG-004's "out of
scope for this report's 4-method list" note) would need a corresponding
server-streaming counterpart on `InfraFleetService` (or a poll-based
`wscompat` bridge) once picked up — not designed here, flagged for
whoever implements the 4 methods above to track alongside.

---

## Test plan

- `channels_accounts_test.go` — one test per channel: fake
  `InfraFleetServiceClient.Relay` returning a canned `result_json`, assert
  the handler decodes `accountId` into `params_json` correctly and returns
  the unmarshaled result.
- Missing-`connectionId` case: assert `ACCOUNTS_NO_CONNECTION` is returned
  without calling `Relay` at all (fail fast, matching `relay.go:42-43`'s own
  guard).
- `Relay` error passthrough: fake client returns `INFRA_CONNECTION_NOT_FOUND`
  → handler returns that error verbatim, not swallowed.
- Once the frontend param addition lands: an integration test exercising
  the full path from a fake WS invoke envelope through to a fake
  `InfraFleetServiceClient`, confirming `connectionId` round-trips from the
  frontend's `getActiveRuntimeTarget` resolution into `RelayRequest`.

## References

- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md` — "What's deliberately not a separate service" (the `ai-vault.*`/browser-automation precedent this solution's service-fit reasoning mirrors)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md` — "Talking to the Dev Server Agent" section, `agent/` out-of-scope boundary
- `specs/backend-go/tdd/services/infra-fleet-service.md` §2 — the PTY/git/fs coordination-vs-execution split this solution's reasoning extends to `accounts.*`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:103-116` — the existing generic `Relay`/`RelayRequest`/`RelayResponse` RPC this solution reuses, unmodified
- `backend-go/services/infra-fleet-service/internal/usecase/relay.go` — `Relay` usecase; doc comment explicitly names `wscompat` as an intended caller
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:279-287` — identity-attachment requirement for infra-fleet-service calls (pattern to follow)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go:12-15` — `Identity`'s fields (no connection scoping today)
- `specs/backend-go/bugs/missing-v1/BUG-004-accounts-channels-not-implemented.md` — full call-site table, dispatch-model quote
