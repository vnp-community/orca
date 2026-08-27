# TASK-036: Remote headless-browser driving on the Dev Server Agent (TASK-036 option b)

**Priority:** P1 — implemented this pass
**Services:** `agent` (new), `backend-go/services/api-gateway` (extended — 3 new ops on already-existing plumbing), `frontend` (gap documented, not changed)
**Status:** `[partial]` — agent + backend-go layers real, built, and tested end-to-end against a real launched headless Chromium. Frontend dispatch to this path is a documented, unclosed gap (see "Frontend" section below). See per-layer status lines at the bottom.

---

## Context

Per the "Dispatch-model disambiguation — RESOLVED" finding already on this
file before this pass (kept below for reference), the OLD `browser.*`
(`backend/src/main/browser/agent-browser-bridge.ts`, `AgentBrowserBridge`)
drives Electron's own embedded `WebContents` via CDP, co-located with the
desktop process — genuinely incompatible with SSH-relay/backend-go dev
servers, which have no Electron process to attach to. `worktreeId` there is
a local multiplexing key only, never a remote-dispatch signal.

**User's explicit product decision (option b):** commission a genuinely new
remote-headless-browser design — frontend initiates → backend-go relays →
the Dev Server Agent (`agent/`) actually drives a real headless browser
process running ON the remote dev server host. This is what this pass
builds.

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_browser.go`
and its `worktree → connectionId` resolution (via
`infrafleetv1.ResolveConnectionRequest.WorktreeId`,
`infrafleetv1.ResolveConnectionResponse.ConnectionId`) were **already real
and merged** on this branch before this pass, covering 9 ops (SOL-006
Groups A/B: `eval`/`keypress`/`mouseDown`/`mouseMove`/`mouseUp`/
`mouseWheel`/`viewport`/`tabCreate`/`tabClose`) and already wired into
`RegisterRealChannels`. This pass's own implementing agent's worktree was
mistakenly branched from a much older commit that predated all of this
(and consequently spent effort reconstructing an already-solved
worktree-resolution layer from scratch) — that reconstruction (a duplicate
`WorktreeConnectionResolver` port, a duplicate proto field, a duplicate
`infra-fleet-service` usecase/repository method) was **discarded, not
merged**, since the real thing it duplicated already existed and worked.
Only the agent's genuinely new work — the agent-side browser driver, its
tests, and 3 new op names appended to the already-real `channels_browser.go`
op list — was merged. This note exists so a future reader isn't confused by
any residual references to that reconstruction in other branches/history.

---

## What was built

### 1. Agent (`agent/src/relay/browser-handler.ts`, new)

Implements 12 `browser.*` JSON-RPC methods for real, registered in
`agent/src/relay/agent-rpc-dispatch.ts`'s switch (Part A — the surface
`infra-fleet-service`'s `Relay` RPC actually reaches; NOT the legacy
`relay.ts`/`RelayDispatcher`, which is Electron-desktop-only):

| Method | Real CLI op | Notes |
|---|---|---|
| `browser.goto` | `open <url>` | additive beyond SOL-006's original 9-op list |
| `browser.snapshot` | `snapshot` | additive; returns accessibility-tree + refs |
| `browser.click` | `click <selector-or-@ref>` | additive |
| `browser.eval` | `eval <js>` | SOL-006 Group A |
| `browser.keypress` | `press <key>` | SOL-006 Group A |
| `browser.mouseMove` | `mouse move <x> <y>` | SOL-006 Group A |
| `browser.mouseDown` | `mouse down [button]` | SOL-006 Group A |
| `browser.mouseUp` | `mouse up [button]` | SOL-006 Group A |
| `browser.mouseWheel` | `mouse wheel <dy> [dx]` | SOL-006 Group A |
| `browser.viewport` | `set viewport <w> <h>` | SOL-006 Group A |
| `browser.tabCreate` | `tab new` | SOL-006 Group B |
| `browser.tabClose` | `tab close [tabId]` + conditional session teardown | SOL-006 Group B |

**Engine choice — `agent-browser` (vercel-labs), not a new CDP client or
chrome-launcher:** already a vendored dependency
(`agent/package.json: "agent-browser": "~0.27.0"`) and, on inspection, is the
**exact same engine** the OLD Electron bridge already shells out to
(`AgentBrowserBridge.execAgentBrowser` in `agent-browser-bridge.ts` calls this
same CLI, just with `--cdp <electronProxyPort>` to attach to the Electron
webview instead of launching its own browser). It is a real, standalone CLI
(native per-platform binaries, `bin/agent-browser-{platform}-{arch}`) that:
finds or launches a real headless Chrome/Chromium on its own, speaks CDP
internally, and exposes exactly the navigate/click/eval/mouse/tab vocabulary
this task needs as `--json`-output subcommands. Verified in this sandbox: no
system `chromium`/`google-chrome` binary was on `PATH`, but `agent-browser`
located a Playwright-cached Chromium (`~/.cache/ms-playwright/chromium-*`)
and launched it headlessly (`--headless=new`) without any `DISPLAY` set —
confirmed via `ps -eo pid,ppid,cmd` showing the full Chrome process tree.

**Session-scoping/cleanup model (decided, documented in
`browser-handler.ts`'s header comment):**
- One `agent-browser` **session per worktree**, via `--session <worktreeId>`.
  `agent-browser` runs its own persistent background daemon per session name
  (confirmed: the daemon process reparents to pid 1 and survives the
  invoking short-lived CLI process) — this file does not manage a long-lived
  child process itself; each RPC call is a short CLI invocation.
- **Idle timeout (primary safety net):** every invocation sets
  `AGENT_BROWSER_IDLE_TIMEOUT_MS=900000` (15 min) so an abandoned worktree's
  daemon + Chrome process self-terminates. Verified this matters: without
  it, three separate test sessions left ~15 Chrome subprocesses each running
  indefinitely in this sandbox after the CLI invocation that spawned them
  exited.
- **Explicit teardown:** `browser.tabClose` closes the tab, checks
  `tab list`, and if no tabs remain, also runs `close` to tear the whole
  session down immediately rather than waiting out the idle timeout.
- **Documented gap, not silently assumed:** no teardown tied to the
  Orca↔agent WebSocket connection closing — `agent-rpc-dispatch.ts`'s
  per-connection `WireState` isn't plumbed into this handler in this pass.
  The idle timeout is the safety net for a dropped connection.

**Operational requirement surfaced, not hidden:** the target host needs a
Chrome/Chromium `agent-browser` can find (or reach for its own first-run
download). `runBrowserCommand()` pattern-matches agent-browser's own
"no usable browser"/"failed to launch"/"executable doesn't exist" failures
and remaps them to a `BROWSER_ENGINE_UNAVAILABLE: ...` error (vs. an opaque
`BROWSER_COMMAND_FAILED`), so a missing-Chrome host fails clearly on first
use instead of silently.

**Tests:**
- `agent/src/relay/browser-handler.test.ts` — 20 tests, `node:child_process`
  execFile mocked (no real Chrome needed): arg construction per op, session
  scoping (`--session <worktreeId>`), idle-timeout env var set on every call,
  validation errors (missing worktree / missing required param) fail fast
  without spawning, CLI-failure → `BROWSER_COMMAND_FAILED`, "no usable
  browser" → `BROWSER_ENGINE_UNAVAILABLE`, tabClose's
  close-tab-then-maybe-close-session branching (both branches).
- `agent/src/relay/browser-handler.real-browser.test.ts` — a REAL Chrome
  WAS available in this sandbox (via Playwright's cache, not a system
  package) — 4 additional tests run a real `agent-browser`-launched headless
  Chromium end-to-end (`goto` against example.com, `snapshot` returns a real
  accessibility tree, `click` on the real "Learn more" link, `tabCreate`
  opens a second real tab), gated by a synchronous top-level probe so the
  suite skips cleanly when no browser is reachable rather than failing CI.
  Re-verified on this branch's actual merge target: 4 passed, 1 skipped.
- `cd agent && npx vitest run src/relay/browser-handler.test.ts
  src/relay/browser-handler.real-browser.test.ts` — 24 passed, 1 skipped.
- `npx tsc --noEmit`: zero errors in `browser-handler.ts` or
  `agent-rpc-dispatch.ts` (confirmed against this branch's real HEAD, which
  has its own separate, pre-existing set of ~100 unrelated `tsc` errors from
  other in-flight work — neither touched file appears in that output).

### 2. Backend-go — 3 new ops on already-real plumbing

`channels_browser.go`'s `registerBrowserChannels` now covers all 12 ops
(SOL-006's original 9 relayed ops + this task's 3 additive navigate/
inspect/interact ops: `goto`/`snapshot`/`click`), all sharing the same
resolve-then-relay `registerBrowserRelay` helper that was already real and
tested — no new backend-go design needed for the additive 3, they are just
3 more entries in the same op list. `channels_browser_test.go`'s existing
`TestBrowserChannels_AllGroupAAndBChannels_ResolveThenRelay` table extended
to cover them.

**Verify:**
```
cd backend-go/services/api-gateway
go build ./... && go vet ./... && go test ./...   # clean
```

### 3. Frontend — investigated, NOT changed this pass (documented gap)

Found two browser-pane components:
- `browser-pane-local.tsx` (webview-embedded) uses `window.api.browser.*`,
  which the web-mode preload (`frontend/src/renderer/src/web/web-preload-api.ts`,
  `createBrowserApi()`) stubs out entirely to no-ops
  (`registerGuest: () => Promise.resolve()`, etc.) — confirms this pane is
  genuinely Electron-webview-only, matching this task's original finding.
- `browser-pane-remote.tsx` ("RemoteBrowserPagePane") calls
  `callRuntimeRpc(target, 'browser.goto', ...)` against a REAL
  `{kind:'local'} | {kind:'environment', environmentId}` dispatch branch
  (`runtime-rpc-client.ts`) — at first glance, exactly the local-vs-remote
  branch this task's brief predicted might not exist.

  Tracing `target.kind === 'environment'` further: it resolves through
  `window.api.runtimeEnvironments.call`, which — per
  `desktop/src/main/ipc/runtime-environments.ts` and
  `web-preload-api.ts`'s `createRuntimeEnvironmentsApi()` (pairing-code /
  `createStoredWebRuntimeEnvironment` / WebRTC offer-based) — is a **third,
  separate "remote" concept**: pairing this Orca client to *another full
  paired Orca desktop instance*, not `infra-fleet-service`'s SSH-relay dev
  servers. It is unrelated to the `browser.*` wscompat channels this task
  builds on.

  **No frontend code path was found that maps a worktree bound to a
  backend-go/`infra-fleet-service` dev-server connection to a call through
  `api-gateway`'s wscompat WebSocket surface for `browser.*` specifically.**
  Other already-migrated backend-go features (`git.*`, `devServer.*`,
  `fleet.*`) reach `api-gateway` through a different client path than
  `RemoteBrowserPagePane`'s `runtimeEnvironments` mechanism — matching the
  same architecture gap this whole session independently found for
  `accounts.*` (TASK-023) before BUG-005's fix narrowed it for that specific
  namespace's transport. The equivalent fix for `browser.*` is NOT simply
  "reuse BUG-005" — `RemoteBrowserPagePane` would need a genuinely new
  local-vs-backend-go-SSH-relay dispatch branch (which existing "remote"
  abstraction, if any, should own this, and whether
  `RemoteBrowserPagePane`'s screencast-frame protocol generalizes over an
  `api-gateway` WebSocket round-trip the same way it does over the
  IPC/WebRTC paths it has today) — a genuine, separately-scoped frontend
  architecture decision, not implemented this pass per this task's own
  instruction not to force a rushed frontend change.

  A client capable of calling `api-gateway`'s wscompat `browser.*` channels
  with a resolved `worktree` selector (e.g. a test script, or a future
  frontend change) can exercise the full agent+backend-go path today.

---

## Status by layer

- **Agent (`agent/src/relay/browser-handler.ts` + dispatch wiring):** `[x]`
  DONE — real, builds clean (`tsc --noEmit` zero errors in touched files),
  20 mocked unit tests + 4 real-headless-Chrome integration tests all pass.
- **Backend-go (`api-gateway`'s `channels_browser.go`, 3 new ops on
  already-real plumbing):** `[x]` DONE — real, builds clean, `go vet`
  clean, all tests pass (new + pre-existing, nothing broken).
- **Frontend (browser-pane dispatch to this new relay path):** `[ ]` NOT
  DONE — real, evidence-backed gap documented above; no frontend code
  changed this pass; needs a separate, deliberately-scoped design decision
  before implementation.
- **Not verified against a real remote SSH-connected dev server host** — all
  verification above ran against a sandbox's own local process tree (agent
  process + backend-go services running in the same environment, not over
  an actual SSH relay hop). The `Relay`/`ResolveConnection` RPC plumbing is
  the same mechanism every other already-shipped relayed feature (`git.*`,
  `devServer.*`) uses, but this specific new surface has not been
  end-to-end tested against a live SSH-relay connection.

---

## Dispatch-model disambiguation — RESOLVED (kept from before this pass)

`browser.*`'s OLD backend, `backend/src/main/browser/agent-browser-bridge.ts`'s
`AgentBrowserBridge`, is confirmed **Electron-process-local**, not a remote
Dev Server Agent (`agent/`) relay at all:

- It imports `electron` (`app`, `WebContents`), `node:child_process`'s
  `execFile`, and `acquireElectronDebugger`/`CdpWsProxy` — it launches and
  drives a real Chrome/Chromium process via the Chrome DevTools Protocol
  **co-located with the desktop app's own process**, not over SSH/relay to
  a separate host.
- `worktreeId` (threaded through every handler in `orca-runtime-browser.ts`,
  e.g. `browserSnapshot`/`browserClick`/`browserGoto`) is confirmed to be a
  **local multiplexing key only** — `activeWebContentsPerWorktree:
  Map<worktreeId, webContentsId>` selects which already-open local Electron
  `WebContents` (browser tab) a given worktree's pane currently targets. It
  is never used to select a remote host/connectionId.
- "Agent" in `AgentBrowserBridge` does not refer to the Dev Server Agent
  (`agent/`) package at all — naming coincidence, not an architectural link.

This pass's new capability (agent-launched headless Chromium, driven over
`Relay`) is genuinely new, not a port of the above.
