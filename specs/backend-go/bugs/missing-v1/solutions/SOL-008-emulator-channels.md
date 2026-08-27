# SOL-008: `emulator.*` has no backend-go-only fix — relay design blocked on an `agent/` extension that is out of scope

**Resolves:** [BUG-008](../BUG-008-emulator-channels-not-implemented.md)
**Service:** none today. The only architecturally sound design
(Dev Server Agent relay, extending `infra-fleet-service`) requires new
`agent/` capability that does not exist and is out of scope for this
rewrite — see "Out-of-scope dependency" below.
**Affected files (proposed) — immediate fix only:**
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (new `registerEmulatorChannels`, honest-stub handlers)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Affected files (future, blocked — do not start until `agent/` lands the capability below):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/usecase/*.go` (new emulator usecases)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go`
- `agent/src/relay/agent-rpc-dispatch.ts` and ADB/`xcrun simctl` driver modules (not part of `backend-go` — a different repo area, listed for completeness only)
**Status:** ✅ Implemented — all 3 task(s) (TASK-046–048) DONE; see each task file's own Status/Verify section for evidence.

---

## The TDD already answers the architecture question BUG-008 raised — this is a scope call, not a design gap

BUG-008 flagged this as an open question: does running mobile emulators on
the shared `backend-go` host make sense, and would relaying to the Dev
Server Agent (the same pattern as `git.*`/`files.*`/`terminal.*`) be the
right fix? `02-microservices-decomposition.md`'s "What's deliberately not a
separate service" section already answers this, verbatim:

> **Browser/computer/emulator automation** — per
> `backend-agent-target-architecture.md` Gap 6, this automates the backend
> host's own machine (Electron `webContents`, macOS accessibility,
> ADB/`simctl`). It has no sensible home in a stateless, horizontally-scaled
> Go microservice fleet — this is a **product decision to make before
> porting**, not a mechanical translation. Default recommendation:
> **out of scope for the Go server deployment** entirely (same conclusion
> the TS gap analysis reached for the multi-user case).
> (`specs/backend-go/tdd/architecture/02-microservices-decomposition.md:91-99`)

That confirms the backend-local port (`emulator.*` driving ADB/`simctl` on
whatever host `backend-go` happens to run on) is explicitly excluded — not
an oversight this solution should patch over with a backend-local
implementation. BUG-008's own alternative — relay to the Dev Server Agent,
the same coordination/execution split `git-gateway-service` and
`infra-fleet-service` already use — is the only design consistent with the
rest of this architecture. But it depends on capability `agent/` does not
have today.

## Out-of-scope dependency: `agent/` has no ADB/`xcrun simctl` driving surface

`infra-fleet-service.md`'s `adapter/devserveragent/methods.go` (§6, lines
335-336) lists the *specific* agent RPC methods that service's Go client
calls today: `pty.spawn/write/resize/kill`, `ports.scan`,
`preflight.check`. There is no `emulator.*`/`device.*` method anywhere in
that list, in `08-inter-service-communication.md`'s protocol discussion, or
in any other TDD service doc. Confirmed independently by BUG-008's own repo
search (no emulator/ADB/simctl logic anywhere in `backend-go/`) — and the
Dev Server Agent side of that relay is a different codebase
(`agent/src/relay/agent-rpc-dispatch.ts` and whatever device-driver module
would sit behind it), which the TDD's own migration-phase framing
(`08-inter-service-communication.md:96-102`, restated for Option A vs.
Option B of the git/infra relay protocol) treats as out of scope for "the
Go rewrite of `backend/`":

> requires changes to `agent/`, which is explicitly out of scope for "the
> Go rewrite of `backend/`" as scoped by the user's request.

That sentence was written about modernizing the *transport* (Option B). The
same boundary applies with more force here: this isn't a transport swap,
it's a wholly new capability class (interactive device control — tap,
gesture, rotate, button, lifecycle) that `agent/` has never implemented on
any transport. Adding it is real engineering in a different repo area, not
a backend-go task. **This solution does not propose starting that work** —
it documents the design so whoever picks up the `agent/` side has a target
contract, and ships the only thing backend-go can honestly ship today: a
stub that says so.

---

## Design — Part 1 (shippable now): honest stub, not a silent timeout

Today every `emulator.*` call times out against `notImplementedHandler`'s
generic "not yet implemented" message, which implies "coming soon." That's
inaccurate — per the finding above, there is no near-term path to
"implemented" without cross-repo `agent/` work nobody has scoped. Follow
SOL-002's precedent (`GET /auth/sso/:provider` → an honest, documented 501
instead of a bare 404 or misleading "not yet" message): register real
`emulator.*` handlers that return a clear, permanent, typed error rather
than falling through to the generic message.

```go
// ── emulator.* ──────────────────────────────────────────────────────────
//
// Mobile emulator/simulator control (ADB/xcrun simctl device driving) has
// no backend-go implementation and, per
// 02-microservices-decomposition.md's "What's deliberately not a separate
// service" section, is explicitly excluded from the Go server deployment
// by design — not a gap awaiting a future pass. The architecturally sound
// alternative (relay to the Dev Server Agent, see SOL-008 in
// specs/backend-go/bugs/missing-v1/solutions/) requires a new agent/
// capability that does not exist today; agent/ changes are out of scope
// for this rewrite. Every emulator.* channel below returns this same
// typed, permanent answer instead of falling through to
// notImplementedHandler's generic "not yet" wording, which would
// incorrectly imply this is only temporarily missing.
var errEmulatorNotSupported = errors.New(
	"mobile emulator control is not supported by the Go backend — " +
		"see specs/backend-go/bugs/missing-v1/solutions/SOL-008-emulator-channels.md")

func registerEmulatorChannels(r *Registry) {
	for _, channel := range []string{
		"emulator.attach", "emulator.availability", "emulator.button",
		"emulator.gesture", "emulator.listDevices", "emulator.rotate",
		"emulator.shutdown", "emulator.tap",
	} {
		r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
			return nil, errEmulatorNotSupported
		})
	}
}
```

`emulator.availability` is worth a narrower carve-out: instead of an error,
it could return the honest `{available: false, reason: "not supported"}`
shape the frontend's settings pane likely renders more gracefully than a
thrown error (mirrors `preflight.check`'s honest `installed: false` answers
rather than erroring). Flag as a frontend-contract question, not a
backend-go one — the sketch above assumes an error is acceptable; swap to a
typed `{available: false}` response if the settings UI needs a non-throwing
path.

## Design — Part 2 (blocked, documented for the future): Dev Server Agent relay

If/when `agent/` grows an ADB/`xcrun simctl` driving surface, the backend-go
side is a small, mechanical extension of `infra-fleet-service` — the same
shape `ScanWorkspacePorts` already uses (§3 of `infra-fleet-service.md`):
resolve `connectionId` via the existing provider registry, relay via
`adapter/devserveragent/methods.go`'s typed wrapper pattern, translate the
JSON-RPC response.

### Proto sketch (additive to `infrafleet.proto`)

```protobuf
// Emulator/simulator control, relayed to the Dev Server Agent — BLOCKED:
// requires a new agent/ RPC surface (device.list/attach/tap/gesture/
// button/rotate/shutdown/availability) that does not exist today. Do not
// implement server-side until that lands; sketch kept here as the target
// contract.
rpc ListEmulatorDevices(ListEmulatorDevicesRequest) returns (ListEmulatorDevicesResponse);
rpc GetEmulatorAvailability(GetEmulatorAvailabilityRequest) returns (GetEmulatorAvailabilityResponse);
rpc AttachEmulatorSession(AttachEmulatorSessionRequest) returns (EmulatorSession);
rpc SendEmulatorTap(SendEmulatorTapRequest) returns (google.protobuf.Empty);
rpc SendEmulatorGesture(SendEmulatorGestureRequest) returns (google.protobuf.Empty);
rpc SendEmulatorButton(SendEmulatorButtonRequest) returns (google.protobuf.Empty);
rpc RotateEmulator(RotateEmulatorRequest) returns (google.protobuf.Empty);
rpc ShutdownEmulator(ShutdownEmulatorRequest) returns (google.protobuf.Empty);

message EmulatorSession {
  string session_id = 1;
  string device_id = 2;
  string connection_id = 3;  // which dev server this device runs on
  string platform = 4;       // "android" | "ios"
}
```

Every RPC except `GetEmulatorAvailability` requires a `connection_id`
(resolved the same way `git-gateway-service` resolves one per worktree,
per `infra-fleet-service.md` §7's `ResolveConnection` flow) — there is
deliberately **no** local/backend-host fallback branch, unlike
`git-gateway-service`'s host-local case: driving emulators on the shared
`backend-go` host is exactly what `02-microservices-decomposition.md`
excludes. If `ResolveConnection` reports no connection for the caller's
active dev server, the honest answer is "unavailable," not "fall back to
local."

### Usecase sketch (blocked)

```go
// internal/usecase/list_emulator_devices.go — BLOCKED on agent/ device.list
func (uc *EmulatorUseCase) ListDevices(ctx context.Context, connID string) ([]domain.EmulatorDevice, error) {
    conn, err := uc.connections.Resolve(ctx, connID)
    if err != nil {
        return nil, err
    }
    if conn == nil {
        // No backend-host fallback by design — see proto sketch's note.
        return nil, usecase.ErrEmulatorRequiresDevServer
    }
    raw, err := uc.agent.Call(ctx, conn, "device.list", nil) // agent/ method TBD
    if err != nil {
        return nil, err
    }
    return decodeEmulatorDevices(raw)
}
```

### wscompat wiring (blocked)

```go
func registerEmulatorRelayChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("emulator.listDevices", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct{ ConnectionID string `json:"connectionId"` }
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListEmulatorDevices(rpcCtx, &infrafleetv1.ListEmulatorDevicesRequest{ConnectionId: in.ConnectionID})
		if err != nil {
			return nil, err
		}
		return resp.GetDevices(), nil
	})
	// attach/tap/gesture/button/rotate/shutdown follow the same
	// decode -> AttachIdentity -> rpcTimeout -> call -> translate shape.
}
```

This section is a target contract only. **Do not register these handlers
or add these RPCs until an `agent/` change (tracked outside this repo
area) actually implements the corresponding device-driving methods** —
registering them earlier would either panic on a missing gRPC method or
require stubbing `infra-fleet-service`'s side with fake data, which is
exactly the kind of fabrication `channels.go`'s own package doc warns
against (see `devServerView`'s honest-placeholder convention).

---

## Test plan

**Shippable now:**
- `channels_test.go`: one table-driven test asserting all 8 `emulator.*`
  channels return `errEmulatorNotSupported` (or the typed
  `{available:false}` shape for `emulator.availability`, if that carve-out
  is taken) rather than falling through to `notImplementedHandler`'s
  generic message — regression guard distinguishing "permanently excluded"
  from "temporarily missing."

**Blocked (write only once the `agent/` side exists):**
- `infra-fleet-service`'s usecase tests against a fake `DevServerAgentClient`
  covering: no-connection → `ErrEmulatorRequiresDevServer` (no local
  fallback), connection present → relay call issued with the right method
  name/params, agent error → propagated typed error.
- `adapter/grpc` contract tests against the new proto RPCs.
- `wscompat` handler tests mirroring `devServer.*`'s existing shape
  (identity attachment, per-RPC timeout, response translation).

## References

- `specs/backend-go/bugs/missing-v1/BUG-008-emulator-channels-not-implemented.md` — problem statement, dispatch-model question this solution answers
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:91-99` — "Browser/computer/emulator automation... out of scope for the Go server deployment entirely" — the binding answer to BUG-008's architecture question
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:84-108` — "Talking to the Dev Server Agent," Option A/B, and the "explicitly out of scope... as scoped by the user's request" framing this solution applies to the emulator-driving capability itself
- `specs/backend-go/tdd/services/infra-fleet-service.md:78-138` (API surface), `:293-348` (`adapter/devserveragent/` package, `methods.go`'s existing method list with no emulator entry), `:405-437` (`ResolveConnection` + relay dispatch flow, the pattern Part 2 reuses)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:506-544` — `preflight.check`/`crashReports.getLatestPending`'s honest-answer convention, precedent for Part 1's stub design
- `specs/backend-go/bugs/missing-v1/solutions/SOL-002-auth-sso-stub-route.md` — precedent for an honest documented stub over a bare/generic not-implemented response
