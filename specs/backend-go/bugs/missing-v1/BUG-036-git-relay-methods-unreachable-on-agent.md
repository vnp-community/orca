# BUG-036: `git.*` relay calls target agent methods that don't exist on the transport backend-go actually uses — including the 2 channels marked "wired"

**Service:** `git-gateway-service` (caller) + `agent/` (missing handler registration)
**File:** `services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`, `agent/src/relay/agent-rpc-dispatch.ts`
**Severity:** Critical — this is not a missing-feature gap like the other 34 bugs in this directory; it means the data flow frontend→backend→agent→backend→frontend is **broken today** for the 2 `git.*` methods currently marked "wired" (`git.status`, `git.diff`), and will silently break every method BUG-032/SOL-032 plans to add the same way, unless fixed first
**Status:** ✅ Resolved — see TASK-227–228 (2 task(s), all DONE) for implementation evidence.

---

## Summary

Tracing the full request path for `git.status` (frontend → `wscompat` →
`git-gateway-service` → `infra-fleet-service` → Dev Server Agent) against
the real agent source confirms: **the agent process backend-go's transport
actually reaches never registers `git.status`, `git.diff`, `git.commit`,
`git.push`, or `git.pull` at all.** A relayed call to any of these methods
returns `Method not found` from the agent, for every connection mode
(`relay-ssh`, `relay-websocket`, `direct-websocket`) — not a param-shape
mismatch that partially works, a hard failure on every call.

This directly contradicts `../README.md`'s summary line that `git.status`/
`git.diff` are among the "8 real, frontend-called RPC methods" wired
end-to-end. They're wired in the sense that `wscompat` → `git-gateway-service`
→ `infra-fleet-service` → gRPC all compile and pass through cleanly (and
unit tests using fake gRPC clients pass) — but the final hop, agent →
real dev server, has no receiver. This was flagged as a real risk by the
code's own author but not confirmed — this bug confirms it.

---

## The chain, traced end to end

1. **Frontend → backend**: `callRuntimeRpc(target, 'git.status', {worktreeId})`
   → `wscompat`'s `git.status` channel (`channels.go:222-235`) decodes
   `{worktreeId}`, calls `gitClient.GetStatus(ctx, &GetStatusRequest{WorktreeId})`.
2. **`git-gateway-service`**: `GetStatus.Execute` (`usecase/get_status.go:28-43`)
   calls `dispatchExecutor` (`usecase/ports.go:91-100`), which asks
   `ConnectionResolver.ResolveConnection` whether the worktree's host is
   connected; if so, dispatches to `RelayExecutor` (the "relay to Dev
   Server Agent" branch).
3. **`RelayExecutor.GetStatus`** (`relay_executor.go:92-98`) calls
   `r.relay(ctx, repoPath, "git.status", {"repoPath": repoPath}, &result)`,
   which calls `infra-fleet-service`'s `Relay` gRPC RPC with
   `method: "git.status"`.
4. **`infra-fleet-service`**: `Relay.Execute` (`usecase/relay.go:37-62`)
   resolves the connection, then calls `uc.agent.Exec(ctx, devServer,
   "git.status", params)` — a **generic JSON-RPC method+params passthrough,
   uniform across all 3 connection modes** (`devserveragent/client.go`'s
   package doc comment, lines 1-40: "Exec is a generic passthrough...
   no per-method translation layer, for every mode").
5. **Transport to the agent**: per `client.go`'s doc comment, ALL THREE
   modes (`relay-ssh`, `relay-websocket`, `direct-websocket`) now converge
   on the **same** 13-byte-framed JSON-RPC wire protocol talking to the
   **same** `agent/out/agent.js` binary — including `relay-ssh`, which
   (unlike the old TS backend) does **not** talk to a separate "SSH Relay
   Daemon" process; it SFTP-deploys `agent.js` and launches it in
   `node agent.js --stdio` mode (`agent-connection-stdio.ts`, wired in
   `agent-entry.ts:58-61`), which routes through the exact same
   `agent-rpc-dispatch.ts` request handler as the WebSocket modes
   (`agent-entry.ts:26`'s own comment: "agent-session.ts/agent-rpc-dispatch.ts/
   agent-wire.ts" — one shared implementation for every mode).
6. **The agent's dispatcher**: `agent-rpc-dispatch.ts` is a flat
   `switch (rpc.method)` (per `specs/agent/api/agent-rpc-catalog-git-fs.md`'s
   own "Part A" description). Confirmed by direct grep — **it has no case
   for `git.status`, `git.diff`, `git.commit`, `git.push`, or `git.pull`**:
   ```
   $ grep -n "'git\.status'\|'git\.diff'\|'git\.commit'\|'git\.push'\|'git\.pull'" agent/src/relay/agent-rpc-dispatch.ts
   (no matches)
   ```
   The `default:` case (`agent-rpc-dispatch.ts:1251`) returns
   `makeError(rpc.id, AgentErrorCode.MethodNotFound, "Method not found: ${rpc.method}")`.

These 5 methods exist **only** on what `specs/agent/api/agent-rpc-catalog-git-fs.md`
calls "Part B — SSH Relay Daemon" (`agent/src/relay/relay.ts` + `dispatcher.ts`'s
`RelayDispatcher`, wired via a `GitHandler` class in `git-handler.ts`) — a
**second, separate dispatcher class** the old TS backend reached only
through its own `relay-ssh` connection mode talking to a standalone SSH
Relay Daemon process. Backend-go's `relay-ssh` mode was rebuilt
differently (§5 above) and never invokes this dispatcher at all, in any
mode.

---

## This is a known, already-documented class of gap — just not evaluated against backend-go's specific architecture

`specs/agent/api/gaps-and-findings.md` (§4, "Same method name, different
contract, across Part A vs Part B") already documents this exact
divergence as a pre-existing agent-side issue:

> Part A has no dedicated `git.stage`/... Part B does — so Part A's
> `git.exec` genuinely needs to keep accepting... porting Part B's ~20
> dedicated per-operation RPCs to Part A first [is] a higher-risk project,
> not attempted here.

In the **old TS backend**, this only mattered for `direct-websocket`/
`relay-websocket` connections (and even those, per
`agent-rpc-catalog-git-fs.md`'s intro, apparently routed `DevServerGitProvider`
calls to Part B's method set some other way not fully explained in that
doc — worth re-verifying against the TS source if this needs closing
there too, out of scope for this backend-go-focused bug). For
**backend-go**, it's unconditional: every connection mode funnels through
`devserveragent.Client.Exec`, which only ever reaches Part A. There is no
path to Part B anywhere in backend-go's execution-plane client code.

---

## A second, independent problem even if reachability were fixed: `git.diff`'s shape doesn't match

Even setting aside reachability, `RelayExecutor.GetDiff` sends
`{repoPath, staged}` and expects a whole-repository `domain.DiffResult`
back. Part B's real `git.diff` contract (`agent-rpc-catalog-git-fs.md`
line 125) is **per-file**: `worktreePath, filePath, staged,
compareAgainstHead?` → a single file's diff object. There is no
"whole-repo diff" RPC on either Part A or Part B — a caller wanting a full
diff is expected to call `git.status` first to enumerate changed files,
then `git.diff` once per file. `GitExecutor.GetDiff`'s Go interface
(`ports.go:63`) has no `filePath` parameter at all — this isn't a
param-naming fix, it's a missing parameter and a wrong response-shape
assumption baked into the interface itself.

---

## Impact

- **`git.status`/`git.diff`** (the only 2 of 34 `git.*` methods currently
  claimed as "wired" in `../README.md`) are broken against a real agent
  today for connected (relay-required) worktrees. They only "work" for
  local (unconnected) worktrees, where `dispatchExecutor` picks the
  `localgit` executor instead and never reaches the agent at all.
- **BUG-032/SOL-032/TASK-206–216** (the largest solution+task set in this
  entire audit, ~28 new `git.*` RPCs) explicitly propose "extending the
  existing dispatch pattern" `GetStatus`/`GetDiff` already use — i.e.
  building 28 more methods on top of the same broken relay-method-naming
  assumption. **Do not implement SOL-032's relay calls as designed without
  first resolving this bug** — every one of them would hit the same
  `Method not found` wall.
- `git.commit`/`git.push`/`git.pull` (SOL-032/TASK-206's "wiring-only
  quick wins") are equally affected — `RelayExecutor` already has code for
  all 3, all equally unreachable.

---

## Fix directions (not decided here — flagging the decision, not making it)

1. **Extend Part A's dispatcher to re-expose Part B's git ops** — mirrors
   the precedent already established for `git.history`/`branchCompare`/
   `commitCompare`/`branchDiff`/`commitDiff`/`checkIgnored`/`forkSync`/
   `submoduleStatus`, which `agent-rpc-catalog-git-fs.md` confirms Part A
   already re-exposes by importing Part B's ops modules
   (`git-handler-ops.ts`, `git-handler-status-ops.ts`, etc.) directly —
   the same pattern would work for `status`/`diff`/`stage`/`commit`/`push`/`pull`.
   This is an `agent/` change; per the TDD's stated boundary that's
   normally out of scope for the backend-go rewrite, but this is
   re-exposing already-built, already-validated logic through routing,
   not new capability — closer to `gaps-and-findings.md`'s own
   already-scoped-but-deferred #4 than to the larger, genuinely-new-capability
   asks in BUG-006/BUG-008/BUG-011.
2. **Redesign `RelayExecutor` to use Part A's generic `git.exec`**
   (ADR-012's original design intent — `docs/adrs/v1/ADR-012-remote-git-ui-via-relay-rpc.md`)
   — compose `git status --porcelain=v2 --branch` /
   `git diff [--staged]` calls via the passthrough Part A already has
   (confirmed present, `agent-rpc-dispatch.ts` registers `git.exec`), and
   parse the raw output in `git-gateway-service` instead of relying on
   Part B's structured JSON responses. No `agent/` change needed, but
   duplicates parsing logic Part B's ops modules already have, and loses
   Part A's per-flag `git.exec` command-injection hardening's benefit of
   a narrow, purpose-built RPC surface (per `gaps-and-findings.md` #4's
   own reasoning for why porting to dedicated RPCs was preferred).

Either way, `GetDiff`'s per-file-vs-whole-repo shape mismatch needs
resolving as part of whichever direction is chosen — it's not a
transport-layer fix.

## References

- `backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go:20-35` — the author's own "not verified against a real agent" flag, now confirmed
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:1-40,241-256` — "generic passthrough... uniform across all 3 modes" confirming single Part-A-only transport
- `agent/src/relay/agent-rpc-dispatch.ts:1251` — the `MethodNotFound` default case; confirmed no `git.status`/`diff`/`commit`/`push`/`pull` registration anywhere in the file
- `agent/src/relay/agent-entry.ts:26,54-61` — stdio (relay-ssh) mode wired through the same dispatcher as WS modes
- `specs/agent/api/agent-rpc-catalog-git-fs.md` — Part A/Part B split, the real `git.status`/`git.diff` (Part B) contract
- `specs/agent/api/gaps-and-findings.md` §4 — the pre-existing, already-documented Part A/B divergence this bug applies to backend-go's specific (worse) case
- `docs/adrs/v1/ADR-012-remote-git-ui-via-relay-rpc.md` — the original `git.exec`-passthrough design, relevant to fix direction 2
- [BUG-032](./BUG-032-git-channels-partially-implemented.md), [`solutions/SOL-032-git-channels.md`](./solutions/SOL-032-git-channels.md) — the solution/task set this blocks
