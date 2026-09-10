# BUG-027: `workspacePorts.*` channels not implemented in backend-go

**Service:** `api-gateway` (dispatch) — owning service is `infra-fleet-service`
**File:** `internal/adapter/wscompat/channels.go`
**Severity:** Medium — port scan/kill is used to detect and free stuck dev-server ports in a workspace.
**Symptom:** `workspacePorts.scan`/`workspacePorts.kill` `callRuntimeRpc` calls fall through to `registry.go`'s `notImplementedHandler` and return `channel "workspacePorts.X" is not yet implemented in backend-go`.
**Status:** ✅ Resolved — see TASK-169–173 (5 task(s), all DONE) for implementation evidence.

---

## Description

`grep -n '"workspacePorts\.' internal/adapter/wscompat/channels.go` returns **zero
matches** — neither `workspacePorts.*` method is registered, even though the owning
service, `infra-fleet-service`, is already fully wired into the gateway's gRPC dial
set and REST layer (`registerDevServerChannels`/`registerFleetChannels`,
`channels.go:390-504`, and `mountInfraRoutes`, `infra_routes.go:19-30`).

**`workspacePorts.scan`** has a real, already-implemented backing RPC:
`InfraFleetService.ScanWorkspacePorts` (`infrafleet.proto:15,146-153`), backed by
`internal/usecase/scan_workspace_ports.go`, and already REST-proxied at `POST
/v1/infra/workspaces/scan-ports` (`infra_routes.go:28,202-227`). It is simply not
exposed as a `wscompat` WS channel — the same "REST-wired but not WS-wired" pattern
as the rest of this audit, not a missing-RPC gap.

**`workspacePorts.kill` has no backing RPC at all** — `grep -rn "[Kk]ill"` across
`internal/usecase/`, `infrafleet.proto`, and `infra_routes.go` returns zero matches.
Killing a process bound to an open port has no usecase, proto RPC, or REST route
anywhere in `infra-fleet-service` today.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `workspacePorts.kill` | `renderer/src/lib/workspace-port-actions.ts` | No backing RPC/usecase/REST route anywhere in `infra-fleet-service` — needs new work, not just a wscompat registration. |
| `workspacePorts.scan` | `renderer/src/lib/workspace-port-actions.ts` | Backing RPC exists and is already REST-wired (`InfraFleetService.ScanWorkspacePorts`, `infra_routes.go:28`) — just needs a `wscompat` registration, following `registerDevServerChannels`'s pattern (`channels.go:390-433`). |

---

## Dispatch model

Old TS backend, per `specs/frontend/api/backend-agent-execution-boundary.md:118`:

> `workspacePorts.scan`/`kill` | 🏠 always-local | ⚠️ **Silently excludes
> remote-connected repos** rather than relaying or erroring — any worktree whose
> repo has a `connectionId` is filtered out before scanning, so scanning a
> Dev-Server-hosted repo just returns an empty port list.

(Also called out as a standalone known gap at `backend-agent-execution-boundary.md:179`:
"5. **Silent capability gap**: `workspacePorts.scan`/`kill` silently return empty
results for remote-connected repos instead of relaying or erroring.")

⚠️ **Do not just port this gap forward.** backend-go's `ScanWorkspacePorts` usecase
has already deliberately fixed it — its own doc comment states it "closes TS Gap 7"
(`scan_workspace_ports.go:17-24`): it always resolves the connection first, and
whenever a `connectionId` is bound to a live dev server it relays the scan to the
agent's `ports.scan` RPC instead of silently returning empty
(`scan_workspace_ports.go:40-55`); only a genuinely unconnected (local) worktree
returns `[]` (`scan_workspace_ports.go:58-62`, itself scoped as "routing, not
executing" — actual local port-scanning is a separately tracked gap in this
service's README, not this bug). So `workspacePorts.scan`'s dispatch model in
backend-go, once wired to `wscompat`, is already 🔀 dynamic (relay when connected,
local no-op stub otherwise) — a strict improvement over the old backend, not a
reproduction of its bug.

`workspacePorts.kill` has no implementation to inherit a dispatch model from at all;
implementers should decide its dispatch (relay a kill command to the Dev Server
Agent vs. local-only) from scratch rather than assuming it mirrors `scan`'s.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:390-433` — `registerDevServerChannels`, wiring pattern to follow for `workspacePorts.scan`
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:15,146-153` — `ScanWorkspacePorts` RPC (no `Kill`-equivalent RPC anywhere in this proto)
- `backend-go/services/infra-fleet-service/internal/usecase/scan_workspace_ports.go:17-62` — usecase doc comment ("closes TS Gap 7") and relay-vs-local branch
- `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go:28,202-227` — `POST /v1/infra/workspaces/scan-ports` REST proxy (already wired)
- `specs/frontend/api/rpc-catalog.md:67,487-492` — `workspacePorts.*` catalog entry and method table
- `specs/frontend/api/backend-agent-execution-boundary.md:118,179` — old-backend 🏠 dispatch row and the standalone "silent capability gap" callout
- `specs/backend-go/bugs/api-v1/BUG-002-missing-channel-registrations.md` — sibling report this follows the shape of
