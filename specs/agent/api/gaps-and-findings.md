# Gaps, Bugs &amp; Drift — Cross-Cutting Findings

Everything found during this audit that's a genuine inconsistency, bug, or
piece of documentation drift, rather than a plain description of intended
behavior. Consolidated here (instead of scattered across the other files)
because several of these span both directions and both transports. Each
finding also has a pointer from the catalog file where it's most relevant.

## Resolution status

**2026-08-15, pass 1 (agent-only):** `backend/` was mid-restructure at the
time, so that round scoped fixes to `agent/` only.

**2026-08-15, pass 2 (backend):** closed out the three items pass 1
deferred to `backend/` — #5 turned out to be a real, live bug (not just a
naming inconsistency) affecting the default connection mode; #9 was
confirmed-safe dead code, deleted; #2 was investigated and confirmed to have
no reachable consumer to build for any of its three methods, so it stays
doc-only by design, not by default.

**2026-08-15, pass 3 (agent-only, `desktop/` explicitly out of scope):**
closed the security-relevant half of #4 — Part A's `git.exec` had no
protection against git's own injection/RCE flags (`-c core.sshCommand=`,
`--upload-pack=`, unrestricted `config` writes), independent of the
larger, still-deferred contract-unification work. Note:
`desktop/src/relay/agent-git-handler.ts` has the identical
`validateGitArgs` function (a parallel/near-duplicate tree, same pattern as
`desktop/src/main/ipc/onboarding-ipc.ts` noted in pass 2) — not patched,
per explicit scope instruction.

| # | Finding | Status |
|---|---|---|
| 1 | Missing `shell.exec`/`notification.send`/`ai.provider.testConnection` handlers | ✅ **Fixed** (pass 1) — all three implemented agent-side |
| 2 | Orphaned agent→backend pushes | ✅ **Resolved as doc-only** (pass 2) — investigated; all three are confirmed dead ends with no reachable consumer, see below |
| 3 | `git.clone` handler-override bug | ✅ **Fixed** (pass 1) — shapes merged, validated, duplicate-registration now warns |
| 4 | Part A/B contract divergence (`preflight.check`, `pty.exit`, `fs.changed`, `git.exec` whitelist) | 🟡 **Partially fixed** (pass 3, agent-only) — `git.exec`'s injection/RCE gap closed; `preflight.check`/`pty.exit`/`fs.changed` shape mismatches and full RPC-surface parity still deferred (too large/risky) |
| 5 | `pty.spawn` vs `pty.create` naming inconsistency | ✅ **Fixed** (pass 2) — turned out to be a live bug (5 call sites, 2 backing a real UI button, broken on the default connection mode), not just a naming quirk |
| 6 | No path confinement in Part B `fs.*` (broken doc reference) | ✅ **Fixed** (pass 1, doc only) — `docs/relay-fs-allowlist-removal.md` restored; security posture intentionally left unchanged (it's the documented-deliberate design) |
| 7 | TDD docs stale | ✅ **Fixed** (pass 1) — `specs/agent/tdd/v5/00-index.md` now points here |
| 8 | `AGENT_TIMEOUT_MS`/`TIMEOUT_MS` idle-timeout unenforced | ✅ **Fixed** (pass 1) — enforced on both Stack A and Stack B (agent-side) |
| 9 | Backend's vendored `dispatcher.ts` unused | ✅ **Fixed** (pass 2) — confirmed dead, deleted |

## 1. Confirmed missing agent-side handlers (backend calls, no agent response)

**Status: ✅ Fixed.** All three now have real agent-side handlers:
`shell.exec` → `handleShellExec` (`fs-agent-extensions.ts`, registered
`agent-rpc-dispatch.ts`); `notification.send` → `handleNotificationSend`
(new file `notification-send-handler.ts`, best-effort OS notification +
always-acknowledge, since the headless agent has no general slack/email
delivery infra — that belongs on the backend); `ai.provider.testConnection`
→ `handleTestConnection` (`agent-credential-store.ts`, a thin alias of
`handleHealthCheck`, matching the pattern already used in the separate
`desktop/` codebase's reference implementation). The test file's
"documented gap" block was updated to assert the new real behavior instead
of `MethodNotFound`, plus new coverage for the idle-watchdog and merged
`git.clone` fixes (findings #3 and #8). See
[`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md) "Confirmed
gaps" section — kept for context on the pre-fix state.

| Method | Backend caller | Status | Impact |
|---|---|---|---|
| `shell.exec` | `backend/src/main/workflow/StepExecutors.ts:194-198` (`executeShell()`) | **No handler in `agent/src`.** An agent-side test explicitly documents this as a known gap: `agent/src/relay/__tests__/agent-rpc-dispatch.test.ts:387-408`, block `'shell.exec / notification.send — documented gap (CR-TRACE-017)'`, asserting the call returns `MethodNotFound`. | Any workflow with a `shell` step fails at runtime. |
| `notification.send` | `StepExecutors.ts:236-240` (`executeNotification()`) | **No handler anywhere.** Same test confirms `MethodNotFound`. No `notification.*` namespace of any kind exists agent-side. | Any workflow with a `notification` step fails at runtime. |
| `ai.provider.testConnection` | `backend/src/main/ai-providers/AIProviderService.ts:483` | **No handler in `agent/src`.** A method of this exact name exists only in the separate `desktop/src/relay/ai-provider-handler.ts` codebase (out of `agent/`/`backend/` scope), suggesting a fork/divergence rather than intentional omission. | The "test connection" UI action against a dev-server-backed AI provider account fails. |

Full detail: [`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md)
"Confirmed gaps" section.

## 2. Orphaned agent→backend pushes (agent sends, no backend consumer found)

**Status: ✅ Resolved as doc-only (investigated in the pass-2 backend
review; no code change, by design).** All three are confirmed dead ends —
building a consumer for any of them would be new feature work disguised as
a bug fix, not a fix:

- **`agent.spawn`/`kill`/`sendInput` + `agent.output`/`agent.exited`**:
  exhaustive re-grep of `backend/src` confirmed zero callers anywhere —
  `task-types.ts`/`tracers.ts`'s `agent.spawn` mentions are stale
  doc-comments with no code near them. The only spawn RPC the backend
  actually uses is the non-interactive `agent.exec`
  (`ProfileAwareAgentSpawner.ts:130`, `StepExecutors.ts:107`). A generic
  multi-user proxy tunnel (`session-manager.ts:311`'s `relayCall`) *could*
  carry these method names but no call site ever supplies them. Wiring a
  consumer would mean building real interactive-agent-session management
  (input/output plumbing, a session store, UI) — out of scope for closing a
  gap.
- **`git.execStream`**: zero backend callers, confirmed by grep across all
  of `backend/src`. Not a gap — the SSH transport already has a separate,
  working streaming mechanism for the same need
  (`__streamResponse`/`git.responseChunk`, `ssh-git-response-stream-reader.ts`),
  reached via a different RPC method (`git.exec`/`git.diff` with
  `__streamResponse: true`), not `git.execStream`.
- **`git.clone.output`**: the agent-side emission is real and correct
  (`git-handler.ts`'s `spawnCloneSimple`, moved there from the now-deleted
  `git-handler-clone.ts` by the finding #3 fix). Its only plausible
  consumer, `backend/src/main/ipc/repo-remote-ipc.ts`'s `repo.cloneRemote`
  IPC handler, is registered and functional but **unreachable from any
  current UI** — the frontend's real "clone by URL" flow
  (`AddRepoStep.tsx`/`useAddRepoCloneFlow.ts`) calls a different Electron
  IPC channel (`'repos:cloneRemote'`, colon-separated) backed by a
  completely separate, SSH-based implementation
  (`desktop/src/main/ipc/repos.ts`) that already has its own working
  progress mechanism via `git.cloneProgress`/`'repos:clone-progress'`.
  `repo-remote-ipc.ts`'s `repo.cloneRemote` (dot-separated channel) looks
  like dead/parallel-track code, not a gap to close — left as-is rather
  than removed (a call to make together with whoever's mid-restructuring
  `backend/`, not unilaterally here).

Full detail: [`backend-rpc-catalog.md`](./backend-rpc-catalog.md) §1.

## 3. `git.clone` handler-override bug (security-relevant)

**Status: ✅ Fixed.** Investigating the real backend call sites (read-only,
no `backend/` files changed) found this was worse than "one handler shadows
a safer one" — the two shapes are **both genuinely used** by different live
callers (`repo-remote-ipc.ts`'s `{url, targetPath}` and
`ssh-git-provider.ts`'s `{args, cwd, progressId}`), so simply reordering
registration would have broken whichever shape lost. The fix:

- `GitHandler` now registers a single `git.clone` handler
  (`handleClone`, `git-handler.ts`) that branches on the param shape and
  routes to the existing validated `clone()` (args-based) or a new
  `cloneSimple()`/`spawnCloneSimple()` (url/targetPath-based).
- `cloneSimple()` adds the validation the url/targetPath shape never had:
  rejects a leading `-` or an embedded NUL byte in either `url` or
  `targetPath` (the same argv-injection class `git-exec-validator.ts`
  guards against for the other shape), and uses `buildRelayGitEnv()`
  (pinned locale) instead of raw `process.env` for consistency with every
  other git spawn in this file.
- `GitCloneHandler`'s standalone second registration is gone —
  `git-handler-clone.ts` was deleted, and `relay.ts` no longer constructs it.
- `RelayDispatcher.onRequest`/`onNotification` (`dispatcher.ts`) now log a
  warning on any duplicate registration for the same method name, so this
  exact bug class (silent last-registration-wins) is caught at wiring time
  in the future instead of surfacing as a confusing runtime param mismatch.
- New test coverage: `git-handler.test.ts` "git.clone (merged shapes)"
  (both shapes routed + validated correctly, real local clone + rejection
  cases).

Original finding, for context: `RelayDispatcher.onRequest` did a plain
`Map.set(method, handler)` with no duplicate-key protection.
`relay.ts:472` constructed `GitHandler` first, registering `git.clone` →
the validated `GitHandler.clone`. `relay.ts:482` then constructed
`GitCloneHandler`, registering `git.clone` again → the unvalidated
`GitCloneHandler.cloneRepo` — the second registration won, silently
discarding the first, so every `git.clone` call was served by
`spawn('git', [...], {env: process.env})` built directly from
client-supplied `url`/`targetPath` with no argument whitelist and no path
validation.

Full detail: [`agent-rpc-catalog-git-fs.md`](./agent-rpc-catalog-git-fs.md)
"git.clone (merged shapes)" / "git.exec / clone" sections.

## 4. Same method name, different contract, across Part A vs Part B

**Status: 🟡 Partially fixed.** This finding bundled two different classes
of problem — a **security gap** (Part A's `git.exec` had no per-flag
denylist) and a **contract/shape mismatch** (`preflight.check`, `pty.exit`,
`fs.changed` return different shapes on each Part). Only the security half
is fixed; the shape-mismatch half is unchanged and still deferred.

**Fixed — Part A `git.exec` injection/RCE hardening** (agent-only, no RPC
surface change): cross-referencing `DevServerGitProvider`'s real
`relay.call()` sites confirmed Part A has no dedicated `git.stage`/
`git.commit`/`git.push`/`git.pull`/`git.fetch`/`git.checkout`/etc. — only
Part B does — so Part A's `git.exec` genuinely needs to keep accepting
`push`/`pull`/`fetch`/`merge`/`rebase`/`stash` with real arguments; porting
Part B's ~20 dedicated per-operation RPCs to Part A first (the only way to
*also* close the contract-shape gap below) remains its own, larger,
higher-risk project, not attempted here. What *was* closed: Part A had
**zero** protection against git's own injection/RCE footguns, independent of
subcommand shape — a `-c core.sshCommand=...` (or any other flag) before the
subcommand, `--upload-pack=`/`--receive-pack=`/`--exec=` (git's documented
local-command-execution vectors for fetch/pull/push/archive),
`-o`/`--output` (arbitrary file write), and unrestricted `git config`
writes (`--file` path traversal, planting a `core.hooksPath`). New file
`agent-git-exec-validator.ts` adds these checks, layered onto
`agent-git-handler.ts`'s existing subcommand allowlist +
shell-metacharacter check — deliberately does **not** block
`--git-dir`/`--work-tree`/`--exec-path` *after* the subcommand (only their
dangerous pre-subcommand form), since `dev-server-git-provider.ts` has a
real, currently-working `git.exec(['rev-parse', '--git-dir'])` caller where
the flag is a benign query, not a redirect.

**Still deferred — contract/shape unification**: the agent runs two
independent RPC surfaces (`connection-modes.md` §0) that happen to share
several method *names* with **different parameter/return shapes**:

| Method | Part A (`direct-websocket`) contract | Part B (`relay-ssh`) contract |
|---|---|---|
| `preflight.check` | `{services: ('github-cli'\|'ripgrep'\|'docker'\|'claude')[]}` → `Record<string,boolean>` (binary-availability probe only) | no params → `{platform, gh:{...}, glab:{...}, git:{...}}` (full install+auth+identity check) |
| `pty.exit` (notification) | `{id, exitCode, signal}` | `{id, code}` |
| `fs.changed` (notification) | `{path, eventType, filename}` — one event per notification | `{events: [{kind, absolutePath, isDirectory?}]}` — batched array |
| `git.exec` (whitelist) | 21 subcommands allowed (`status, diff, add, restore, commit, push, pull, fetch, branch, checkout, merge, rebase, stash, log, worktree, remote, tag, show, rev-parse, config, describe, shortlog`), now with an injection/RCE-flag denylist (see above) but still no per-subcommand shape restriction | 14 subcommands allowed, each with its own extra per-flag/shape restriction (see git/fs catalog); `push/pull/fetch/merge/rebase/stash/worktree` are **not allowed at all** |

**Confirmed real backend caller for `preflight.*` and most PTY traffic talks
to Part B.** A generic backend handler that assumes one contract for a
shared method name will still silently misbehave against the other
transport for `preflight.check`/`pty.exit`/`fs.changed` — that part of this
finding is unchanged.

Full detail: both `agent-rpc-catalog-*.md` files.

## 5. `pty.spawn`/`pty.create` — a real, live bug, not just naming

**Status: ✅ Fixed.** Investigating this for the backend fix pass found it
was much bigger than "one mismatched method name":

- **Five call sites**, not one, all doing a raw, connection-type-unaware
  `relay.call('pty.spawn', {command, args, ...})`:
  `onboarding-ipc.ts` (`onboarding.openGhAuthTerminal`),
  `runtime/rpc/methods/github-auth.ts` (`github.startAuthLogin`,
  `github.revokeAuth`), `runtime/rpc/methods/gitlab-auth.ts`
  (`gitlab.startAuthLogin`, `gitlab.revokeAuth`). The GitHub/GitLab pair
  back a real, live Settings button
  (`WebModeCliAuthSection.tsx`) — this wasn't dead code, it was a **broken
  feature** on the default connection mode.
- **Bug 1**: `pty.spawn` doesn't exist for `direct-websocket`/
  `relay-websocket` (the default mode) — the agent's WS dispatch table only
  registers `pty.create`, which had no command-injection param at all.
  Every call from these 5 sites threw `MethodNotFound`.
- **Bug 2**: even on `relay-ssh` (the one mode where `pty.spawn` exists),
  `args` was silently dropped — `pty-handler.ts`'s `spawn()` only ever read
  `params.command` (typed into the shell once ready), never
  `params.args`. So `{command: 'gh', args: ['auth', 'login']}` typed
  literally `gh` into the shell, not `gh auth login`.
- **Bug 3** (found while fixing bug 1): `DevServerPtyProvider.spawn()` —
  the standard `IPtyProvider` implementation every other PTY caller in the
  codebase goes through for `direct-websocket` dev servers — also never
  forwarded `opts.command`/`opts.commandDelivery` to `pty.create`, unlike
  `SshPtyProvider.spawn()`. `commandDelivery: 'provider'` is the
  already-shipping pattern `orca-runtime-terminal-create.ts` uses for
  AI-agent terminal launches and pane splits — so this wasn't
  auth-login-specific; **any** provider-delivered-command PTY spawn was
  silently dropping its command on `direct-websocket` dev servers.
- **Bug 4** (found while fixing bug 2): `gitlab-auth.ts`/`github-auth.ts`
  typed their result as `relay.call<string>('pty.spawn', ...)`, but the
  agent's `pty.spawn` returns an object (`{id, cols, rows, cwd, shell}`),
  not a bare string — the returned `ptyId` was actually the whole result
  object at runtime.

**Fix**: `DevServerPtyProvider.spawn()` now forwards `command`/
`commandDelivery`/`envToDelete`/`userId` to `pty.create` (fixes bug 3 for
every caller, not just these 5). The 5 call sites now go through
`getRemotePtyProvider(devServerId)` (the existing, already-live
`IPtyProvider` registry `dev-server-provider-lifecycle.ts` populates —
mirrors `ssh-git-dispatch.ts`'s pattern for git/fs) instead of a raw
`relay.call`, building a single shell-quoted command string
(`backend/src/shared/posix-shell-quote.ts`, new — the string is typed as
literal shell keystrokes, so an unescaped `--hostname` value was a real
shell-injection risk, not just a correctness one) instead of a separate,
silently-ignored `args` array. Agent-side: `pty-handler.ts`'s
`BUG-BE-HLD-005` `command === 'gh'` exact-match (for
`GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per-user isolation) became a prefix match
since `command` now carries the full line; `pty-agent-bridge.ts` (Part A)
gained `command`/`commandDelivery`/`userId` support — a fixed
`STARTUP_COMMAND_WRITE_DELAY_MS`-style delay (50ms, matching Part B's
non-shell-ready-gated default), not the full OSC shell-ready-marker
scanning Part B uses for its more finicky multiline-prompt cases, since
Part A had none of that machinery (`pty-shell-launch.ts` usage) wired in at
all before this fix and porting it wasn't needed to close this bug.

Full detail: [`agent-rpc-catalog-runtime.md`](./agent-rpc-catalog-runtime.md)
"pty.\*" sections.

## 6. No path confinement in Part B's `fs.*` surface (deliberate, documented)

**Status: ✅ Fixed (doc only).** `docs/relay-fs-allowlist-removal.md` has
been restored, explaining the trust-boundary rationale (the relay runs as
the SSH user; `pty.spawn`/`git.exec` already reach anywhere that user can)
and cross-referencing what's still enforced (`git.exec`'s whitelist,
`git.clone`'s new argv-injection guard from finding #3, the Windows
shell-override restriction, terminal-artifact identity checks). The security
posture itself is **intentionally unchanged** — this finding's own title
already says "deliberate, documented"; the actual defect was the broken doc
reference, not the absence of confinement.

Part B's `FsHandler` (`fs-handler.ts` — `readDir/readFile/writeFile/stat/
lstat/deletePath/createFile/createDir/rename/copy/realpath/search/listFiles`)
has **no `SecureFs`, no allowlist, no root-confinement, no `..`-traversal
rejection** anywhere. `agent/src/relay/context.ts:20-29` states this was
intentionally removed because "the relay runs as the SSH user and trusts the
renderer process... a compromised renderer can already weaponize
`pty.spawn`/`git.exec` to reach any path the SSH user can reach", citing a
doc at `docs/relay-fs-allowlist-removal.md` that **does not exist in the
repo** — a broken/stale doc reference worth fixing regardless of whether the
underlying security posture is intentional.

Contrast with Part A: only `fs.writeFile` has real path confinement
(`resolvedPath.startsWith(resolvedWork + '/')`); every other Part A `fs.*`
handler (`readDir`, `readFile`, `grep`, `stat`, `glob`, `mkdir`) resolves
`isAbsolute(raw) ? raw : join(config.workDir, raw)` ad hoc, with an absolute
path honored unchanged. `fs.glob` is the weakest — `cwd` used directly with
no `isAbsolute` check at all. `fs.rmdir`'s check is not real confinement
either (`config.workDir.startsWith(absPath) || absPath === '/'` only blocks
deleting `workDir`/its ancestors or `/`; an unrelated absolute path passes
through to `rm(...,{recursive:true,force:true})`).

Full detail: [`agent-rpc-catalog-git-fs.md`](./agent-rpc-catalog-git-fs.md)
`fs.*` sections for both Parts.

## 7. TDD spec docs are stale relative to the actual RPC surface

**Status: ✅ Fixed.** `specs/agent/tdd/v5/00-index.md` now carries a pointer
to this `specs/agent/api/` doc set as the current source of truth for the
RPC surface, explaining why (method count grew substantially; the two
independent Part A/Part B surfaces aren't distinguished in the original
TDDs). The individual TDD files (`07`, `10`, `11`) were left as-is —
rewriting design-time documents to describe current behavior would erase
their value as a historical record of the original design intent; the
index pointer is the right level to redirect readers.

`specs/agent/tdd/v5/07-jsonrpc-dispatch.md`, `10-git-handler-extension.md`,
`11-fs-handler-extension.md` document only a narrow original v5.0 method set
(`git.exec`, `git.execStream`, `fs.readDir`, `fs.readFile`, `fs.grep`,
`fs.watch`/`fs.unwatch`/`fs.changed`). They do **not** mention `fs.stat`,
`fs.glob`, `fs.writeFile`, `fs.mkdir`, `fs.rmdir`, `git.history`,
`git.branchCompare`, `git.commitCompare`, `git.branchDiff`, `git.commitDiff`,
`git.checkIgnored`, `git.forkSync`, `git.submoduleStatus`, `git.worktree.*`,
`git.pr.create`, `shell.eval`, `preflight.*`, `ai.*`, `github.*`, `gitlab.*`,
`pty.*` — all added later — and never describe Part B (the SSH relay's
`GitHandler`/`FsHandler`, ~39+36 methods) at all. This spec doc set (this
directory, `specs/agent/api/`) supersedes those TDD files for the RPC-surface
question; the TDD files remain useful for architecture/deployment narrative
not covered here.

## 8. `AGENT_TIMEOUT_MS`/`TIMEOUT_MS` idle-timeout is declared but not enforced

**Status: ✅ Fixed.**

- **Stack A** (`agent-session.ts`): tracks `lastFrameReceivedAt`, updated on
  every successfully-decoded frame (data or keepalive). A watchdog reusing
  the existing 5s keepalive interval closes the connection
  (`ws.close(1001, 'idle timeout - no frames received')`) once
  `AGENT_TIMEOUT_MS` (20s) has elapsed with nothing received — the existing
  `ws.on('close', ...)` handler then runs the normal `stop()` cleanup
  (PTY/watch teardown) unchanged. New tests in `agent-session.test.ts`
  ("idle watchdog (AGENT_TIMEOUT_MS enforcement)") cover: closes after 20s
  idle, does *not* close while frames keep arriving, does not act on an
  already-closed `ws`.
- **Stack B** (`dispatcher.ts`, `RelayDispatcher`): each `RelayClient` now
  tracks its own `lastFrameReceivedAt`. The existing keepalive-send loop
  (already iterating every client every `KEEPALIVE_SEND_MS`) also checks
  idle time: a **non-primary** attached client past `TIMEOUT_MS` is
  `detachClient()`'d (that path was already safe/tested for normal
  disconnects). The **primary** client (the SSH exec channel itself) is
  only *warned about*, not force-closed — its lifecycle is managed
  externally via `setWrite()` on SSH reconnect, and forcing closure here
  risked racing that flow without full visibility into
  `ssh-relay-deploy.ts`/`SshConnectionManager` (out of this pass's read
  scope, see "Not investigated" below). This is a deliberately conservative
  partial enforcement, not a claim that Stack A and Stack B are now
  symmetric.
- Full `src/relay/` test suite (1256 tests) re-run after both changes: same
  2 pre-existing, unrelated failures as before this fix pass (a
  `subprocess.test.ts` esbuild resolution error and one `pty-handler.test.ts`
  assertion mismatch, both confirmed via `git stash` to predate this work),
  zero new regressions.

Both wire-protocol stacks declare a 20-second idle-timeout constant
(`agent-wire-protocol.ts:22`, `protocol.ts:44`) with a comment describing the
intended contract ("if no frame received in 20000ms → close connection"), but
no code path in the files read for this audit actually implements that
enforcement — only the keepalive *senders* were found, not a corresponding
receive-side watchdog timer. If this is load-bearing (e.g. relied on to
detect a genuinely dead peer faster than TCP-level detection would), it's
worth a dedicated verification pass; if it's dead/aspirational documentation,
the comment should be removed or the constant should be marked unused.

Full detail: [`connection-modes.md`](./connection-modes.md) §7.

## 9. Backend's vendored `backend/src/relay/dispatcher.ts` is unused

**Status: ✅ Fixed — deleted.** Re-verified for the backend fix pass: all 4
files in the directory (`dispatcher.ts`, `protocol.ts`,
`client-request-aborts.ts`, `fs-handler-directory-browse.ts`) were
unreachable at runtime, and no build entry point targeted the directory
(`backend/vite.config.ts` only builds `server/index.ts`,
`main/daemon/daemon-entry.ts`, `main/session/user-process-entry.ts`; the
real deployed `relay.js` comes from `agent/build.mjs`, unrelated to this
directory). The one live dependency was a **type-only** import —
`backend/src/main/ipc/repo-remote-ipc.ts` imported `DirectoryEntry` from
`fs-handler-directory-browse.ts` purely as a TS return-type annotation.
Fix: inlined `DirectoryEntry` directly into `repo-remote-ipc.ts` (single
consumer, not worth a shared module), deleted all 4 files plus the now-empty
`backend/src/relay/` directory, removed `"./src/relay/**/*"` from
`backend/tsconfig.json`'s `include`. `backend`'s typecheck error count and
full test suite (198 tests) were unchanged before/after (one pre-existing,
unrelated failure in `push-api-routes.test.ts` — a web-push VAPID-key test,
traced to in-flight changes elsewhere in the ongoing restructure, not this
deletion).

## Not investigated (out of scope for this pass)

- `fs.workspaceSpaceScan`'s underlying `scanWorkspaceSpaceDirectory()`
  (`agent/src/relay/workspace-space-scan.ts`) was not deep-dived.
- The PTY-daemon Unix-socket protocol (`pty-daemon-protocol.ts`) internals
  beyond what's needed to document the `pty.*` RPC surface.
- `ssh-relay-deploy.ts`/`SshConnectionManager` reconnection semantics for
  `relay-ssh` mode (referenced but not read in full).
- Whether `agent.output`/`agent.exited`/`git.clone.output`/`git.execStream`'s
  `stream.chunk`/`stream.end` are genuinely dead code vs. consumed through a
  path this audit's grep patterns didn't surface — recommend a `git blame`/
  history check before removing anything based on this doc alone.
