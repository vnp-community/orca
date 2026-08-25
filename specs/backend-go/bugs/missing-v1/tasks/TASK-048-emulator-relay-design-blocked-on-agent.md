# TASK-048: [BLOCKED] `emulator.*` Dev Server Agent relay — target design only, do not implement

**From Solution:** SOL-008 (Design — Part 2: blocked, documented for the future)
**Priority:** P3 — explicitly blocked, not schedulable
**Service:** `infra-fleet-service` (target) — **out of scope today**
**File:** none to modify now; target files listed below for whoever picks this up once unblocked
**Depends on:** blocked on an `agent/` capability (ADB/`xcrun simctl` device-driving RPC surface) that does not exist in this repo today and is out of scope for this rewrite. Not blocked on any other TASK-0XX in this set.
**Status:** `[ ]` TODO — **DO NOT START.** This task exists only to record the target contract for a future pass; there is no buildable work here until `agent/` gains a `device.*` RPC surface.

---

## Context

**This task is not shippable today and must not be implemented.** BUG-008
asked whether `emulator.*` should relay to the Dev Server Agent (the same
coordination/execution split `git-gateway-service` and
`infra-fleet-service` already use for `git.*`/`files.*`). SOL-008's answer
is yes, architecturally — but `infra-fleet-service.md` §6's
`adapter/devserveragent/methods.go` lists the exact agent RPC methods this
service's Go client calls today (`pty.spawn/write/resize/kill`,
`ports.scan`, `preflight.check`), and none of them touch ADB/`xcrun
simctl`. The Dev Server Agent side of this relay
(`agent/src/relay/agent-rpc-dispatch.ts` plus whatever device-driver
module would sit behind it) is a different codebase area that this
rewrite's own scoping (`08-inter-service-communication.md`: "requires
changes to `agent/`, which is explicitly out of scope... as scoped by the
user's request") excludes.

TASK-046 already ships the honest stub that resolves BUG-008's symptom
(a timeout) with a clear, typed, permanent error. This task documents the
target contract only, so whoever eventually picks up the `agent/` side has
a concrete starting point — registering any of the code below before that
`agent/` work lands would either panic on a missing gRPC method or force
`infra-fleet-service` to fabricate fake data, which SOL-008 explicitly
rules out.

## Target design (do not implement until `agent/` adds a `device.*` RPC surface)

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

Every RPC except `GetEmulatorAvailability` requires a `connection_id`.
There is deliberately **no** local/backend-host fallback branch — driving
emulators on the shared `backend-go` host is exactly what
`02-microservices-decomposition.md` excludes. If `ResolveConnection`
reports no connection for the caller's active dev server, the honest
answer is "unavailable," not "fall back to local."

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

### wscompat wiring sketch (blocked)

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

If this task is ever picked up, **first** replace TASK-046's
`registerEmulatorChannels(r)` call with `registerEmulatorRelayChannels(r,
infraFleetClient)` in `RegisterRealChannels` — do not register both, and
do not leave the honest-stub error path silently shadowed without
updating TASK-046/TASK-047's tests accordingly.

## Target files (once unblocked)

- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
- `backend-go/services/infra-fleet-service/internal/usecase/*.go` (new emulator usecases)
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (`registerEmulatorRelayChannels`, replacing TASK-046's stub registration)
- Corresponding test files per SOL-008's "Blocked" test-plan section

## Verify

Not applicable — no code changes ship from this task. Do not run
`go build`/`go test` against the sketches above; they reference RPCs and
methods that do not exist in the current proto/agent surface and will not
compile.
