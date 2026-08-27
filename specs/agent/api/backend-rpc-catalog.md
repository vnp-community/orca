# Backend → provides → Agent

Everything the **backend** process must handle when it's the **agent** that
initiates traffic — true request/response RPC calls the agent makes into the
backend, unsolicited event/notification pushes, and the one plain-HTTP
endpoint the agent calls. This is the reverse of
[`agent-rpc-catalog-git-fs.md`](./agent-rpc-catalog-git-fs.md) /
[`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md). See
[`connection-modes.md`](./connection-modes.md) for the transport.

## 0. Two transports carry this traffic, and they diverge

- **`direct-websocket`/`relay-websocket` mode**: `agent/src/relay/agent-session.ts`
  + `agent-rpc-dispatch.ts` (switch-based router) ↔
  `backend/src/main/dev-server/agent-ws-server.ts` (wraps the WS in a
  `SshChannelMultiplexer`).
- **`relay-ssh`/WSL-guest mode**: `agent/src/relay/relay.ts` +
  `dispatcher.ts` (`RelayDispatcher`, event-based) ↔
  `backend/src/main/ssh/ssh-relay-session.ts` (drives the backend's own
  `SshChannelMultiplexer` over the SSH channel).

**Method names and even wire *shapes* diverge between the two transports for
the same logical event** (e.g. `pty.exit`'s exit-code field, `fs.changed`'s
payload shape — see [`gaps-and-findings.md`](./gaps-and-findings.md)).

## 1. Agent → Backend method/event table

| Method / event | Transport | Agent-side sender | Backend-side handler/receiver | Payload shape | Purpose | Style |
|---|---|---|---|---|---|---|
| `POST /api/agent-token` | plain HTTP | `agent-token-manager.ts:206-240` (`fetchOnce`) | `backend/src/server/agent-token-routes.ts:76-208` (`handleAgentTokenRequest`), route wired at `:222-245` | `{devServerId,name,ttl,permanent}` → `{token,expiresIn,...}` | Initial token fetch + proactive 80%-TTL renewal for `direct-websocket` reconnect | Request/response (HTTP) |
| `agent.handshake` | WS JSON-RPC (id=1) | `agent-session.ts:196-215` (`sendHandshake`) | `ws-handshake.ts` (`runOrcaReceiverHandshake`), invoked from `agent-ws-server.ts:139-147` | `{agentVersion,platform,arch,nodeVersion,capabilities,agentToken?,devServerId,tools}` | Establish session, authenticate token, negotiate capabilities | Request/response — full details in [`connection-modes.md`](./connection-modes.md) §5 |
| KeepAlive frame | WS binary frame | `agent-session.ts:221-227` (every 5000ms) | Consumed by `SshChannelMultiplexer` framing inside `agent-ws-server.ts`; app-level `ws.ping()` also runs at `:124-126` | none (empty payload) | Liveness only — no metrics/health payload (see §3) | Fire-and-forget |
| `pty.data` / `pty.exit` / `pty.replay` (direct-ws shape: exit key `exitCode`) | WS JSON-RPC notification | `pty-agent-bridge.ts:202,207,285` via `makeNotifier` (`agent-rpc-dispatch.ts:251-256`) | `backend/src/main/providers/dev-server-pty-provider.ts:53-71` | `{id,data}` / `{id,exitCode,signal}` / `{id,data}` | Real-time PTY output/exit/replay push, independent of the request that spawned the PTY | Fire-and-forget |
| `fs.changed` (direct-ws shape: singular event) | WS JSON-RPC notification | `fs-agent-extensions.ts:662,719,722` via `notify` from the `fs.watch` case (`agent-rpc-dispatch.ts:875-883`) | `backend/src/main/providers/dev-server-filesystem-provider.ts:216-217` | `{path,eventType,filename}` | Filesystem watch change push | Fire-and-forget |
| `agent.output` / `agent.exited` | WS JSON-RPC notification | `agent-spawner.ts:499,519-522` (raw `ws.send`, not via `makeNotifier`) | **No consumer found** — repo-wide grep found no match outside a doc comment | `{taskId?,...}` / `{taskId?,exitCode,...}` (inferred) | Streaming output of `agent.spawn`-launched interactive processes | Fire-and-forget — **orphaned/unconsumed on backend**, see [`gaps-and-findings.md`](./gaps-and-findings.md) |
| `stream.chunk` / `stream.end` (git.execStream result frames) | WS JSON-RPC — response frames to the original request id | `agent-git-handler.ts:232-241,275-279` | **No dedicated consumer confirmed** in `dev-server-filesystem-provider.ts`/git provider (name not found by grep) | `{type:'stream.chunk',line,source?}` / `{type:'stream.end',exitCode}` | Streaming git stdout/stderr for direct-ws mode | Streamed response — flagged for follow-up |
| `orca.cli` | SSH-relay JSON-RPC **request** (agent/relay-initiated, expects a response) | `relay.ts:494-499` — relay's own `dispatcher.onRequest('orca.cli', ...)` forwards via `dispatcher.requestAnyClient('orca.cli', params, {...})` | `backend/src/main/ssh/ssh-relay-session.ts:809-833` (`mux.onRequest('orca.cli', ...)` → `runRemoteOrcaCli`) | `{argv,cwd,env,stdin?}` → CLI result | A remote `orca` CLI shim (invoked from a PTY on the host) round-trips through the relay to backend's runtime | **True request/response, agent-initiated** — see §2 |
| `agent.hook` (`AGENT_HOOK_NOTIFICATION_METHOD`) | SSH-relay/WSL-relay JSON-RPC notification | `agent-hook-server.ts:104,367` (`this.forward(envelope)`) → `dispatcher.notify(...)` in `relay.ts:529-538`; also `wsl-agent-hook-relay.ts:53-57` | `ssh-relay-session.ts` `wireUpAgentHookEvents` (~`:897-940`) → `backend/src/main/agent-hooks/server.ts:1102-1230` (`ingestRemote`) | See §4 | Forwards a locally-POSTed agent-CLI hook event (Claude/Codex/etc. status) to Orca | Fire-and-forget |
| `fs.changed` (SSH-relay shape: batched events array) | SSH-relay JSON-RPC notification | `relay-filesystem-watch-registry.ts:192,203` | `backend/src/main/providers/ssh-filesystem-provider.ts:45-56` | `{events:[{kind,absolutePath,isDirectory?}]}` | Filesystem watch change push (SSH mode) | Fire-and-forget |
| `pty.data` / `pty.exit` / `pty.replay` (SSH-relay shape: exit key `code`) | SSH-relay JSON-RPC notification | `pty-handler.ts:482,571,598,614,874,953` | `backend/src/main/providers/ssh-pty-provider.ts:56-75` (`case 'pty.data'/'pty.replay'/'pty.exit'`) | `{id,data}` / `{id,code}` / `{id,data}` | PTY streaming for SSH-relay mode | Fire-and-forget |
| `git.responseChunk` / `git.responseEnd` / `git.responseError` | SSH-relay JSON-RPC notification (bulk lane), correlated by `streamId` | `git-response-stream.ts:172,181,186` | `backend/src/main/ssh/ssh-git-response-stream-reader.ts:222-245` | `{streamId,seq,data}` / `{streamId}` / `{streamId,message}` | Chunks a large git diff/exec response off the interactive lane so it doesn't head-of-line-block `pty.data` | Streamed push, credit-windowed (client acks flow back) |
| `git.cloneProgress` | SSH-relay JSON-RPC notification | `git-handler.ts:1311` | `backend/src/main/providers/ssh-git-provider.ts:790` (`mux.onNotificationByMethod('git.cloneProgress', ...)`) | clone progress fields | git clone progress push | Fire-and-forget |
| `git.clone.output` | SSH-relay JSON-RPC notification | `git-handler-clone.ts:30,40` | **No consumer found** in backend | clone stdout lines | Legacy/duplicate of `git.cloneProgress`? | **Orphaned** — see [`gaps-and-findings.md`](./gaps-and-findings.md) |
| `workspace.changed` | SSH-relay JSON-RPC notification | `workspace-session-handler.ts:154` | `ssh-relay-session.ts` `wireUpRemoteWorkspaceEvents` (~`:884-887`) → `backend/src/main/ipc/remote-workspace-events.ts` (`notifyRemoteWorkspaceHandlers`) | workspace session diff | Remote workspace state change push | Fire-and-forget |
| `fs.streamChunk`/`fs.streamEnd`/`fs.streamError` | SSH-relay JSON-RPC notification (bulk lane) | `fs-handler-file-read.ts:206,230,233` | `backend/src/main/ssh/ssh-filesystem-stream-reader.ts` (same credit-window pattern as git-response-stream) | large-file-read chunking | Streams a large file read off the interactive lane | Streamed push |

## 2. Is the JSON-RPC dispatcher truly bidirectional?

**Yes, confirmed with a concrete example — but only exercised in
`relay-ssh`/WSL-relay mode.**

- Agent-side `RelayDispatcher` (`dispatcher.ts`) exposes `onRequest(method,
  handler)` (registers what the agent answers when backend calls it — the
  normal direction) **and** `requestPrimary()`/`requestAnyClient()` (lets the
  relay/agent process *initiate* a request toward a connected client and
  await its response) — `dispatcher.ts:148-150, 276-300`.
- Backend's mirror engine, `SshChannelMultiplexer`
  (`backend/src/main/ssh/ssh-channel-multiplexer.ts`), symmetrically exposes
  `onRequest(method, handler)` (`:145-150`) — a real registration API letting
  the **agent/relay call into backend and get a handled response**, not just
  backend calling the agent.
- Concrete round-trip proving it: a CLI shim on the remote host sends
  `orca.cli` to the relay → relay's own `dispatcher.onRequest('orca.cli', …)`
  forwards it via `dispatcher.requestAnyClient('orca.cli', …)` to the primary
  client (Orca backend's SSH connection) → **backend registers `mux.onRequest(
  'orca.cli', async (params) => runRemoteOrcaCli(...))`** at
  `backend/src/main/ssh/ssh-relay-session.ts:809-833`. This is a genuine
  agent-initiated RPC call answered by a backend-registered handler, not a
  reflection of a backend-issued request.
- **Caveat**: this same `mux.onRequest` capability exists on the
  `direct-websocket` backend transport too (it's the same
  `SshChannelMultiplexer` class wrapping the WS), but a repo-wide grep found
  **only one** backend `.onRequest(...)` registration total (`orca.cli`,
  `relay-ssh` mode). No `direct-websocket`-mode example of backend
  registering a handler the agent calls as a request/response was found — all
  `direct-websocket` "backend-provides-to-agent" traffic in the reverse
  direction is backend *calling* the agent
  (`relay.call('git.exec', …)` etc., see the agent-rpc-catalog files), and
  all agent→backend `direct-websocket` traffic found is unsolicited
  notifications (`pty.data`, `fs.changed`) rather than requests.
- `backend/src/relay/dispatcher.ts` (byte-identical to the agent copy) is
  **not actually imported anywhere in `backend/src/main`** except by another
  file inside the same vendored `backend/src/relay/` directory — a
  cross-package vendoring artifact, not the live runtime dispatcher backend
  actually uses. See [`gaps-and-findings.md`](./gaps-and-findings.md).

## 3. Health-reporting protocol

There is **no dedicated health-report payload** — only a bare liveness
keepalive:

- **`direct-websocket` mode**: `agent-session.ts:221-227` (`startKeepalive`)
  sends an empty `MessageType.KeepAlive` binary frame every
  `AGENT_KEEPALIVE_INTERVAL_MS` = **5000ms**. Backend ACKs at the framing
  level inside `SshChannelMultiplexer`; app-level, `agent-ws-server.ts:122-128`
  also runs its own `ws.ping()` every 30s to survive reverse proxies
  (ALB/Cloudflare).
- The agent also responds to backend-sent KeepAlive frames immediately
  (`agent-session.ts:276-281`) to maintain ACK progress.
- **SSH-relay mode**: `dispatcher.ts:513-531` (`startKeepalive`) sends an
  empty `MessageType.KeepAlive` frame every `KEEPALIVE_SEND_MS` = **5000ms**,
  same no-payload liveness pattern.
- What might look like "agent health" (`ai.provider.healthCheck`,
  `agent-credential-store.ts:208-254`) is actually **backend-initiated** —
  backend calls the agent to test an AI provider's reachability, the reverse
  direction, not agent-pushed. See
  [`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md).
- The HLD's "Health Reporter — emits metrics every 60s" narrative
  (`cpu/ram/disk/latencyMs`) does **not** correspond to any confirmed live
  code path in this audit — see [`gaps-and-findings.md`](./gaps-and-findings.md).

## 4. Hook-forwarding protocol (`agent.hook`)

- **Transport**: SSH-relay/WSL-relay only (not `direct-websocket`) — JSON-RPC
  notification, method `AGENT_HOOK_NOTIFICATION_METHOD = 'agent.hook'`
  (`agent/src/shared/agent-hook-relay.ts:97`).
- **Agent side**: `RelayAgentHookServer` (`agent-hook-server.ts`) hosts a
  loopback HTTP server (`127.0.0.1:0`, bearer-token auth via
  `x-orca-agent-hook-token`, routes `/hook/<source>`) that receives POSTs
  from agent CLIs (Claude, Codex, Gemini, etc.) running in PTYs on the remote
  host. `handleRequest` (`:294-339`) → `normalizeHookPayload` →
  `applyEvent`/`forwardEvent` (`:341-393`) → `this.forward(envelope)`, wired
  in `relay.ts:529-538` and `wsl-agent-hook-relay.ts:53-57` to
  `dispatcher.notify('agent.hook', envelope)`.
- **Wire envelope** (`AgentHookRelayEnvelope`,
  `agent/src/shared/agent-hook-relay.ts:58-94`):
  `{source, paneKey, launchToken?, tabId?, worktreeId?, connectionId: null (always),
  hasExplicitPrompt?, promptInteractionKey?, hookEventName?, toolUseId?,
  toolAgentId?, toolAgentType?, providerSession?, isReplay?, env?, version?,
  payload: ParsedAgentStatusPayload}`. `connectionId` is always `null` on the
  wire — the relay doesn't know Orca's local handle.
- **Backend side**: `ssh-relay-session.ts` `wireUpAgentHookEvents`
  (~`:897-940`) filters `mux.onNotification` for
  `method === AGENT_HOOK_NOTIFICATION_METHOD`, stamps the real `connectionId`
  from `this.targetId`, and calls
  `agentHookServer.ingestRemote(envelope, connectionId)` →
  `backend/src/main/agent-hooks/server.ts:1102-1230`, which re-runs
  `normalizeAgentStatusPayload` at the SSH trust boundary (never trusts the
  relay's own normalization) before caching into
  `state.lastStatusByPaneKey` and firing `onAgentStatus`/status-change
  listeners.
- **Replay**: backend can pull `dispatcher.onRequest(AGENT_HOOK_REQUEST_REPLAY_METHOD
  ='agent_hook.requestReplay', ...)` (agent-side handler, `relay.ts:634`)
  after reconnect to replay the relay's per-pane cache — this direction is
  **backend→agent** (a request), not agent-initiated.
- This entire path is **relay-only** — `direct-websocket`-mode agents have no
  equivalent hook-forwarding surface.

## 5. PTY output and fs-watch-change streaming push protocols

Two independent wire shapes per transport for the same logical events:

**PTY streaming**
- `direct-ws`: `pty-agent-bridge.ts` — `term.onData` (`:200-203`) →
  `entry.notify('pty.data', {id, data})`; `term.onExit` (`:204-208`) →
  `entry.notify('pty.exit', {id, exitCode, signal})`; `handlePtyAttach`
  (`:284-286`) → `notify('pty.replay', {id, data})`. Frames go out via
  `makeNotifier` (`agent-rpc-dispatch.ts:251-256`), rebound to whatever WS
  connection last issued a `pty.*` request. Backend:
  `dev-server-pty-provider.ts:53-71`.
- SSH-relay: `pty-handler.ts:482` (`pty.exit`, key `code`), `:571,598,614`
  (`pty.data`), `:874` (`pty.replay`), `:953` (forced `pty.exit`). Backend:
  `ssh-pty-provider.ts:56-75`.
- **The exit-code field name differs between transports** (`exitCode`
  direct-ws vs `code` SSH-relay) — see
  [`gaps-and-findings.md`](./gaps-and-findings.md).

**fs-watch-change streaming**
- `direct-ws`: `fs-agent-extensions.ts:662,719,722` →
  `notify('fs.changed', {path, eventType, filename})` — one event per
  notification. Backend: `dev-server-filesystem-provider.ts:216-217` (filters
  `params.path === rootPath`).
- SSH-relay: `relay-filesystem-watch-registry.ts:192` (batched) / `:203`
  (overflow sentinel) → `dispatcher.notify('fs.changed', {events:
  [{kind, absolutePath, isDirectory?}, ...]})`. Backend:
  `ssh-filesystem-provider.ts:45-56` (filters each event by
  `isPathInsideOrEqual(rootPath, e.absolutePath)`).
- **The payload shape differs by transport** for the identical method name
  `fs.changed` (singular field-set vs `events` array) — any shared/generic
  backend handler keyed only on method name must branch on transport, which
  is exactly what the two separate provider classes
  (`dev-server-filesystem-provider.ts` vs `ssh-filesystem-provider.ts`) do
  today.

## Sources

Agent side: `agent/src/relay/agent-session.ts`, `agent-rpc-dispatch.ts`,
`dispatcher.ts`, `protocol.ts`, `agent-hook-server.ts`, `agent-token-manager.ts`,
`agent-credential-store.ts`, `pty-agent-bridge.ts`, `git-response-stream.ts`,
`relay-filesystem-watch-registry.ts`, `wsl-agent-hook-relay.ts`,
`wsl-hook-fs-bridge.ts`, `agent-spawner.ts`, `agent-git-handler.ts`, `relay.ts`,
`pty-handler.ts`, `git-handler.ts`, `workspace-session-handler.ts`,
`fs-handler-file-read.ts`, `git-handler-clone.ts`,
`agent/src/shared/agent-hook-relay.ts`, `agent/src/shared/agent-wire-protocol.ts`.
Backend side: `backend/src/server/agent-token-routes.ts`,
`backend/src/main/dev-server/agent-ws-server.ts`, `dev-server-relay-bridge.ts`,
`dev-server-manager.ts`, `backend/src/main/providers/dev-server-pty-provider.ts`,
`dev-server-filesystem-provider.ts`, `ssh-pty-provider.ts`,
`ssh-filesystem-provider.ts`, `ssh-git-provider.ts`,
`backend/src/main/ssh/ssh-relay-session.ts`, `ssh-channel-multiplexer.ts`,
`ssh-git-response-stream-reader.ts`, `ssh-filesystem-stream-reader.ts`,
`backend/src/main/agent-hooks/server.ts`,
`backend/src/main/runtime/orca-runtime-pty-data-ingest.ts`,
`backend/src/relay/dispatcher.ts` (confirmed unused/vendored).
