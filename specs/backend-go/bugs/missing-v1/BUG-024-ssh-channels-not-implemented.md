# BUG-024: `ssh.*` channels not implemented in backend-go

**Service:** `api-gateway` (dispatch) — owning service is `infra-fleet-service`
**File:** `internal/adapter/wscompat/channels.go`
**Severity:** High — SSH connectivity underlies most remote-dev-server workflows; `ssh.connect`/`ssh.getState`/`ssh.listTargets` block any SSH-backed dev server from becoming usable.
**Symptom:** Every `ssh.*` `callRuntimeRpc` from the renderer falls through to `registry.go`'s `notImplementedHandler` and returns `channel "ssh.X" is not yet implemented in backend-go`.
**Status:** ❌ Open

---

## Description

`grep -n '"ssh\.' internal/adapter/wscompat/channels.go` returns **zero matches** —
none of the 4 `ssh.*` methods frontend calls are registered.

The owning service is unambiguous: `infra-fleet-service` — its proto lives at
`backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, it is already REST-wired at
`/v1/infra` (`internal/adapter/httpgateway/infra_routes.go:19-30`), and it already has
two of its RPCs (`ListDevServers`, `RegisterDevServer`) wired for real in `wscompat` via
`registerDevServerChannels` (`channels.go:390-433`) — the closest existing pattern to
follow for `ssh.*`.

Checking `infrafleet.proto`'s `InfraFleetService` (`infrafleet.proto:10-32`) for
anything SSH-target-shaped:

- `CreateSshTarget(host, user, vault_ssh_role) → ssh_target_id` exists
  (`infrafleet.proto:13,118-127`) — a **write-only** create, backed by
  `internal/usecase/create_ssh_target.go` and REST-wired at `POST
  /v1/infra/ssh-targets` (`infra_routes.go:26,149-176`).
- **No** `ListSshTargets`, `GetSshTargetState`, `GetUserAccount`, or `Connect`/`Disconnect`
  RPC exists anywhere in the proto or in `internal/usecase/` (directory listing:
  `create_connection.go`, `create_ssh_target.go`, `get_fleet_health.go`,
  `list_dev_servers.go`, `register_dev_server.go`, `relay.go`,
  `resolve_connection.go`, `scan_workspace_ports.go` — no `list_ssh_targets.go`,
  `ssh_state.go`, or `ssh_user_account.go`).

So all 4 `ssh.*` methods are missing **both** the wscompat registration **and** their
backing RPC — this is not a "channel exists, just not exposed" gap like
`devServer.*`; the read/connect surface for SSH targets doesn't exist on
`InfraFleetService` yet beyond creation.

Per `specs/frontend/api/rpc-catalog.md:426`, `ssh.getUserAccount` in the *old* TS
backend was real, sourced from the SSH target's actual configured username (no
separate "Linux account provisioning" concept) — that note describes the old
backend's semantics only; it is not evidence of any equivalent existing in
backend-go, where the RPC to source it from doesn't exist at all.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `ssh.connect` | `renderer/src/runtime/runtime-environment-ssh-state.ts`, `renderer/src/runtime/runtime-ssh-client.ts` | No `Connect`-shaped RPC on `InfraFleetService`. |
| `ssh.getState` | `renderer/src/runtime/runtime-environment-ssh-state.ts`, `renderer/src/runtime/runtime-ssh-client.ts` | No backing RPC; no `internal/usecase/*ssh_state*` file. |
| `ssh.getUserAccount` | `renderer/src/hooks/useSshUserAccount.ts` | No backing RPC. Old backend sourced this from the target's configured username — see rpc-catalog.md:426 note (old-backend semantics only). |
| `ssh.listTargets` | `renderer/src/runtime/runtime-environment-ssh-state.ts`, `renderer/src/runtime/runtime-ssh-client.ts` | Only `CreateSshTarget` exists (`infrafleet.proto:13`); no `ListSshTargets`. |

---

## Dispatch model

`specs/frontend/api/backend-agent-execution-boundary.md:112-113` documents this
namespace's dispatch precisely (old TS backend):

> `ssh.getState`/`listTargets`/... | 🏠 always-local | Pure in-memory reads
> (`SshConnectionStore`/`connectionManager`/`fleetHealthStore`), backed by the
> Postgres blob for the persisted parts (targets), but no relay ever.

> `ssh.connect`/`disconnect` | 🔌 always remote | *Is* the connection-establishment
> act itself.

So: `ssh.getState`, `ssh.listTargets`, `ssh.getUserAccount` are 🏠 always-local reads
(would read from `infra-fleet-service`'s Postgres-backed SSH target store, no relay
involved), while `ssh.connect` is 🔌 always-remote — it IS the act of establishing the
connection, via the Dev Server Agent / raw SSH relay
(`infra_routes.go`'s `handleRelay`/`client.Relay` is the closest existing plumbing for
this in backend-go today).

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:390-433` — `registerDevServerChannels`, the closest existing wiring pattern to follow (`devServer.list`/`devServer.add`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:10-32` — `InfraFleetService` RPC list (only `CreateSshTarget`, no list/state/user-account/connect)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:118-127` — `CreateSshTargetRequest`/`Response`
- `backend-go/services/infra-fleet-service/internal/usecase/` — directory listing confirms no `list_ssh_targets.go`/ssh-state/user-account usecase exists
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:19-30,149-176` — `mountInfraRoutes`, `/v1/infra/ssh-targets` (create-only)
- `specs/frontend/api/rpc-catalog.md:420-427,519` — `ssh.*` method table and `ssh.getUserAccount` note
- `specs/frontend/api/backend-agent-execution-boundary.md:112-113` — dispatch rows for `ssh.getState`/`listTargets`/`getUserAccount` (🏠) and `ssh.connect` (🔌)
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling report this follows the shape of
