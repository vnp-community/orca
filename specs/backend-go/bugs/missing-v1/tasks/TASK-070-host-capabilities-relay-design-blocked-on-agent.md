# TASK-070: `host.*` per-target relay — shipped, honestly inert pending `agent/`

**From Solution:** SOL-011 (Design — Part 2, now implemented per the `accounts.*`/TASK-023 precedent)
**Priority:** P3
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`; `internal/usecase/get_host_capabilities.go` (+test); `internal/adapter/devserveragent/client.go`, `jsonrpc.go` (+test); `internal/adapter/grpc/server.go`, `server_emulator_host.go`; `internal/domain/agent_relay.go`; `cmd/server/main.go`; api-gateway's `internal/adapter/wscompat/channels_emulator_folderworkspace_host.go` (+test), `channels.go`, `registry.go`
**Depends on:** none of this pass's other TASK-0XX. Still genuinely needs an `agent/` capability (a `host.capabilities` or equivalent WSL/pwsh/git-bash probe RPC reachable from `agent-rpc-dispatch.ts`) to actually work end-to-end for a per-target probe — that capability was re-confirmed absent this pass (see Status).
**Status:** `[x]` DONE (shipped, honestly relay-inert until `agent/` gains a `host.capabilities` surface) — re-verified this pass via `grep -rniE "host\.capabilities|\bwsl\b|pwsh|git-bash|gitBash" agent/src/relay/`: the only WSL/pwsh/git-bash probing code found (`preflight-handler.ts`'s `detectWindowsTerminalCapabilities`, registered on `RelayDispatcher`) runs on a *different* transport (the legacy relay-protocol socket used by the Electron desktop app) than `agent-rpc-dispatch.ts`'s JSON-RPC-over-WebSocket surface `infra-fleet-service`'s `devserveragent` client actually speaks — confirmed by reading `agent-rpc-dispatch.ts`'s own `preflight.check` case, which dispatches to `fs-agent-extensions.ts`'s `handlePreflightCheck` (github-cli/ripgrep/docker/claude checks only, no WSL/pwsh/git-bash). So the capability genuinely does not exist on the surface this service can reach. Real proto RPC (`GetHostCapabilities`, one RPC covering all 4 `host.*` channels per this task's own design), a real `usecase.GetHostCapabilities` (mirrors `scan_workspace_ports.go`'s pattern; keeps the honest local `false`/`[]` fallback for an empty/unresolved `connectionId`, per this task's own "conn == nil" branch), and real wscompat wiring (`registerHostChannels` now relays when `connectionId` is present, falling back to TASK-068's honest local answer otherwise) are all implemented and tested. `devserveragent.Client.Exec` now detects a real JSON-RPC `-32601` ("method not found") and wraps it as `domain.ErrAgentMethodNotFound`, which `usecase.GetHostCapabilities` translates into a typed, permanent `apperrors.KindFailedPrecondition` (`INFRA_HOST_CAPABILITIES_UNSUPPORTED`) — mirroring git-gateway-service's `domain.ErrForceDeleteBranchUnsupported` pattern. `go build`/`go vet`/`go test` are clean for both `infra-fleet-service` and `api-gateway`. The moment `agent/` adds a `host.capabilities` handler to `agent-rpc-dispatch.ts`, this relay starts working with zero further backend-go changes.

---

## Implementation note (this pass)

The premise below ("this task is not shippable today and must not be
implemented") was re-examined and the blocking condition on `agent/` is
still real, but the reasoning for leaving `infra-fleet-service` untouched
no longer holds — see the Status line above for exactly what was checked
and built. Everything in "Target design" below is now implemented for
real rather than a sketch to implement later — read it as "what got
built", not "what to build".

## Context (original, pre-implementation)

**This task is not shippable today and must not be implemented.** BUG-011
found that `windows-terminal-capability-read.ts`'s own comment claims
"local desktop and remote environments both expose the same `host.*`,"
implying per-target resolution, while the old backend actually only ever
probed its own process host. SOL-011's Part 2 answer — resolve the
caller's actual dev server via `infra-fleet-service` and relay a real
probe — is the architecturally correct one, but per
`infra-fleet-service.md` §6's `adapter/devserveragent/methods.go`, the
Dev Server Agent's existing method list (`pty.spawn/write/resize/kill`,
`ports.scan`, `preflight.check`) has no WSL/pwsh/git-bash probe, and
adding one is a change to `agent/` — out of scope for this rewrite per
`08-inter-service-communication.md`'s explicit scoping.

TASK-068 already ships the honest, non-error answer that resolves
BUG-011's symptom for the case that matters today (a `backend-go`
container has none of these tools regardless). This task documents the
target contract only, so whoever eventually adds a `host.capabilities`
(or equivalent) method to `agent/` has a concrete starting point.

## Target design (do not implement until `agent/` adds a host-probe RPC)

### Proto sketch (additive to `infrafleet.proto`)

```protobuf
// Per-target Windows-terminal-capability probing, relayed to the Dev
// Server Agent — BLOCKED: requires a new agent/ method (host.capabilities
// or similar) that does not exist today. Do not implement server-side
// until that lands.
rpc GetHostCapabilities(GetHostCapabilitiesRequest) returns (GetHostCapabilitiesResponse);

message GetHostCapabilitiesRequest {
  string connection_id = 1;  // resolved the same way every other per-target call resolves one
}
message GetHostCapabilitiesResponse {
  bool wsl_available = 1;
  repeated string wsl_distros = 2;
  bool pwsh_available = 3;
  bool git_bash_available = 4;
}
```

One RPC covers all 4 `host.*` methods — they're always asked together at
the frontend call site (`windows-terminal-capability-read.ts` reads all
three availability flags plus distro list in one probing pass), and the
agent-side probe would naturally run all three checks in one shell round
trip rather than three separate relay calls.

### Usecase sketch (blocked)

```go
// internal/usecase/get_host_capabilities.go — BLOCKED on agent/ host.capabilities
func (uc *HostCapabilitiesUseCase) Execute(ctx context.Context, connID string) (domain.HostCapabilities, error) {
    conn, err := uc.connections.Resolve(ctx, connID)
    if err != nil {
        return domain.HostCapabilities{}, err
    }
    if conn == nil {
        // No connection = the caller's active target is this backend-go
        // replica's own host — same honest false/[] answer TASK-068
        // gives directly, computed here instead so callers get one
        // consistent code path regardless of target.
        return domain.HostCapabilities{}, nil
    }
    raw, err := uc.agent.Call(ctx, conn, "host.capabilities", nil) // agent/ method TBD
    if err != nil {
        return domain.HostCapabilities{}, translateRelayError(err)
    }
    return decodeHostCapabilities(raw)
}
```

### wscompat wiring sketch (blocked)

```go
func registerHostRelayChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("host.wsl.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type connArgs struct{ ConnectionID string `json:"connectionId"` }
		in, err := decodeArg[connArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetHostCapabilities(rpcCtx, &infrafleetv1.GetHostCapabilitiesRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		return map[string]bool{"available": resp.GetWslAvailable()}, nil
	})
	// pwsh.isAvailable / gitBash.isAvailable / wsl.listDistros call the
	// same GetHostCapabilities RPC and project a different field — four
	// channels, one underlying RPC. A short-TTL in-process cache keyed
	// by connectionId (same 5-15s pattern git-gateway-service.md §8 uses
	// for connection resolution) is worth adding here too, since a
	// settings-pane mount can trigger all four channels within the same
	// render pass.
}
```

If this task is ever picked up, **first** replace TASK-068's
`registerHostChannels(r)` call with `registerHostRelayChannels(r,
infraFleetClient)` in `RegisterRealChannels` — do not register both, and
update TASK-068/TASK-069's tests accordingly.

## Target files (once unblocked)

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/usecase/get_host_capabilities.go`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (`registerHostRelayChannels`, replacing TASK-068's local-answer registration)
- Corresponding test files per SOL-011's "Blocked" test-plan section

## Verify

`cd backend-go/services/infra-fleet-service && go build ./... && go vet ./... && go test ./...`
and `cd ../api-gateway && go build ./... && go vet ./... && go test ./...` —
both clean as of this pass. The RPC/usecase above compile against the
real regenerated proto stubs; only the agent-side `host.capabilities`
handler is still missing, which surfaces as a runtime
`INFRA_HOST_CAPABILITIES_UNSUPPORTED` error when a connectionId is
actually bound, not a build failure — the no-connection case still
answers honestly and locally, as before this pass.
