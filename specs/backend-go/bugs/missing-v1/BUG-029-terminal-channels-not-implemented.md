# BUG-029: `terminal.*` channels not implemented in backend-go

**Service:** `infra-fleet-service` (via `api-gateway`)
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Severity:** High — terminal/PTY sessions are core to agent-driven workflows; this is the same PTY subsystem this repo's current branch (`fix/pty-session-expired-on-pane-remount`) is patching on the TS side
**Symptom:** Every `terminal.*` call (create/send/close/list/focus/stop/wait/agentStatus/isRunningAgent/inspectProcess) falls through to `notImplementedHandler` and times out client-side — no agent-driven terminal work is possible against backend-go today
**Status:** ✅ Resolved — see TASK-180–187 (8 task(s), all DONE) for implementation evidence.

---

## Description

`specs/frontend/api/rpc-catalog.md` lists 10 `terminal.*` methods:

```
grep -n '"terminal\.' services/api-gateway/internal/adapter/wscompat/channels.go
```

returns **zero matches** — no `terminal.*` channel is registered in
`RegisterRealChannels` (`channels.go:79-89`). Every call reaches
`registry.go`'s `notImplementedHandler` (`registry.go:59`).

There is **no PTY-shaped RPC anywhere in backend-go's proto surface**.
`infrafleet.proto` (`proto/orca/infrafleet/v1/infrafleet.proto:10-31`) exposes
`RegisterDevServer`/`ResolveConnection`/`CreateSshTarget`/`GetFleetHealth`/
`ScanWorkspacePorts`/`ListDevServers`/`CreateConnection`/`Relay` — no
`CreateSession`/`SendInput`/`CloseSession`/`ListSessions` or anything
PTY-specific.

The one piece of existing plumbing worth citing: `infrafleet.proto`'s
`Relay` RPC (`infrafleet.proto:24-31`) is explicitly documented as "the
generic `connectionId`+`method`+`params` passthrough onto the Dev Server
Agent execution plane" and its own doc comment names `wscompat's devServer.*
/fleet.* channels` as an intended caller — i.e. the transport backend-go
would use to reach a PTY session on the Dev Server Agent already exists,
generically, even though no `terminal.*`-specific RPC or wscompat wrapper
sits on top of it yet.

`internal/adapter/devserveragent/` (`session.go`, `jsonrpc.go`, `client.go`,
`transport.go`) is infra-fleet-service's client for talking to the Dev
Server Agent over the JSON-RPC-over-websocket-frame protocol — this is the
Go-side counterpart of the TS agent bundle's relay, and is PTY-adjacent
infrastructure (session framing, keepalive, incremental decode) but carries
no PTY session lifecycle methods itself; `grep -n "pty\|Pty\|PTY"` across
that directory only matches a `BUG-FE-PTY-001 fix comment` doc-reference in
`session.go:152`, not an actual PTY implementation.

On the agent side, `agent/src/relay/` already has a full PTY subsystem —
`pty-handler.ts`, `pty-daemon-server.ts`, `pty-daemon-client.ts`,
`pty-daemon-protocol.ts`, `pty-agent-bridge.ts`, `pty-shell-launch.ts`,
`pty-shell-utils.ts` — plus, on this branch,
`agent/src/relay/agent-connection-stdio.ts` (new file, uncommitted) for the
SSH-deployed stdio transport variant. This confirms the PTY runtime already
exists on the agent side that infra-fleet-service relays to; what's missing
is backend-go's own gRPC/wscompat surface for driving it, not the underlying
PTY execution capability.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `terminal.create` | `renderer/src/lib/launch-agent-background-session.ts` | No backing RPC. Would need a new infra-fleet-service RPC (e.g. `CreateSession`) relayed to the Dev Server Agent's `pty-handler.ts`, or dispatched generically through the existing `Relay` RPC. |
| `terminal.close` | `renderer/src/lib/launch-agent-background-session.ts` | No backing RPC. |
| `terminal.send` | `renderer/src/lib/active-agent-note-send.ts`, `renderer/src/runtime/runtime-terminal-inspection.ts` | No backing RPC. PTY input write. |
| `terminal.stop` | `renderer/src/store/slices/repos.ts` | No backing RPC. |
| `terminal.wait` | `renderer/src/lib/active-agent-note-send.ts`, `renderer/src/lib/automation-session-observer.ts`, `renderer/src/lib/launch-agent-background-session.ts` | No backing RPC. |
| `terminal.list` | `renderer/src/lib/active-agent-note-target.ts` | No backing RPC. |
| `terminal.focus` | `renderer/src/components/terminal-pane/terminal-handle-links.ts`, `terminal-orchestration-task-links.ts` | No backing RPC. |
| `terminal.agentStatus` | `renderer/src/lib/active-agent-terminal-send-readiness.ts` | No backing RPC. |
| `terminal.isRunningAgent` | `renderer/src/lib/active-agent-note-target.ts`, `active-agent-terminal-send-readiness.ts` | No backing RPC. |
| `terminal.inspectProcess` | `renderer/src/runtime/runtime-terminal-inspection.ts` | No backing RPC. |

All 10 methods have zero backing RPC in backend-go. None can be wired as a
thin wscompat wrapper today — this whole namespace needs new
infra-fleet-service RPCs (or a generic passthrough through the existing
`Relay` RPC) before any wscompat registration is possible.

---

## Dispatch model

🔀 (Electron) / 🔌 (server) in the old TS backend — in desktop deployment,
an unset `connectionId` meant a real local `node-pty`; a set `connectionId`
routed through `SshPtyProvider`/`DevServerPtyProvider`. In the pure-Node
multi-user SERVER deployment (the one backend-go replaces), there is **no
local-shell path at all**: `backend/src/main/runtime/server-pty-controller.ts`
throws immediately when `connectionId` is missing —

```
'This server only supports Dev Server- or SSH-backed terminals (no local shell).'
```

(`server-pty-controller.ts:66-71`, confirmed via
`grep -n "no local shell\|connectionId\|throw"`). This means for backend-go's
deployment model, **all terminal sessions relay to the Dev Server Agent** —
no local fallback needs to be built. No Postgres involvement for PTY I/O
itself; session state lives in-memory on whichever process owns the PTY
(the Dev Server Agent), addressed by `connectionId` the same way
`git.*`/`devServer.*`/`fleet.*` already resolve connections via
`infrafleet-service`'s `ResolveConnection`/`Relay` RPCs.

---

## Related work

This is the same PTY subsystem the current branch
(`fix/pty-session-expired-on-pane-remount`) is patching in the TS codebase —
`agent/src/relay/agent-connection-stdio.ts` (new, uncommitted) and changes to
`agent/src/relay/agent-config.ts`/`agent-entry.ts` are part of that fix. That
work is on the TS agent/relay layer the Dev Server Agent runs, not on
backend-go's Go services — backend-go still needs its own RPC/wscompat
surface built from scratch; it should not be assumed to reuse the TS
`server-pty-controller.ts` code directly.

---

## References

- `services/api-gateway/internal/adapter/wscompat/channels.go:79-89` — `RegisterRealChannels` (no `terminal.*` registration)
- `services/api-gateway/internal/adapter/wscompat/registry.go:59` — `notImplementedHandler`
- `proto/orca/infrafleet/v1/infrafleet.proto:10-31` — `InfraFleetService` RPC surface (no PTY RPCs); `Relay` RPC doc comment at lines 24-31
- `services/infra-fleet-service/internal/adapter/devserveragent/session.go:152` — only PTY-adjacent reference (a `BUG-FE-PTY-001` doc comment, not an implementation)
- `backend/src/main/runtime/server-pty-controller.ts:66-71` — old TS backend's "no local shell" design for server deployment
- `agent/src/relay/pty-handler.ts`, `pty-daemon-server.ts`, `pty-daemon-client.ts`, `pty-daemon-protocol.ts`, `pty-agent-bridge.ts`, `pty-shell-launch.ts`, `pty-shell-utils.ts` — existing agent-side PTY infrastructure
- `agent/src/relay/agent-connection-stdio.ts` — new/uncommitted, current branch, SSH stdio transport variant of the same relay layer
- `specs/frontend/api/rpc-catalog.md:457-470` — `terminal.*` catalog entries
