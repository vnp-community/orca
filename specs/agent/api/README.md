# Backend ↔ Agent API Surface

This directory catalogs every connection `backend/` (the Orca backend/gateway
process) makes to `agent/` (the dev-server agent that runs on a local or
remote dev host), and every connection the reverse way — what a from-scratch
reimplementation of either side would need to support for the pair to work
together. It was generated on 2026-08-15 by reading the current source
directly (not from `specs/agent/tdd/v5/`'s existing design docs, which
describe an earlier/narrower shape of this surface — see
[`gaps-and-findings.md`](./gaps-and-findings.md) §7 for the drift), and
cross-referencing agent-side handler registrations against real backend call
sites (and vice versa).

- [`connection-modes.md`](./connection-modes.md) — every distinct way the two
  processes connect: the 3 `DevServerConnectionType` values
  (`direct-websocket`, `relay-websocket`, `relay-ssh`), the two underlying
  wire-protocol stacks, the `agent.handshake` sequence, the generic
  bidirectional JSON-RPC dispatcher both sides run, keepalive/reconnection,
  and agent-token lifecycle.
- [`agent-rpc-catalog-git-fs.md`](./agent-rpc-catalog-git-fs.md) — every RPC
  method the **agent provides for the backend to call**, scoped to `git.*`
  and `fs.*` (the largest namespaces).
- [`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md) — the rest
  of what the **agent provides for the backend to call**: `pty.*`,
  `ai.*`/`agent.*`, `github.*`/`gitlab.*`, `externalAutomations.*`,
  `preflight.*`, `ports.*`, `workspace.*`, plus the confirmed gap methods
  (`shell.exec`, `notification.send`, `ai.provider.testConnection`) the
  backend calls with no agent-side handler to answer them.
- [`backend-rpc-catalog.md`](./backend-rpc-catalog.md) — everything the
  **backend provides for the agent to call**: the one plain-HTTP token
  endpoint, the bidirectional-dispatcher proof case (`orca.cli`), health/
  keepalive, agent-hook event ingestion, and every PTY/fs/git streaming push
  the agent sends unsolicited.
- [`gaps-and-findings.md`](./gaps-and-findings.md) — every cross-cutting bug,
  inconsistency, or piece of doc drift this audit surfaced (missing
  handlers, an orphaned notification, a handler-registration-order security
  bug in `git.clone`, method-name/shape divergence between the two RPC
  surfaces, etc.), consolidated in one place since several span multiple of
  the files above.

## The headline architectural finding

The agent process runs **two almost entirely independent RPC surfaces**
simultaneously, not one:

- **Part A — "Dev Server Agent"**: a local process the backend connects to
  directly over WebSocket (`direct-websocket`/`relay-websocket` modes).
  Entry: `agent/src/relay/agent-connection-direct.ts`/`agent-connection-relay.ts`.
  Router: `agent/src/relay/agent-rpc-dispatch.ts`, a flat
  `switch (rpc.method)` with dynamically-imported handler modules.
- **Part B — "Orca Relay"**: a standalone script (`relay.ts`) SCP'd to the
  remote host and driven over an SSH exec channel (`relay-ssh` mode).
  Router: `agent/src/relay/dispatcher.ts`'s `RelayDispatcher`, a
  `Map<method, handler>` that handler classes (`GitHandler`, `FsHandler`,
  `PtyHandler`, ...) register into via their constructors.

Both stacks converge on the same 13-byte-header wire framing and the same
`SshChannelMultiplexer`-based transport once each mode's own handshake
completes (see [`connection-modes.md`](./connection-modes.md) §0), but they
are **separately implemented, separately registered, and frequently diverge**
— same method name with a different parameter contract
(`preflight.check`), same domain with entirely different method names
(`pty.create` vs `pty.spawn`), or a method that exists on one side and not
the other. Any consumer of this doc set that assumes "the agent's RPC
surface" is a single flat namespace will be wrong; every catalog entry in
this directory is labeled with which Part it belongs to.

There is also a third, narrower surface — a **WSL-guest micro-relay**
(`wsl-agent-hook-relay.ts`), a separate binary the Windows host launches
inside a WSL distro, running its own `RelayDispatcher` instance for a
home-scoped `wslfs.*` filesystem bridge and hook-event forwarding only.

## Methodology

- **Agent-side registry**: read `agent/src/relay/agent-rpc-dispatch.ts` (Part
  A's switch statement, ~1050 lines) and `agent/src/relay/relay.ts` +
  `dispatcher.ts` (Part B's handler-class wiring) directly, plus every
  handler implementation file each router delegates to, to record every
  method name, its registration site, its params/return shape, and any
  security/isolation constraint on it.
- **Backend-side cross-reference**: grepped `backend/src/**` for every
  `relay.call(...)`/`.call(...)` call site and every `mux.onRequest(...)`/
  `onNotification(...)` registration, to confirm which agent-side methods
  actually have a real backend caller today (vs. existing only in the
  registry with no live caller), and to find the reverse-direction handlers
  the backend registers for agent-initiated traffic.
- **Cross-check, not exhaustive param-shape verification**: like the
  frontend↔backend catalog (`specs/frontend/tdd/api/rpc-catalog.md`), this is
  primarily a name-and-registration-level audit backed by direct source
  reads — it confirms a method exists and is called, and documents the
  params/returns visible in the handler signature, but does not runtime-test
  every call. Where a mismatch or gap *was* found by this deeper read (e.g.
  the confirmed-missing `shell.exec`/`notification.send`/
  `ai.provider.testConnection` handlers, or the `git.clone` registration-order
  bug), it's called out explicitly in
  [`gaps-and-findings.md`](./gaps-and-findings.md) rather than left implicit.
- Five parallel deep-read passes fed this doc set: connection/transport
  architecture, the `git.*`/`fs.*` agent-side catalog, the remaining
  agent-side namespaces, backend's outbound call sites, and the
  agent→backend reverse direction. Each file below cites `file:line` for
  every claim; re-verify against current source before relying on this doc
  set for a security decision, since the codebase moves faster than any
  snapshot of it.

## Scope note

This directory documents the **wire-level RPC/API contract** between the two
processes. It does not cover: the backend's own HTTP/RPC surface toward the
frontend (see `specs/frontend/tdd/api/`), the agent's local-only PTY-daemon
IPC (a single-host implementation detail, only mentioned where it explains
why `pty.*` exists twice), or product/UX-level behavior built on top of this
RPC surface.
