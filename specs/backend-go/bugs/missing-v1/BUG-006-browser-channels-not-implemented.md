# BUG-006: `browser.*` channels not implemented in backend-go

**Service:** none found in backend-go's 17 services — probable architecture gap, not just a missing wscompat wrapper
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` (absent), `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` (falls through to `notImplementedHandler`)
**Severity:** Medium — a large namespace (15 methods) backing the in-app "browser pane" preview feature, but it's a secondary/preview surface, not core daily-usage functionality like git/files/terminal.
**Symptom:** All 15 `browser.*` calls from `browser-pane-remote.tsx`/`browser.ts`/`workspace-port-actions.ts` time out with `channel "browser.X" is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table`.
**Status:** 🚧 Partially resolved — agent + backend-go layers DONE end-to-end (TASK-031–036); frontend dispatch to the new relay path remains an open, separately-scoped gap — see TASK-036's "Status by layer" section.

---

## Description

`browser.*` is "Embedded browser pane control (navigation, input, cookies,
tabs)" (`specs/frontend/api/rpc-catalog.md:38`). None of the 15 methods the
frontend calls are registered in `wscompat.Registry`. Confirmed via:

```
$ grep -n '"browser\.' backend-go/services/api-gateway/internal/adapter/wscompat/channels.go
(no matches)
```

**Important disambiguation** — do not conflate this with `window.api.browser`,
a separate desktop-only Electron `<webview>`/CDP control surface documented
in `specs/frontend/api/ipc-surface.md` as explicitly NOT migrated to RPC.
This `browser.*` RPC namespace, called via `callRuntimeRpc`, is a distinct
"browser-pane-remote" concept.

A repo-wide search for anything browser-process-control related in
backend-go returns nothing:

```
$ grep -rli "browser.*profile\|browser.*tab\|browser.*viewport\|browserpane\|browser-pane\|browser_pane" backend-go --include="*.go" --include="*.proto" --include="*.md"
(no matches)
```

`registry.go`'s `NewDefaultServiceRegistry()`
(`backend-go/services/api-gateway/internal/domain/registry.go:82-101`) has no
routing rule for this either. No service — `infra-fleet-service` included —
has proto or usecase code for driving a remote browser process. This is a
**service-doesn't-have-this-capability gap**.

### ⚠️ Dispatch-model caveat — verify before implementing

`specs/frontend/api/backend-agent-execution-boundary.md:161` documents a
`browser.*`/`browser.screencast` namespace as 🏠 backend-local, "Drives
Electron `webContents`/CDP **inside the Orca backend process itself** — a
browser tab lives on the backend host, not any dev server." That description
does **not** match what this RPC namespace's real call sites do: every
`callRuntimeRpc` call in `browser-pane-remote.tsx` passes a `worktree`
selector as part of its params (e.g.
`{ worktree: runtimeWorktree, page: pageId, width, height, ... }` for
`browser.viewport`, `browser-pane-remote.tsx:352-359`), and the component
resolves its runtime environment via `getRuntimeEnvironmentIdForWorktree`/
`toRuntimeWorktreeSelector(worktreeId)` (`browser-pane-remote.tsx:14,41,201`)
— i.e. this pane is scoped to whichever host owns the worktree, not
unconditionally the backend's own process. The execution-boundary doc's
`browser.*` entry may be describing a different, desktop-only concept than
this remote-pane one (consistent with the `window.api.browser` vs.
RPC-`browser.*` distinction above). **Do not assume the 🏠 backend-local
classification below is correct without re-verifying against the real old
TS backend's `browser.*` RPC handler source** — it is included here only
because it's the only documented dispatch note for a namespace named
`browser.*`, not because it's been confirmed to describe this specific
15-method surface.

---

## Missing channels

| Method | Frontend call site | Notes |
|---|---|---|
| `browser.eval` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:365-367,1436-1438` | No owning service. |
| `browser.keypress` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:1493-1495` | No owning service. |
| `browser.mouseDown` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:1343-1345` | No owning service. |
| `browser.mouseMove` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:1337-1339,1384-1386,1539-1541` | No owning service. |
| `browser.mouseUp` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:1390-1392` | No owning service. |
| `browser.mouseWheel` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:1545-1547` | No owning service. |
| `browser.profileClearDefaultCookies` | `frontend/src/renderer/src/store/slices/browser.ts:2054-2056` | No owning service. |
| `browser.profileCreate` | `frontend/src/renderer/src/store/slices/browser.ts:1755-1757` | No owning service. |
| `browser.profileDelete` | `frontend/src/renderer/src/store/slices/browser.ts:1791-1793` | No owning service. |
| `browser.profileDetectBrowsers` | `frontend/src/renderer/src/store/slices/browser.ts:1917-1919` | No owning service. |
| `browser.profileImportFromBrowser` | `frontend/src/renderer/src/store/slices/browser.ts:1957-1959` | No owning service. |
| `browser.profileList` | `frontend/src/renderer/src/store/slices/browser.ts:1732-1734` | No owning service. |
| `browser.tabClose` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:589-591,703-705` | No owning service. |
| `browser.tabCreate` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:696-698`, `frontend/src/renderer/src/lib/workspace-port-actions.ts` | No owning service. |
| `browser.viewport` | `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:349-361` | No owning service. Params include `worktree`, confirming worktree/host scoping (see dispatch-model caveat above). |

None of these are registered anywhere in `channels.go`, confirmed by the
grep above — this is a full-namespace gap, and the namespace has no owning
service at any layer (proto, usecase, or REST route).

---

## Dispatch model

The execution-boundary doc's only entry naming `browser.*`
(`specs/frontend/api/backend-agent-execution-boundary.md:161`) says:

> `browser.*`, `browser.screencast` | 🏠 | Drives Electron
> `webContents`/CDP **inside the Orca backend process itself** — a browser
> tab lives on the backend host, not any dev server.

As detailed in the caveat above, this description conflicts with the
observed frontend call sites, which pass a `worktree` selector and resolve
their runtime target per-worktree — suggesting the real dispatch is
worktree/host-scoped (possibly relayed to the Dev Server Agent for a
remote-host worktree, similar to `git.*`/`files.*`'s 🔀 dynamic pattern), not
unconditionally backend-local. **Before implementing this namespace in
backend-go, trace the actual old TS backend RPC handler for `browser.*`**
(not just this execution-boundary doc entry, which may describe the
unrelated `window.api.browser` Electron surface) to determine the correct
dispatch model. If it does turn out to be worktree-scoped and relayable,
the natural home is either `infra-fleet-service` (already owns SSH/dev-server
fleet concerns) or a new service, driving a browser process via the Dev
Server Agent relay — but this should not be assumed without verification.

---

## References

- `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go` — confirmed no `browser.*` registrations
- `backend-go/services/api-gateway/internal/adapter/wscompat/registry.go` — `notImplementedHandler`
- `backend-go/services/api-gateway/internal/domain/registry.go:82-101` — `NewDefaultServiceRegistry()`, no browser-pane routing rule
- `frontend/src/renderer/src/components/browser-pane/browser-pane-remote.tsx:14,41,201,349-361,365-367,589-591,696-705,1337-1392,1436-1438,1493-1495,1539-1547` — call sites, `worktree` param, `toRuntimeWorktreeSelector`/`getRuntimeEnvironmentIdForWorktree` usage
- `frontend/src/renderer/src/store/slices/browser.ts:1732-2056` — `profile*` call sites
- `frontend/src/renderer/src/lib/workspace-port-actions.ts` — `browser.tabCreate` call site
- `specs/frontend/api/backend-agent-execution-boundary.md:161` — the only `browser.*` dispatch-model entry (flagged above as possibly describing a different, desktop-only concept)
- `specs/frontend/api/ipc-surface.md` — documents `window.api.browser` as the separate, not-migrated-to-RPC Electron surface this namespace must not be conflated with
- `specs/frontend/api/rpc-catalog.md:38,109-127` — `browser.*` namespace summary and catalog entries
- `specs/backend-go/bugs/missing-v1/BUG-009-files-channels-not-implemented.md` — sibling bug report this follows the format of
