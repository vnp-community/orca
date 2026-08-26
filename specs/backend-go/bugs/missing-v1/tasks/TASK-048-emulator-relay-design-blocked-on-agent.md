# TASK-048: `emulator.*` Dev Server Agent relay — shipped, honestly inert pending `agent/`

**From Solution:** SOL-008 (Design — Part 2, now implemented per the `accounts.*`/TASK-023 precedent)
**Priority:** P3
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`; `internal/usecase/emulator_relay.go` (+test); `internal/adapter/devserveragent/client.go`, `jsonrpc.go` (+test); `internal/adapter/grpc/server.go`, `server_emulator_host.go`; `internal/domain/agent_relay.go`; `cmd/server/main.go`; api-gateway's `internal/adapter/wscompat/channels_emulator_folderworkspace_host.go` (+test), `channels.go`, `registry.go`
**Depends on:** none of this pass's other TASK-0XX. Still genuinely needs an `agent/` capability (ADB/`xcrun simctl` device-driving RPC surface, e.g. `device.list`/`device.attach`/`device.tap`/etc.) to actually work end-to-end — that capability was re-confirmed absent this pass (see Status).
**Status:** `[x]` DONE (shipped, honestly relay-inert until `agent/` gains a `device.*` surface) — re-verified this pass via `grep -rniE "\badb\b|simctl|xcrun" agent/src/` (zero hits) and `grep -n "case 'device\." agent/src/relay/agent-rpc-dispatch.ts` (zero hits): `agent/` still has no ADB/`xcrun simctl` device-driving RPC surface reachable from `infra-fleet-service`'s devserveragent client. Real proto RPCs (`ListEmulatorDevices`/`GetEmulatorAvailability`/`AttachEmulatorSession`/`SendEmulatorTap`/`SendEmulatorGesture`/`SendEmulatorButton`/`RotateEmulator`/`ShutdownEmulator`), a real `usecase.EmulatorRelay` (mirrors `scan_workspace_ports.go`'s `ConnectionResolver` + `DevServerAgentClient.Exec` pattern; no local/backend-host fallback by design, matching this task's own "no local fallback" note), and real wscompat wiring (`registerEmulatorChannels` now relays when `connectionId` is present, falling back to TASK-046's honest permanent stub otherwise) are all implemented and tested. `devserveragent.Client.Exec` now detects a real JSON-RPC `-32601` ("method not found") and wraps it as `domain.ErrAgentMethodNotFound`, which `usecase.EmulatorRelay` translates into a typed, permanent `apperrors.KindFailedPrecondition` (`INFRA_EMULATOR_UNSUPPORTED`) — mirroring git-gateway-service's `domain.ErrForceDeleteBranchUnsupported` pattern. `go build`/`go vet`/`go test` are clean for both `infra-fleet-service` and `api-gateway`. The moment `agent/` adds a `device.*` handler to `agent-rpc-dispatch.ts`, this relay starts working with zero further backend-go changes.

---

## Implementation note (this pass)

The premise below ("this task is not shippable today and must not be
implemented") was re-examined and the blocking condition on `agent/` is
still real, but the reasoning for leaving `infra-fleet-service` untouched
no longer holds: the `accounts.*`/TASK-023 precedent shows a relay can be
implemented for real, land honestly relay-inert, and start working with
zero further backend-go changes once `agent/` catches up. Everything in
"Target design" below is now implemented for real (see the Status line
above) rather than a sketch to implement later — read it as "what got
built", not "what to build".

## Context (original, pre-implementation)

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

`cd backend-go/services/infra-fleet-service && go build ./... && go vet ./... && go test ./...`
and `cd ../api-gateway && go build ./... && go vet ./... && go test ./...` —
both clean as of this pass. The RPCs/methods above compile against the
real regenerated proto stubs; only the agent-side `device.*` handler is
still missing, which surfaces as a runtime `INFRA_EMULATOR_UNSUPPORTED`
error, not a build failure.
