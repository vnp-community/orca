# TASK-070: [BLOCKED] `host.*` per-target relay design — target design only, do not implement

**From Solution:** SOL-011 (Design — Part 2: blocked, documented for the future)
**Priority:** P3 — explicitly blocked, not schedulable
**Service:** `infra-fleet-service` (target) — **out of scope today**
**File:** none to modify now; target files listed below for whoever picks this up once unblocked
**Depends on:** blocked on an `agent/` capability (a `host.capabilities` or equivalent probe RPC) that does not exist in this repo today and is out of scope for this rewrite. Not blocked on any other TASK-0XX in this set.
**Status:** `[blocked]` Confirmed blocked on an `agent/` host-capability-
probing method that does not exist in this repo — per this task's own
instructions, no code was implemented; this file's target-design
documentation is the only deliverable, and it is unchanged from the
version reviewed in this pass.

---

## Context

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

Not applicable — no code changes ship from this task. Do not run
`go build`/`go test` against the sketches above; they reference an RPC
and agent method that do not exist in the current proto/agent surface and
will not compile.
