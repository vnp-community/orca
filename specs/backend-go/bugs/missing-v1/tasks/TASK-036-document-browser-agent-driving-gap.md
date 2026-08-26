# TASK-036: Document the browser-driving `agent/` gap and the desktop-automation product decision (blocked, no code)

**From Solution:** SOL-006
**Priority:** P3 — tracking only, blocks nothing in this repo's `backend-go` build
**Service:** `agent` (out of scope) / product decision
**File:** none — this task produces no code change; see "What to do" below
**Depends on:** TASK-034
**Status:** `[blocked]` — item 2 (dispatch-model disambiguation) is now DEFINITIVELY RESOLVED by tracing the real old TS backend source (see below) — this changes the picture for item 1's product decision, which is still genuinely not mine to make. Item 3 (agent-side capability) is now known to be the WRONG framing entirely, not just unstarted — see below. No code implemented; still correctly blocked, but for a different, now-clearer reason.

**Dispatch-model disambiguation — RESOLVED, backend.*/`agent/` framing was based on a documentation gap:**
`browser.*`'s real backend, `backend/src/main/browser/agent-browser-bridge.ts`'s `AgentBrowserBridge`, is confirmed **Electron-process-local**, not a remote Dev Server Agent (`agent/`) relay at all:
- It imports `electron` (`app`, `WebContents`), `node:child_process`'s `execFile`, and `acquireElectronDebugger`/`CdpWsProxy` — it launches and drives a real Chrome/Chromium process via the Chrome DevTools Protocol **co-located with the desktop app's own process**, not over SSH/relay to a separate host.
- `worktreeId` (threaded through every handler in `orca-runtime-browser.ts`, e.g. `browserSnapshot`/`browserClick`/`browserGoto`) is confirmed to be a **local multiplexing key only** — `activeWebContentsPerWorktree: Map<worktreeId, webContentsId>` selects which already-open local Electron `WebContents` (browser tab) a given worktree's pane currently targets. It is never used to select a remote host/connectionId — there is no such concept anywhere in this file.
- "Agent" in `AgentBrowserBridge` does not refer to the Dev Server Agent (`agent/`) package at all — naming coincidence, not an architectural link. `agent/` genuinely has zero browser-driving capability, and (per this finding) was never the intended place for one — the real feature's execution boundary is the desktop Electron process, matching `backend-agent-execution-boundary.md:161`'s 🏠 classification **correctly**, not the mismatch SOL-006 suspected.

**Practical implication:** TASK-031–035's existing backend-go plumbing (`channels_browser.go`, relaying `browser.*` through `infra-fleet-service`'s `Relay` RPC to a remote Dev Server Agent) is not just "inert until `agent/` catches up" — it is built on an **architecturally incompatible model**. Even if `agent/` grew a full CDP driver tomorrow, a remote SSH-connected dev server has no Electron process/desktop `WebContents` for it to control; the old feature fundamentally requires desktop co-location. A real "browser panes over backend-go/SSH-relay" feature would need a genuinely new design (e.g. driving a headless browser process ON the remote dev server host itself via `agent/`, screencasting frames back — closer to what SOL-006's own sketch already guessed at, coincidentally, but for the wrong reason: not because the old feature needs porting, but because the old feature literally cannot be ported as-is).

**Item 1 (product decision) — still not mine to make, but now much better-scoped:** the real choice is no longer "should `agent/` grow CDP driving" (that was always going to be a big, separate feature even if yes) — it's simpler: **(a)** leave `browser.*` permanently backend-go-unsupported for SSH-relay/server deployments, consistent with `02-microservices-decomposition.md`'s other explicitly-desktop-only exclusions (same bucket as `emulator.*`'s ADB/simctl driving), and remove/reconsider TASK-031-035's now-known-incompatible relay plumbing; or **(b)** commission a genuinely new remote-headless-browser design from scratch, not a port. I have not implemented either — that's a real product call, not something to guess into code.

---

## Context

TASK-031 through TASK-035 make all 15 `browser.*` methods relay/resolve
correctly through `infra-fleet-service`, but SOL-006 is explicit that this
is the **larger** of the two agent-side gaps in this task batch (contrast
TASK-023's `accounts.*` gap, flagged as small):

> "driving a browser pane means the Dev Server Agent must be able to
> **launch and control a full browser process** — navigate, inject input,
> evaluate JS, manage tabs, stream frames back (`browser.screencast`), and
> read/import OS-level browser profile data... Nothing in `specs/agent/api/`
> documents the Dev Server Agent having this capability today — this is
> not a small companion change."

SOL-006 also flags an open, **unresolved** question this solution
deliberately does not answer on its own:

> "is backend-go expected to support live remote browser panes at all, and
> if so, is `agent/` growing a CDP/Playwright driver its own team's roadmap
> already plans, or does this wait? That product decision is not
> backend-go's to make unilaterally, and this proposal does not attempt to
> make it — it only establishes where the plumbing goes *if* the answer is
> yes."

There is also an unresolved **dispatch-model disambiguation** SOL-006
could not fully close from specs alone: `backend-agent-execution-boundary.md:161`
classifies `browser.*` as 🏠 backend-local, but the worktree-scoping
evidence in `browser-pane-remote.tsx`'s actual call sites contradicts that.
SOL-006 concludes this doc's `browser.*` entry likely describes the
separate `window.api.browser` Electron surface instead, but flags this as
"pending someone tracing the actual old TS backend's `browser.*` RPC
handler source to confirm definitively" — not fully dischargeable from
specs alone.

This task exists to track both open items explicitly, separate from
`accounts.*`'s smaller companion-work tracking (TASK-023), since SOL-006
frames this as needing product sign-off before agent-side work even
starts, not just an engineering task to schedule.

---

## What to do

Not a code change. File (or link) three tracking items:

1. **Product decision (blocking, do this first):** "Decide whether
   backend-go should support live remote browser panes (`browser.*`), and
   if so, whether `agent/` grows a CDP/Playwright driver on its own
   roadmap or waits." Link SOL-006's "The honest limit of this proposal"
   section and this TASK file. TASK-031–TASK-035's shipped-but-inert
   plumbing is safe to merge regardless of this decision's outcome (it's
   dead code path until the agent side exists), but the agent-side work
   itself should not start without this sign-off.
2. **Dispatch-model confirmation:** "Confirm whether `browser.*` in
   `backend-agent-execution-boundary.md:161`'s 🏠 classification actually
   describes `window.api.browser` (Electron-local) rather than this
   worktree-scoped RPC namespace — trace the old TS backend's `browser.*`
   RPC handler source to confirm." Link SOL-006's "Resolving the
   dispatch-model uncertainty" section.
3. **Agent-side capability (only after item 1 is resolved "yes"):**
   "Implement browser-process launch/control on the Dev Server Agent:
   navigate, input injection, JS eval, tab management, frame streaming
   (`browser.screencast` — needs its own server-streaming RPC design, not
   covered by TASK-031–035's unary `Relay` plumbing), and OS-level browser
   profile detection/import." Link TASK-034 (Groups A/B) and TASK-033
   (Group C's 3 relayed profile ops) as the backend-go plumbing this
   capability activates.

---

## Verify

N/A — no code produced by this task. "Done" means all three tracking items
exist and are linked from this file, so TASK-031–TASK-035's shipped-but-
inert state — and the still-open product/dispatch-model questions above
it — are discoverable rather than silently assumed resolved.
