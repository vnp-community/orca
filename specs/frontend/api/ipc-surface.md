# Electron IPC Surface (`window.api.*`)

**Updated 2026-08-15** — this doc originally catalogued `window.api.*` as a
same-process channel "listed for completeness, not exhaustively catalogued."
As of this update it also documents a deliberate scope narrowing: on the web
build, the parts of this surface that were pure indirection over an RPC call
have been migrated to call the RPC layer directly, and `window.api.*` is now
understood as covering exactly two things:

1. **The RPC transport itself** (`window.api.runtime`/`window.api.runtimeEnvironments`)
   — load-bearing, not indirection. `callRuntimeRpc()`
   (`renderer/src/runtime/runtime-rpc-client.ts`), the shared helper
   `rpc-catalog.md` documents, is *implemented on top of* these two IPC
   namespaces. They are not migration candidates.
2. **Genuine desktop/OS-only work with no backend to relay to**: window
   control, native file/directory pickers, OS keychain, tray, embedded
   `<webview>` browser-pane control, OS notifications, local PTY spawn,
   local crash-dump/log collection, Windows shell detection (WSL/pwsh/git-bash),
   LAN device pairing, and similar. These correctly stay on `window.api` for
   desktop; the web build either has no equivalent (native dialogs) or
   reaches the same outcome through a different, already-separate RPC surface
   (e.g. terminals: web uses `terminal.*` RPC directly, not `window.api.pty`).

## What changed

Before this pass, several namespaces (`gh`, `gl`, `repos`, `worktrees`,
`hostedReview`, `linear`, parts of `fs`/`ssh`, and `credentials`) had a web
implementation in `renderer/src/web/web-preload-api.ts` that mostly just
re-issued the identical RPC call `rpc-catalog.md` documents — two names for
one operation, which is exactly the kind of drift that caused BUG-FE-RPC-004/005
this session. Investigation found the situation was better than the raw
`window.api.*` occurrence count suggested: **most of these namespaces were
already migrated ad-hoc, per-call-site**, to a hybrid pattern established by
`renderer/src/runtime/runtime-git-client.ts` (one of the earliest examples):

```ts
const target = getActiveRuntimeTarget(settings)
if (target.kind !== 'environment') {
  return window.api.git.status({ worktreePath })   // desktop-local path — unchanged
}
return callRuntimeRpc(target, 'git.status', { worktree: toRuntimeWorktreeSelector(...) })
```

The remaining real gaps were migrated the same way. New wrapper files:
`runtime-github-client.ts`, `runtime-gitlab-client.ts`, `runtime-ssh-client.ts`,
`runtime-credentials-client.ts` (joining the existing `runtime-git-client.ts`,
`runtime-repo-client.ts`, `runtime-linear-client.ts`). Net: roughly 40 call
sites across ~20 files converted from `window.api.<ns>.<method>()` to a direct
RPC call via one of these wrappers — much smaller than the full `window.api.*`
surface, because two of its largest namespaces were explicitly excluded (see
below) and most of the rest turned out to already be correct.

## Explicitly NOT migrated, and why

- **`ui` (98 files, largest namespace in the surface)** — not a passthrough.
  The web implementation maintains a local-storage-backed UI-state cache with
  optimistic updates and offline fallback (`mergeWebUIState` and friends)
  layered around `ui.get`/`ui.set`. A mechanical replace would silently drop
  that offline/local-persistence behavior. Needs its own dedicated design
  pass (e.g. extracting the merge logic into a shared hook), not a bulk
  find-replace — left alone.
- **`settings`** — same shape as `ui`: local merge + RPC sync, plus a
  synchronous `getSync()` read path used for pre-hydration that has no RPC
  equivalent (RPC is inherently async). Left alone for the same reason.
- **`pty` (30 files)** — genuinely desktop-only; the web terminal UI already
  uses the separate `terminal.*` RPC namespace directly, not `window.api.pty`.
- **`shell`, `browser`, `notifications`, `mobile`, `cli`, `agentStatus`,
  `crashReports`, `diagnostics`, `wsl`/`pwsh`/`gitBash`** — real
  Electron/OS-only work, no RPC equivalent makes sense. Correctly stay
  desktop-only; web already no-ops or hides the relevant UI.
- **`session`** — pure `localStorage` on web vs. desktop's disk-backed
  persistence: different backing stores by design, not a naming duplication.
- **The Bucket-2 methods inside otherwise-migrated namespaces** — e.g.
  `repos.pickFolder/pickFolders/pickDirectory/removeForHost/cloneRemote/createRemote`
  are native-dialog/desktop-only and stayed on `window.api.repos.*` even
  though most of that namespace migrated.

## A latent bug found and partially fixed along the way

`web-preload-api.ts`'s `installWebPreloadApi()` wraps everything in a
`withFallback()` proxy: any `window.api.<namespace>.<method>()` call for a
namespace/method **not explicitly implemented** silently resolves via a
name-prefix heuristic (`on*` → no-op unsubscribe, `is*/has*` → `false`,
`list*/detect*` → `[]`, else → `Promise.resolve(undefined)`) — **it never
throws**. At least 21 namespaces have no explicit web implementation at all
and rely entirely on this fallback. A handful of confirmed call sites
(`pet`, `notebook`, `ephemeralVm`) didn't check `isWebClientLocation()` first
and would silently resolve `undefined` on web instead of showing an honest
"unavailable" state. Three of these (`PetStatusSegment.tsx`, `IpynbViewer.tsx`,
`EphemeralVmsPane.tsx`) got an explicit `isWebClientLocation()` guard as part
of this pass. **Not all 21 orphan namespaces were individually audited** —
a full sweep for the same unguarded-fallback pattern is real follow-up work,
not done here.

## `web-preload-api.ts` was intentionally NOT deleted or trimmed

Every method that could theoretically be trimmed after the migration is still
referenced by some other call site's desktop-local branch (e.g. `gh.repoSlug`
is still used locally by `repository-icon-github.ts`). Proving that's safe to
remove needs a dedicated method-by-method audit (ruling out `target.kind ===
'local'` while running the web build for each one) — not something to do as
part of a mixed-scope migration pass. The file still needs to keep the
transport (`createRuntimeApi`/`createRuntimeEnvironmentsApi`), the genuine
Bucket-2 stub namespaces, and the `ui`/`settings` merge logic regardless.

## `fs`/`ssh` triage detail

Both namespaces mix real RPC-passthrough methods with genuine desktop-only
stubs in the same file. Per-method breakdown:

- **`fs`**: 21 files referenced `window.api.fs.*`. 2 genuine gaps
  (`McpConfigSection.tsx`, `mcp-config-inspection.ts`) migrated; the other 19
  were already correctly gated to local-only paths or are explicit
  desktop-only stubs (log-tail, file-download-chunk).
- **`ssh`**: 24 files referenced `window.api.ssh.*`. Of the 4 real-RPC
  methods (`listTargets`, `listRemovedTargetLabels`, `connect`, `getState`),
  9 files had a genuine gap and were migrated; 4 files are legitimately
  desktop-only (gated by `use-sidebar-host-scope-options.ts`, which excludes
  SSH-kind host options in web mode); the port-forward/fleet/target-CRUD
  methods are explicit "unavailable in web client" stubs, untouched.

## If you need the full IPC list

Read `frontend/src/preload/api-types.ts` directly for the type contract, and
`frontend/src/main/ipc/` for the desktop implementations — that's the
authoritative source. `frontend/src/renderer/src/runtime/runtime-*-client.ts`
is the authoritative source for which namespaces now have a hybrid
desktop/web wrapper.
