# Agent Compliance Audit — 2026-08-15

Scope: does `agent/` (a) implement every RPC method `backend/` actually calls
on it, and (b) comply with the architecture described in `docs/` (ADRs,
HLD)? `desktop/` is explicitly out of scope for changes — findings that
require a `desktop/` change are called out but not acted on.

This audit was triggered by a direct question after two rounds of fixes
already landed (see [`gaps-and-findings.md`](./gaps-and-findings.md)). It
found the prior audits' method-name-level check (via the six connection-
type-aware provider classes) was accurate but **incomplete** — it missed an
entire category of backend call sites that bypass those providers.

**Bottom line: no, agent/ has not fully satisfied backend/'s requirements or
docs/'s architecture.** Several concrete, high-severity gaps exist. Some are
fixable inside `agent/` alone (in progress / candidates below); several
others require changes in `desktop/` (out of scope this session) or are
actually bugs in `backend/`, not `agent/`. The docs/ADR half of this audit
found the newest architecture documents (ADR-014/015/017/018/019 — the "v3
protocol" / "signed execution context" / "A0-A4 layer model") describe a
system that was never built at all; the code still runs the older
ADR-004/005 design. That's a documentation-vs-reality gap, not a regression,
and isn't something to "fix" by writing more agent code — see §2.

---

## 1. NEW — critical discovery: `agent/src/relay/relay.ts` is not the shipped `relay-ssh` binary

Before the RPC findings below make sense, one structural fact has to be
stated plainly: **the `relay.js` binary actually deployed to remote hosts
for `relay-ssh` connections is built from `desktop/src/relay/relay.ts` via
`desktop/config/scripts/build-relay.mjs`** (`RELAY_ENTRY = .../desktop/src/relay/relay.ts`),
**not from `agent/src/relay/relay.ts`**. `agent/build.mjs`'s only entry
point is `agent/src/relay/agent-entry.ts`, which only supports
`direct-websocket`/`relay-websocket` — it never imports `relay.ts` at all
(confirmed by reading `agent-entry.ts` in full).

`agent/src/relay/` and `desktop/src/relay/` are two independent copies of
largely the same file set (`relay.ts`, `dispatcher.ts`, `git-handler.ts`,
`pty-handler.ts`, `fs-handler.ts`, …), kept in sync **manually, with no
sync script found**. Diffing confirmed: `dispatcher.ts` was **byte-identical**
between the two trees before this session's fixes; `git-handler.ts` and
`pty-handler.ts` had only minor (20-30 line) pre-existing drift.

**Consequence**: the prior session's "Part B" (`relay-ssh`) fixes — the
`git.clone` handler-override security bug, idle-timeout enforcement in
`dispatcher.ts`, the `pty.spawn` args-drop fix in `pty-handler.ts`, and the
`agent-git-exec-validator.ts` injection/RCE hardening — are all still
correct, valuable fixes to `agent/`'s copy of this code. **But because the
real shipped `relay-ssh` binary is built from `desktop/`'s copy, none of
those fixes are in the binary backend/docs actually rely on for `relay-ssh`
connections today.** The `git.clone` security bug in particular (unvalidated
`spawn('git', ['clone', ..., url, targetPath])` from client-supplied input)
is still live in the real deployed relay-ssh binary.

This needs an explicit decision — see "Open decisions" at the end.

---

## 2. Backend RPC compliance — fresh re-audit

Methodology: every backend call site that goes through the six
connection-type-aware provider classes (`DevServerGitProvider`/
`DevServerFilesystemProvider`/`DevServerPtyProvider`/`SshGitProvider`/
`SshFilesystemProvider`/`SshPtyProvider`) — **zero gaps, fully compliant**.
The gaps are all in **raw `relay.call()` sites that bypass those providers**
(`devServerManager.getRelay(id).call(method, params)`, or the
`RelayConnectionPool`/`ProjectServerRouter.getRelayForProject` wrappers
around the same thing) — these are reachable against a Dev Server configured
for **either** connection type, so the method must exist on both Parts, and
frequently doesn't.

### Confirmed gaps, most severe first

| # | Method(s) | Backend caller | Missing on | Real-world impact |
|---|---|---|---|---|
| 1 | `agent.exec` (param shape) | `backend/src/main/workflow/StepExecutors.ts:107` (`executeAgent`) | **Both Parts, but it's a backend bug, not an agent gap** — see below | Every workflow step of type `agent` fails, on every connection mode |
| 2 | `ai.provider.writeCredential`/`readCredential`/`testConnection` | `backend/src/main/ai-providers/AIProviderService.ts` (multiple) | Part B (`relay.ts` never wires any `ai.provider.*` handler) | AI provider account add/rotate/test completely broken on `relay-ssh` Dev Servers |
| 3 | `preflight.setGitIdentity`/`detectGhosttyConfig`/`detectWindowsTerminalCapabilities`/`detectAgents` | `backend/src/main/ipc/onboarding-ipc.ts`, `dev-server-relay-bridge.ts:572` | **Part A** (`agent-rpc-dispatch.ts` has no case for any of these) | Onboarding git-identity setup, Ghostty detection, Windows terminal capability probe, and installed-CLI detection all fail on `direct-websocket` — **the default connection mode** |
| 4 | `shell.exec`/`notification.send`/`ai.complete` | `StepExecutors.ts` (shell/notification steps), `TaskAIPlanner.ts:62` | Part B | Workflow `shell`/`notification` steps and AI task decomposition fail on `relay-ssh` Dev Servers. `shell.exec`/`notification.send` were "fixed" in the pass-1 agent-only round — **that fix only covered Part A; Part B was never touched**, so `relay-ssh` Dev Servers are still broken for these two |
| 5 | `git.clone`, `fs.listDirectory` | `backend/src/main/ipc/repo-remote-ipc.ts` | Part A | Lower severity — this consumer was already flagged as UI-unreachable in the prior pass; still a real gap if ever wired up |
| 6 | `git.worktree.list` vs `git.listWorktrees`, `fs.mkdir` vs `fs.createDir`, `fs.rmdir` vs `fs.deletePath` | `WorkspaceService.ts`, `runtime/rpc/methods/dev-server.ts` | one Part each (name mismatch, not a missing concept) | Workspace worktree listing / `devServer.mkdir`/`devServer.rmdir` break on whichever Part doesn't recognize that specific name |
| 7 | `preflight.check` contract shape, `git.exec` whitelist width | (already documented) | both, different shapes | Re-confirmed, unresolved — see [`gaps-and-findings.md`](./gaps-and-findings.md) #4 |

### On finding #1 (`agent.exec`) — this is a `backend/` bug, not an `agent/` gap

`StepExecutors.executeAgent()` sends `{stepId, prompt, worktreePath,
trustPreset, traceId, accountId?, model?}` to `agent.exec`. The agent's
handler (`agent-rpc-dispatch.ts:733-808`) correctly implements the RPC
contract every other real caller uses —
`ProfileAwareAgentSpawner.ts:130` sends the right shape
(`{binary, args, cwd, env, timeoutMs}`, with its own `FIX TASK-TG-001`
comment documenting that this was already corrected once). `StepExecutors.ts`
was simply never updated to match, and its own file header
(`StepExecutors.ts:5-8`) still documents the stale, wrong shape as if it
were correct. **Nothing to fix in `agent/` for this one** — flagging so it
doesn't get miscategorized as an agent-side gap in a future pass.

### On findings #2 and #4 (Part B gaps)

Per §1, "Part B" (`relay.ts`) inside `agent/` is not what's actually
deployed for `relay-ssh` — `desktop/src/relay/relay.ts` is. Closing these
gaps for real means changing `desktop/`, which is out of scope this
session. Documented here so the gap isn't lost, not being fixed now.

### On finding #3 — the one gap that's both agent-only-fixable AND high value

`preflight.setGitIdentity`/`detectGhosttyConfig`/
`detectWindowsTerminalCapabilities`/`detectAgents` missing from **Part A**
(`agent-rpc-dispatch.ts`) is squarely inside `agent/`'s scope, requires no
`desktop/` change, and affects the *default* connection mode — this is the
one item from this audit worth fixing in this session. See "Open decisions."

---

## 3. `docs/` (ADR/HLD) compliance — architecture-level findings

Full detail was gathered by reading `ADR-004/005/008/011/012/013/014/015`
(v1) and `ADR-017/018/019` (v2) plus `docs/hld/v1/{C3-components,C4-code,
security}.md` and `docs/hld/dev-server-architecture.md`, cross-referenced
against current `agent/src/` (and `backend/src/` for protocol
counterparts). Full per-claim tables are not reproduced here (they were
extensive) — this is the synthesized verdict per document, most significant
drift first.

1. **ADR-014/015 + ADR-017/018/019 (the entire "v3 protocol" / "signed
   execution context" / "A0-A4 layer model") — 0% implemented.** No
   `ContextVerifier`, `SignedExecutionContext`/`RpcExecutionContext`,
   `ReconnectManager` class, `AgentConnectionManager`, `SignedContextIssuer`,
   `StepExecutor`, generic `EventEmitter`/event-buffer-with-replay,
   `HealthReporter`, or local SQLite exist anywhere in `agent/src` or
   `backend/src` (confirmed via direct grep — 0 hits for every one of these
   symbol names). The wire protocol running today is unambiguously the
   older ADR-004/005 design: 13-byte `[TYPE|SEQ|ACK|LEN]` binary framing,
   plain unsigned `agent.handshake` JSON-RPC, connection-level bearer-token
   trust with no per-call signed context. `agent/src/relay/context.ts`
   shows this is a **deliberate**, documented choice (see
   `docs/relay-fs-allowlist-removal.md`), not an oversight — the team
   explicitly removed the equivalent FS-path allowlist and reasoned that a
   compromised caller can already reach anything via `pty.spawn`/`git.exec`
   regardless. This isn't a bug to fix; it's a set of design documents that
   describe a future/abandoned direction, and should probably be marked as
   such rather than left presented as current architecture.

2. **ADR-018's control/data-plane boundary is actively violated on the
   `backend/` side for GitHub/GitLab.** Backend runs `gh`/`glab` directly in
   its own process (`backend/src/main/github/*`, `gitlab/*` →
   `child_process.execFile`) with **no** `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR`
   isolation, even though a correct, per-user-isolated implementation
   already exists agent-side (`agent/src/relay/external-api-connector.ts`)
   and is simply never called for this path. This is a real, currently-live
   multi-tenancy gap (shared gh/glab auth context across all Web Server
   users) — worse than "undocumented," it's the opposite of what the ADR
   requires. This is a `backend/` fix, not an `agent/` one.

3. **ADR-008's credential architecture diverged in every concrete mechanism**
   (storage path, KDF, file format, double-encryption layering) while the
   high-level goal mostly holds — except the ADR's core claim ("Orca Server
   never sees plaintext credentials") is **false** on the `agent.spawn` path:
   `agent-spawner.ts` requires the backend to forward a plaintext
   `resolvedApiKey` because the agent cannot decrypt the browser's Layer-1
   blob itself. This is a real architectural fact undermining the ADR's
   stated threat model, not just doc drift — worth a security-team decision
   on whether to accept this as the real trust model (and update the ADR)
   or invest in closing it.

4. **ADR-011's "v6.0 amendment" (`AgentConnectionManager` replacing
   `RelayConnectionPool`) never happened** — `RelayConnectionPool`
   (`backend/src/main/dev-server/relay-connection-pool.ts`) is what's
   actually running, and it correctly implements the *original*
   pre-amendment design (ref-counted, 5-min idle cleanup). The amendment
   section of that ADR should be marked aspirational/unimplemented, not
   left reading as current.

5. **ADR-019's "autonomous operation" claims are real only for the narrow
   PTY-reattach-within-2-minutes case.** Reconnect backoff is real but uses
   a fixed 5-value table `[1s,2s,5s,15s,30s]` (`agent-connection-direct.ts:27`),
   not the ADR's `1000*2^n` exponential formula. SQLite-backed state
   survival across an agent *process* restart, a generic event buffer with
   priority-preserving replay, and workflow-step continuity across
   reconnect are all unimplemented.

6. **Positive finding**: several gaps a prior doc-correction pass (dated
   2026-08-14, visible inline in `docs/hld/dev-server-architecture.md` and
   `C3/C4`) flagged have since been fixed in code — git-identity per-client
   scoping (`BUG-AG-HLD-003`), PTY `cols`/`rows` honoring caller values
   (`BUG-AG-HLD-006`), Gemini/OpenCode `resumeId` support (`BUG-AG-HLD-007`),
   `trustPreset` now read (`BUG-AG-HLD-008`), and `DevServerGitProvider`'s 9
   previously-"not supported" methods now wired (`BUG-BE-HLD-018`). The team
   is actively closing the concrete, small-scope gaps; the large aspirational
   architecture (§3.1 above) remains untouched.

7. **Minor, concrete, easy to fix if anyone's debugging a connection issue**:
   `backend/src/main/dev-server/agent-ws-server.ts:103`'s runtime error
   message tells operators to set `ORCA_URL=ws://<host>:6768/agent` — the
   actual listening port is `httpPort` (default 6769, `rpcPort+1`). A
   `backend/` one-line fix, not `agent/`.

---

## Decisions made (2026-08-15)

1. **`desktop/src/relay/` sync — deferred, kept as documented tech debt.**
   Scope stays `agent/`-only this session. The real `relay-ssh` binary ships
   from `desktop/`, not `agent/`; the prior session's Part-B fixes (git.clone
   security bug, idle-timeout, pty.spawn args, git-exec-validator hardening)
   plus this audit's newly-found Part-B gaps (`ai.provider.*`, `shell.exec`/
   `notification.send`/`ai.complete`) remain absent from the shipped binary
   until a dedicated `desktop/` session applies the equivalent changes there.
2. **Finding §2.3 (`preflight.*` missing on Part A) — ✅ fixed.** Added
   `preflight.detectAgents`/`detectWindowsTerminalCapabilities`/
   `detectGhosttyConfig`/`setGitIdentity` to `agent-rpc-dispatch.ts` (Part A),
   new file `agent-preflight-handler.ts`, reusing Part B's already
   transport-agnostic `isCommandOnPathForRelay` and the shared
   `agent/src/main/{pwsh,wsl,git-bash}.ts` probes rather than duplicating
   them. `setGitIdentity` needed real BUG-AG-HLD-003-style identity storage
   to be an effective fix (not just an RPC method that stores data nobody
   reads) — extended `git-identity-registry.ts` with a WebSocket-keyed
   variant (`setConnectionGitIdentity`/`getConnectionGitIdentity`, since
   Part A has no multi-tenant numeric-`clientId` concept the way Part B's
   SSH relay daemon does — see that file's inline rationale) and wired
   `handleGitExec`/`handleGitExecStream`'s `commit` subcommand to inject the
   stored per-connection identity as `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL`/
   `GIT_COMMITTER_NAME`/`GIT_COMMITTER_EMAIL` env vars, never touching global
   git config. 12 new tests (`agent-preflight-handler.test.ts` +
   `agent-git-handler.test.ts`'s new "commit identity injection" block,
   including a leak-isolation test between two connections and a
   backward-compat no-`ws` fallback case). Full `agent/src/relay/` suite
   re-run: 1267 passed / 21 skipped, same 2 pre-existing unrelated failures
   as every prior pass this session (confirmed via `git stash` earlier),
   zero new regressions. Build (`node build.mjs`) clean.
3. **Findings requiring `backend/` changes** (`StepExecutors.ts`'s
   `agent.exec` params, `agent-ws-server.ts`'s port typo, the `gh`/`glab`
   control-plane violation, the remaining Part-B-only gaps from §2) — not
   acted on, flagged for a future `backend/` session per this session's
   stated scope.
4. **The docs/ADR drift (§3)** is informational — none of it implies an
   `agent/` code change (either the architecture was deliberately not built,
   or it's a `backend/`-side gap). Left as-is; revisit if the ADRs themselves
   should be annotated/updated separately.
5. **Finding §2.5 (`git.clone`/`fs.listDirectory` missing on Part A) — ✅
   fixed.** Two new files: `agent-git-clone-handler.ts` (mirrors
   `GitHandler.cloneSimple()`'s `{url,targetPath}` validation — reject a
   leading `-`/embedded NUL — and streams `git.clone.output` via the Part A
   `makeNotifier` convention) and `fs-agent-directory-browse.ts` (near-verbatim
   port of Part B's `FsDirectoryBrowserHandler.listDirectory()`). Both wired
   into `agent-rpc-dispatch.ts`. As noted in the table above, this consumer
   (`repo-remote-ipc.ts`'s `repo.cloneRemote`/`repo.listRemoteDirectory`/
   `repo.scanRemote`) was already flagged UI-unreachable in the prior pass —
   fixed anyway since it's a real gap on the wire contract regardless of
   today's UI reachability. 10 new tests
   (`agent-git-clone-handler.test.ts`, `fs-agent-directory-browse.test.ts`).
   Full `agent/src/relay/` suite re-run: 1277 passed / 21 skipped, same 2
   pre-existing unrelated failures as every prior pass this session, zero
   new regressions. Build (`node build.mjs`) clean.
6. **2026-08-16 — `backend/` back in scope, per a new coordination/
   execution-split directive** (see [`gaps-and-findings.md`](./gaps-and-findings.md)'s
   2026-08-16 note). Closed §2 finding #1 (`agent.exec` param-shape backend
   bug — new `agent.execPrompt` RPC) and the ADR-018 gh/glab-in-backend
   violation this document's §3 item 2 flagged as `backend/`-only (new
   `github.exec`/`gitlab.exec` RPCs + 4 provider classes + a no-local-fallback
   rewrite of `ghExecFileAsync`/`glabExecFileAsync`) — see
   `gaps-and-findings.md` #10/#11 for full detail. Also closed the small
   `agent-ws-server.ts` port typo (§3 item 7) and the `git.worktree.list`/
   `fs.mkdir`+`fs.rmdir` Part-B name aliases (§2 row 6). Findings §2 #2
   (`ai.provider.*` missing on Part B) and #4 (`shell.exec`/
   `notification.send`/`ai.complete` missing on Part B) remain open — still
   blocked on the `desktop/` sync deferred in decision §1 above.
