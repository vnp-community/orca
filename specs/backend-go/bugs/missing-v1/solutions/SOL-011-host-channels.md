# SOL-011: `host.*` — honest local answer now, per-target relay via `infra-fleet-service` once `agent/` can answer it

**Resolves:** [BUG-011](../BUG-011-host-channels-not-implemented.md)
**Service:** `infra-fleet-service` (extended) + `api-gateway`. The
per-target answer depends on an `agent/` capability that does not exist
today — see "Out-of-scope dependency" below; the immediate fix does not.
**Affected files (proposed) — immediate fix:**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerHostChannels`, local-only honest answers)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Affected files (future, blocked — do not start until `agent/` lands the capability below):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/usecase/get_host_capabilities.go`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go`
**Status:** ✅ Implemented — all 3 task(s) (TASK-068–070) DONE; see each task file's own Status/Verify section for evidence.
per-target design is documented but blocked.

---

## Two questions BUG-011 raised, two different answers

BUG-011 flagged that `windows-terminal-capability-read.ts`'s own comment
claims "local desktop and remote environments both expose the same
`host.*`" — implying per-target resolution — while the old backend actually
only ever probed its own process host. It asked whoever picks this up to
choose between preserving backend-local-only behavior or extending to
probe the real target host via `infra-fleet-service`'s relay.

**Both, at different time horizons**, following the same shape SOL-008
uses for `emulator.*`:

1. **Now**: `backend-go` has no per-tenant host of its own worth probing —
   WSL/PowerShell/git-bash are Windows-desktop concepts; a `backend-go`
   replica runs in a Linux container per this system's deployment model
   (`10-deployment-infrastructure.md`), so "probe the backend host" would
   answer `false`/`[]` unconditionally regardless of implementation effort.
   That's not a stub to apologize for — it's the honest answer, the same
   way `preflight.check`'s `gh`/`glab` fields are honestly `false` because
   `scm-integration-service` isn't a CLI wrapper (`channels.go`'s own
   precedent). Ship this now: zero new services, zero proto changes.
2. **Later, blocked**: per-target resolution (does *this worktree's* dev
   server have WSL/pwsh/git-bash) is the answer that actually matches the
   frontend's stated intent, and BUG-011's own framing is right that
   `infra-fleet-service` is the natural home — it already resolves
   "which host does this belong to" for every other per-target capability
   question in the system. But it requires a new Dev Server Agent method
   that doesn't exist today (see below), so it can't ship in this pass.

---

## Design — Part 1 (shippable now): honest local answer, no relay

```go
// ── host.* ──────────────────────────────────────────────────────────
//
// WSL/PowerShell/git-bash availability on the *backend-go host itself* —
// per BUG-011, the old backend probed only its own process host, never a
// per-target dev server. backend-go's own host is a Linux container
// (10-deployment-infrastructure.md's deployment model) with none of these
// three tools meaningful on it, so "false"/"[]" is the honest answer here,
// not a placeholder — same posture as preflight.check's honest gh/glab
// false answers below. Per-target (does the CALLER'S ACTIVE DEV SERVER
// have these) is a distinct, more useful question — see SOL-011 in
// specs/backend-go/bugs/missing-v1/solutions/ for that design, which is
// blocked on an agent/ capability that doesn't exist yet.
func registerHostChannels(r *Registry) {
	r.Register("host.wsl.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
	r.Register("host.wsl.listDistros", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return []string{}, nil
	})
	r.Register("host.pwsh.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
	r.Register("host.gitBash.isAvailable", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"available": false}, nil
	})
}
```

This is a direct, complete fix for the literal `host.*` namespace as it
exists today — every channel gets a real, typed, honest response instead
of a timeout. It resolves BUG-011's symptom outright. Part 2 below is a
*better* answer to the underlying product need, not a requirement for
closing this bug.

---

## Design — Part 2 (blocked, documented for the future): per-target relay

### Out-of-scope dependency

Same shape as SOL-008's finding: `infra-fleet-service.md` §6's
`adapter/devserveragent/methods.go` lists the exact agent methods this
service's Go client calls today — `pty.spawn/write/resize/kill`,
`ports.scan`, `preflight.check`. No WSL/pwsh/git-bash probe exists in that
list, and BUG-011's own repo-wide search confirms no such logic exists
anywhere in `backend-go/` either. Answering "does this dev server have WSL"
requires the Dev Server Agent to run the actual probe (`wsl.exe -l -q`,
`pwsh -Version`, locating `bash.exe`) on the target host and report back —
capability `agent/` does not expose today.

This is a **smaller** ask than SOL-008's ADB/`simctl` device-driving
surface — no interactive control, just three cheap read-only shell probes,
conceptually adjacent to `preflight.check`'s existing CLI-presence probing
(same shape: "is this binary on PATH / does this tool report a version").
An agent maintainer could plausibly add a `host.capabilities` method to the
same dispatcher `preflight.check` already lives on with low risk. But it is
still a change to `agent/`, and per this rewrite's own explicit boundary
(`08-inter-service-communication.md`'s "requires changes to `agent/`, which
is explicitly out of scope... as scoped by the user's request," the same
framing BUG-008/SOL-008 apply), this solution does not propose starting
that work — it documents the target contract only.

### Proto sketch (additive to `infrafleet.proto`, blocked)

```protobuf
// Per-target Windows-terminal-capability probing, relayed to the Dev
// Server Agent — BLOCKED: requires a new agent/ method (host.capabilities
// or similar) that does not exist today. See SOL-011's "Out-of-scope
// dependency" note. Do not implement server-side until that lands.
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
three availability flags plus distro list in one probing pass) and the
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
        // replica's own host — same honest false/[] answer Part 1 gives
        // directly, computed here instead so callers get one consistent
        // code path regardless of target.
        return domain.HostCapabilities{}, nil
    }
    raw, err := uc.agent.Call(ctx, conn, "host.capabilities", nil) // agent/ method TBD
    if err != nil {
        return domain.HostCapabilities{}, translateRelayError(err)
    }
    return decodeHostCapabilities(raw)
}
```

### wscompat wiring (blocked)

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
	// same GetHostCapabilities RPC and project a different field —
	// four channels, one underlying RPC, matching the "ask together"
	// rationale above. A short-TTL in-process cache keyed by connectionId
	// (same 5-15s pattern git-gateway-service.md §8 uses for connection
	// resolution) is worth adding here too, since a settings-pane mount
	// can trigger all four channels within the same render pass.
}
```

This section is a target contract only, matching SOL-008's precedent —
register nothing here until `agent/` actually implements
`host.capabilities` (or equivalent) on at least one transport mode.

---

## Test plan

**Shippable now:**
- `channels_test.go` — table-driven test asserting all 4 `host.*` channels
  return the honest `false`/`[]` shape (not an error, not
  `notImplementedHandler`'s generic message) with zero downstream calls.

**Blocked (write only once the `agent/` side exists):**
- `infra-fleet-service`'s usecase test against a fake `DevServerAgentClient`:
  no connection → honest zero-value `HostCapabilities` without calling the
  agent; connection present → relay call issued with `host.capabilities`;
  agent error → propagated typed error.
- `adapter/grpc` contract test against the new proto RPC.
- `wscompat` handler tests, one per channel, asserting each projects the
  correct field off the shared `GetHostCapabilitiesResponse`.

## References

- `specs/backend-go/bugs/missing-v1/BUG-011-host-channels-not-implemented.md` — problem statement, the per-target-vs-backend-local framing this solution resolves in two parts
- `specs/backend-go/tdd/services/infra-fleet-service.md:78-138` (§3 API surface, `ResolveConnection` as the pattern `GetHostCapabilities` would reuse), `:293-348` (§6, `adapter/devserveragent/methods.go`'s existing method list — no host-capability entry), `:405-437` (§7, `ResolveConnection` + relay dispatch flow)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` — Dev Server Agent relay boundary and the "explicitly out of scope... as scoped by the user's request" framing applied to the blocked `agent/` dependency
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:506-528` — `preflight.check`'s honest-false-answer precedent this solution's Part 1 follows directly
- `specs/backend-go/bugs/missing-v1/solutions/SOL-008-emulator-channels.md` — sibling solution this one's two-part (honest-now / blocked-relay-later) structure mirrors
