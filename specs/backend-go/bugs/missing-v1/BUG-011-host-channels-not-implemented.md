# BUG-011: `host.*` channels not implemented in backend-go

**Service:** none — `infra-fleet-service` is the closest candidate (owns Dev Server Agent capability/fleet operations) but has no host-capability-probe RPCs
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Low — Windows-terminal-capability probing (WSL/PowerShell/git-bash availability) is a niche, platform-specific concern, not part of core daily workflows.
**Symptom:** Every `host.*` call from `windows-terminal-capability-read.ts` times out with `channel "host.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** ❌ Open

---

## Description

None of the 4 `host.*` methods the frontend calls are registered in
`wscompat.Registry`. Confirmed via:

```
$ grep -n '"host\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

`infra-fleet-service` is the closest candidate by domain — it's the service
that already deals with Dev Server Agent connectivity and fleet health
(`RegisterDevServer`, `ResolveConnection`, `CreateSshTarget`,
`GetFleetHealth`, `ScanWorkspacePorts`, `ListDevServers`, `CreateConnection`,
`Relay` — `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:11-31`), but
none of its RPCs or usecases (`backend-go/services/infra-fleet-service/internal/usecase/`:
`create_connection.go`, `create_ssh_target.go`, `get_fleet_health.go`,
`list_dev_servers.go`, `register_dev_server.go`, `relay.go`,
`resolve_connection.go`, `scan_workspace_ports.go`) touch WSL/PowerShell/
git-bash capability probing. A repo-wide search for `wsl`/`pwsh`/`gitbash`
across `backend-go/proto/` and every service's `internal/usecase/` turns up
nothing. This is a **service-doesn't-have-this-capability gap**, not a
missing wscompat handler over an existing method.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `host.gitBash.isAvailable` | `frontend/src/renderer/src/lib/windows-terminal-capability-read.ts:55` | |
| `host.pwsh.isAvailable` | `frontend/src/renderer/src/lib/windows-terminal-capability-read.ts:52` | |
| `host.wsl.isAvailable` | `frontend/src/renderer/src/lib/windows-terminal-capability-read.ts:46` | |
| `host.wsl.listDistros` | `frontend/src/renderer/src/lib/windows-terminal-capability-read.ts:49` | |

None of these are registered anywhere in `channels.go`, confirmed by the grep
above — this is a full-namespace gap.

---

## Dispatch model

Per `specs/frontend/api/backend-agent-execution-boundary.md:167`, the old
TypeScript backend ran this 🏠 **backend-local**: `host-capabilities.ts`
probes the **backend host machine itself** (`process.platform`,
WSL/PowerShell/git-bash availability) — not per-worktree, no relay to the
Dev Server Agent, no Postgres.

Note the frontend call site's own comment
(`frontend/src/renderer/src/lib/windows-terminal-capability-read.ts:39`)
that "local desktop and remote environments both expose the same `host.*`"
surface — i.e. the frontend expects this to resolve per active runtime
target, even though the old backend's actual implementation only ever
probed its own host process, not a remote one. Whoever implements this in
backend-go should decide whether to preserve that backend-local-only
behavior (simplest, matches old backend) or extend it to probe the actual
target host (e.g. via `infra-fleet-service`'s Dev Server Agent relay) to
honestly answer the question for remote/SSH targets too — worth flagging
alongside BUG-008's similar architecture question, since both involve
probing/driving something on "a host" without today's backend-go having a
clear per-tenant host to act on.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `host.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:11-31` — closest candidate service's existing (non-host-capability) RPCs
- `backend-go/services/infra-fleet-service/internal/usecase/` — no WSL/pwsh/git-bash usecases
- `specs/frontend/api/backend-agent-execution-boundary.md:167` — `host.*` 🏠 dispatch classification
- `specs/frontend/api/rpc-catalog.md:278-281` — `host.*` catalog entries
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling bug report this follows the format of
