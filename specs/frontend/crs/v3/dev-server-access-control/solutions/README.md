# Frontend Solutions — Dev Server Access Control

**CRs:** [docs/crs/v2/dev-server/CR-DS-006..008](../../../../../docs/crs/v2/dev-server/README.md)
**Backend counterpart:** [specs/backend-go/crs/v0/dev-server-access-control/](../../../../backend-go/crs/v0/dev-server-access-control/solutions/README.md)

## Solutions

| Solution | CR | Status |
|---|---|---|
| [FE-SOL-001](./FE-SOL-001-admin-approval-console.md) | CR-DS-006 | ✅ Completed |
| [FE-SOL-002](./FE-SOL-002-first-login-department-gate.md) | CR-DS-008 §2.1 | ✅ Completed |
| [FE-SOL-003](./FE-SOL-003-skip-onboarding-role-branch-and-access-request.md) | CR-DS-008 §2.2, §2.3 | ✅ Completed |

All three solutions in this topic are implemented and wired against the
real backend-go RPCs (BE-SOL-001..004, all shipped and deployed to
b15.openledger.vn at api-gateway 0.4.0). See each solution doc for its
"Resolved during implementation" section — the open product decisions
these docs originally recorded as blocking have all been resolved with a
documented, reversible default.

## What shipped in this pass

- **Prerequisite fix**: `profile.getUserProfile`/`profile.listDepts`/`profile.updateUser`
  (`channels_tenant_project.go`) were leaking snake_case JSON keys to the
  frontend (raw proto messages serialized via plain `encoding/json`, whose
  struct tags are snake_case, not the camelCase these channels' callers
  need) — the exact same bug class fixed for the new access-control
  channels earlier this session, now also fixed here since FE-SOL-002
  depends on `departmentId` actually arriving populated. Regression-tested
  in `channels_tenant_project_test.go`.
- New shared frontend types: `frontend/src/shared/tenant-user-profile-types.ts`
  (`TenantUserProfile`, `TenantDepartment` — deliberately separate from the
  unrelated, differently-shaped `Department` type in
  `renderer/src/types/profile-types.ts`, which backs an unconnected
  org-hierarchy admin panel, not this feature).
- New preload API surface: `window.api.devServer.{approve,reject,assignGroup,
  listForUser,requestAccess,listPendingAccessRequests,resolveAccessRequest}`,
  `window.api.devServerGroup.{create,list,grant,revoke,listGrants}`,
  `window.api.tenantProfile.{getUserProfile,listDepartments,setUserDepartment}` —
  all in `frontend/src/preload/api-types.ts` + `web-preload-api.ts`, following
  the existing `callRuntimeResult<T>('channel.name', args)` pattern.
- `DevServer`'s shared type gained optional `approvalStatus`/`groupId` fields.
- Verified: `tsc --noEmit` shows zero new errors (diffed against a
  pre-change baseline of 150 pre-existing unrelated errors — this
  codebase's typecheck baseline is not currently clean, out of scope to
  fix here), `vite build` succeeds, and the two directly-relevant existing
  test suites (`useSettingsNavigationMetadata.*.test.ts(x)`,
  `Settings.load-performance.test.ts`) pass. `OnboardingFlow.test.tsx` has
  14 pre-existing failures (confirmed via `git stash` bisection — identical
  failure count before and after this pass' edits, caused by an unrelated
  `activeDevServerId`/`WindowsTerminalStep` prop mismatch from earlier in
  this session) — not introduced or worsened by this work.
- `gitnexus impact` flagged `buildSettingsNavigationMetadata` CRITICAL
  (3249 impacted) — verified false positive: `maxDepth:1` shows the only
  real caller is `useSettingsNavigationMetadata` itself; the rest is
  monorepo-wide name collisions with unrelated Go `run()` entrypoints and
  legacy TS backend code, the same false-positive pattern already
  confirmed twice earlier this session for Go symbols.

## Critical fix found via live user report (post-deploy, same day)

The user logged in as the bootstrap admin (`admin@b15.openledger.vn`,
confirmed `role='admin'` in `auth.users` via direct DB check) and reported
not seeing the admin-gated UI. Root cause: `useAppStore`'s `authSlice`
(`currentUser`/`authStatus`) is **separate, dead scaffolding** — the real
web login flow (`main-web-bootstrap.tsx`'s `WebRootBoundary`/`WebRoot`)
resolves the session via its own local React state (`sessionUser`, from
`fetchCurrentUser()`) and never wires it into the Zustand store. Every
`useAppStore(s => s.currentUser)` read below `<App/>` — including all of
this pass's new `isAdmin` checks (`DepartmentGate`, `Settings.tsx`,
`useSettingsNavigationMetadata`, `OnboardingFlow`'s skip branch) — was
therefore always reading `null`/`undefined`, i.e. **admin-gating never
worked for anyone, including real admins**, from the first deploy of this
feature. Fixed in `main-web-bootstrap.tsx`: `WebRoot` now mirrors
`sessionUser` into `useAppStore`'s `setCurrentUser`/`setAuthStatus` via a
`useEffect` once resolved. `DepartmentGate` was also hardened to gate its
profile check on `authStatus === 'authenticated'` (not just `isAdmin`) so
it can't fire before the session resolves. Redeployed as version 0.4.2.
Also confirmed separately (DB check): the bootstrap admin account has zero
rows in `tenant.user_profiles` — expected given nothing provisions one for
it, and harmless once the admin bypass above actually engages, but worth a
follow-up if department-scoped tenant data is ever needed for the bootstrap
admin specifically.

## Third live bug found: connected agents never appeared in dev_servers at all

User reported 3 real dev-server agents connected (verified via
`deploy/agent/scripts/deploy-agents.sh --status` — all 3 systemd services
active, agent logs showing successful `agent.handshake`/"Connection
established and authenticated") but the Admin Console's Approvals tab
showed nothing. DB check confirmed: `SELECT * FROM infra.dev_servers`
returned **zero rows**.

Root cause (backend-go, pre-existing — not introduced by this session's
work, but this is the first feature to depend on it): the direct-websocket
agent-connection pathway (`POST /api/agent-token` mints a token →
`agentwsserver.Registry` tracks it in-memory → agent dials in → handshake →
`devserveragent.Client.AttachInboundSession`) **never called
`RegisterDevServer`** — it only ever tracked live sessions in an in-memory
map keyed by the caller-supplied `devServerId` string (e.g. `"dev-01"`),
completely disconnected from the SQL-backed `infra.dev_servers` table
`ListDevServers`/`ApproveDevServer`/the Admin Console all read from. This
gap predates CR-DS-006 entirely (ported as-is from the legacy TS
`agent-token-routes.ts`) — CR-DS-006/007/008 built the entire
approval/grouping model on the assumption a `dev_servers` row exists for
every connected agent, which was never true for this connection mode.

Fixed with a find-or-create pattern mirroring the existing
`FindBySshTarget`/`EstablishConnection` precedent in this exact codebase:

- `DevServerRepository.FindByHostAndMode(tenantID, host, mode)` (new
  interface method + Postgres implementation) — `host` doubles as the
  direct-websocket agent's external `devServerId` string, a column
  otherwise unused in that mode.
- `ResolveDirectWebSocketDevServer` usecase: looks up an existing row by
  (tenant, devServerId, direct-websocket); if none exists, registers a new
  one (defaulting to `pending_approval`, same as every other registration
  path). Reused across reconnects so an admin's approval/group survives the
  agent's own restart cycle.
- `TokenIssuer` (the `/api/agent-token` HTTP handler) now calls this at
  mint time and registers the **resolved row's real UUID** — not the raw
  `devServerId` string — as the `agentwsserver.Registry` slot key, so the
  later `AttachInboundSession(id, ...)` session key matches
  `domain.DevServer.ID` exactly.
- `TenantID` can't come from request context here — this endpoint
  authenticates via a single shared `ORCA_AGENT_API_SECRET`, not a
  per-user session — so `agentwsserver.Config` gained
  `DefaultTenantID` (env `ORCA_AGENT_DEFAULT_TENANT_ID`, falling back to
  the bootstrap tenant sentinel `00000000-0000-0000-0000-000000000001`,
  confirmed live via `auth.users` — correct for today's
  single-tenant-per-deployment reality; a true multi-tenant agent-token
  flow needs this per-token, not per-deployment, as a follow-up).
- A resolve failure fails open (logs, keeps the old raw-string behavior)
  rather than breaking token issuance/agent connectivity — DB visibility
  degrading gracefully beats agents being unable to connect at all.

5 new/updated Go test files (`resolve_direct_websocket_dev_server_test.go`,
`token_endpoint_test.go`'s new resolver-wiring test, plus 2 test fakes
extended for the new interface method). All 17 backend-go services build
clean; full `infra-fleet-service` test suite passes. Redeployed as version
0.4.4 — force-recreating containers disconnects all 3 live agents, whose
systemd `Restart=always` re-mints a token and reconnects automatically
through the fixed path within ~15s.

## Fourth/fifth live bugs (see backend-go solutions/README.md for the
## Role-propagation double-AttachIdentity fix, version 0.4.5) and:

## Sixth live bug: devServer.approve/reject/assignGroup sent the wrong arg key

With admin-gating fixed (0.4.5), Approve started failing with
`INFRA_APPROVE_DEV_SERVER_FAILED` for every dev server, unconditionally.
Root cause: `web-preload-api.ts`'s `approve`/`reject`/`assignGroup` sent
`{ id }`, but `channels_dev_server_access_control.go`'s handlers decode the
key as `devServerId` — the mismatch left the backend's `DevServerID` empty,
so `UPDATE ... WHERE id = ''` matched no row and failed. Fixed by sending
`{ devServerId: id }` (matching every other channel in that file, which
all use fully-qualified key names). 3 new regression tests asserting the
exact params object sent over the wire. Redeployed as version 0.4.6.

## Seventh: onboarding's dev-server step didn't fit the real connection model

User request: direct-websocket (dev server → Orca) should be the default
connection type in onboarding's "Connect a dev server" step, and for that
mode the user should pick from dev servers they're allowed to see
(department-granted, or all of them if admin) instead of typing a
WebSocket URL — direct-websocket means the agent dials Orca, so there's no
URL for the user to type in the first place. Reworked `DevServerStep.tsx`:
default `connectionType` is now `direct-websocket`; that branch fetches
`devServer.listForUser()` (or `devServer.list()` for admins, client-filtered
to approved + direct-websocket) instead of showing a host `Input`, and
selecting one calls `updateSettings({ activeDevServerId })` instead of the
relay-ssh/relay-websocket test-connection/add-server flow (nothing to test
— the dev server is already connected via its agent). Deployed as 0.4.7.

## Eighth: built the org/user administration UI the department picker needed

Following the empty-picker question above, built the missing pieces (see
backend-go solutions/README.md for the channel-level detail): a new
`AdminOrgConsole.tsx` (Departments tab: list + create; Users tab: list,
change role, deactivate/reactivate, assign department to any user) wired
into Settings as a new admin-gated section (`admin-org`, mirroring
`admin-dev-servers`'s gating pattern) and nav entry. New shared types
`admin-user-types.ts` (`AdminUser`/`AdminUserRole`), extended
`tenant-user-profile-types.ts` with `TenantCompany`, extended
`tenantProfile`/added `admin` preload namespaces.

## Ninth: user asked directly where to create/edit Company — added a Company tab

Follow-up question: where to create/edit Company info, and confirmed edit
rights should stay admin-only (already the case — every channel here checks
`id.Role == "admin"`). Found there was no way to even *read* the current
company's name (`GetCompany` RPC didn't exist — only Create/Update did), so
added it: new `tenant.proto` RPC + `GetCompany` usecase (wraps the
already-existing `CompanyRepository.Get`) + gRPC handler + `profile.getCompany`
wscompat channel (defaults to the caller's own company, matching
`profile.listDepts`'s established default-to-tenant convention) + fixed
`profile.updateCompany`'s same-class snake_case bug while touching this
file. New `Company` tab in `AdminOrgConsole.tsx` — fetch + rename. "Create
company" is still deliberately not exposed (see that file's doc comment:
`CreateCompany` mints a disconnected tenant_id, not useful from an existing
admin's console).

## Tenth: onboarding's dev-server step silently refused to advance

User report: picked a dev server, clicked "Use this dev server", nothing
happened — no error, no progress. Root cause, **pre-existing, not
introduced by this pass**: `usePersistCurrentStep` (the switch `flow.next()`
awaits before advancing any onboarding step) never had a case for
`'dev_server'` at all — it silently fell through to `return { ok: false }`,
and `next()` treats `!ok` as "don't advance," not an error. This means
`DevServerStep`'s "Continue"/"Use this dev server" buttons — both call
`flow.next()` directly, like every other step — **never worked**, even
before today's onboarding rework; it just went unnoticed because reaching
a `connectedServers.length > 0` state at this exact step was rare until
this session's direct-websocket picker made it common. Fixed by adding the
missing `dev_server` case (mirrors `windows_terminal`/`integrations`'s
"just mark the step complete" pattern — the actual selection already
persists via `DevServerStep`'s own `updateSettings({ activeDevServerId })`
call before `onNext()` runs). Also hardened `DevServerStep`'s own handler
with a `.catch()` for defense in depth.

## Eleventh: real multi-company support (user explicitly requested it)

After being shown the tradeoff (no company-switcher exists — a new company
is only reachable by creating its first admin and logging in separately),
user confirmed they want real multi-company support. See backend-go
solutions/README.md for the `CreateUser` password-flow fix this required.
New `NewCompanySection` in `CompanyTab` (name input → creates an isolated
company, then inline-prompts for its first admin via a new shared
`CreateUserForm`); `UsersTab` also gained `CreateUserForm` for the common
"add a teammate to my own company" case. `CreateUserForm` shows the
one-time generated password in a persistent (not auto-dismissing) box when
the admin doesn't supply one — the only chance to ever see it. New
`admin.createUser` preload method. Also bundled a small deploy-tooling fix:
`sync-to-server.sh`'s image-pull step is now best-effort (`|| true` with a
warning) — hit a transient Docker Hub TLS-handshake-timeout twice in a row
this session, which aborted an otherwise-ready deploy even though every
image is a pinned tag already cached on the server.

## Second live bug found immediately after: wrapped-array unwrap missing

With the role fix live, the user reached the Admin Console's Groups tab and
hit `TypeError: t.map is not a function`. Root cause:
`channels_dev_server_access_control.go`'s four list-shaped channels each
wrap their array in a named object key — `devServerGroup.list` →
`{groups: [...]}`, `.listGrants` → `{grants: [...]}`, `devServer.listForUser`
→ `{devServers: [...]}`, `.listPendingAccessRequests` → `{requests: [...]}`
— matching this file's own established `TestDevServerGroupListChannel_ReturnsEmptyArrayNotNull`
test expectations. `web-preload-api.ts`'s implementations of these four
methods were never updated to unwrap that key (every other method in these
two namespaces returns a bare single-object already, so the inconsistency
was easy to miss) — callers received the wrapper object itself, and
`.map()` on it crashed. Fixed by unwrapping each key, same pattern already
established for `listSshTargets`/`addSshTarget` elsewhere in the same file.
Added 4 regression tests (`web dev server access-control preload API` in
`web-preload-api.test.ts`) asserting each method returns a bare array from
its wrapped RPC response. Redeployed as version 0.4.3. The `PreloadApi`
contract in `api-types.ts` (`Promise<X[]>`) was already correct — only the
implementation needed the fix.

## Twelfth: "No agents detected on your PATH" for every web user

User asked why the onboarding "Pick your default agent" step always showed
"No agents detected", and for a fix. Traced the actual detection call
(`use-onboarding-flow.ts`'s mount-time `refreshDetectedAgents()`) down to
`preflight.detectAgents` in `web-preload-api.ts`, which is gated on
`requireActiveEnvironmentOrNull()` — a check for a paired Electron desktop
"runtime environment" pairing, an entirely different, older concept from
this CR's dev-server-agent connection (`settings.activeDevServerId`). A
plain browser session never has one, so this always resolved to `[]`
regardless of which dev server the user had picked in `DevServerStep`.

The correct mechanism — `window.api.onboarding.detectAgents({devServerId})`,
the `detectRuntimeOnboardingAgents`/`useRemoteAgentDetection` hook chain —
already existed in the type/hook surface (evidently scaffolded for this
CR's [CR-OB-003] tags already visible in `AgentStep.tsx`) but was wired to
nothing: no backend channel answered it, and no onboarding component ever
called the hook. `AgentStep` only ever received `activeDevServerId` as a
display-only prop.

Fixed: `use-onboarding-flow.ts` now calls `useRemoteAgentDetection(settings
?.activeDevServerId ?? null)` and, whenever a dev server is active, sources
`detectedAgentIds`/`isDetectingAgents` (and the mount-time auto-select
effect) from its result instead of the local-PATH store slice — a null
`activeDevServerId` (Electron desktop) leaves the original local-PATH path
completely untouched. The backend half (`onboarding.detectAgents` wscompat
channel, relaying to the agent's own real `preflight.detectAgents` RPC) is
documented in backend-go's solutions/README.md, "Eighth" section. New
`frontend/src/shared/agent-detection-commands.ts` builds the probe catalog
client-side from `TUI_AGENT_CONFIG`, mirroring
`desktop/src/shared/agent-detection-commands.ts` exactly — this frontend
does not duplicate the catalog anywhere else, and the backend channel does
not need its own copy either (passthrough).

2 new preload tests (`web-preload-api.test.ts`: relays devServerId +
non-empty commands catalog; degrades to an empty result on relay failure
instead of throwing). `tsc --noEmit`/`vite build` both clean for the
touched files (this repo's `tsc` has substantial pre-existing, unrelated
errors elsewhere — confirmed none were introduced by this change by diffing
error counts before/after on each touched file).

## Thirteenth: a created company had nowhere to be found after refresh

Mid-turn, user reported creating a new company + its first admin, then not
finding it anywhere after a page refresh. `CompanyTab` only ever called
`getCompany()` with no id (always the caller's own tenant); nothing else
listed companies. `NewCompanySection`'s success card was the ONLY place a
created company was ever visible, and only for the rest of that session.

Fixed with the backend's new `profile.listCompanies` (see backend-go
solutions/README.md, "Ninth"): added a `CompanyListSection` to `CompanyTab`
showing every company (name + id), clickable to load it into the
rename/edit form via `getCompany({id})` (now accepts an optional id — was
previously always-caller-only). `NewCompanySection` takes an `onCreated`
callback so the list refreshes right after a create instead of only ever
reflecting the state at first mount.

## Fourteenth: Users tab "Assign department" looked like it silently reverted

Mid-turn, user reported: assign a department in the Users tab, click
Assign, refresh — looks gone. The assign genuinely persisted server-side
(see backend-go solutions/README.md, "Tenth") — the bug was purely
display: the department `<Select>` was seeded from local-only
`departmentChoice` state with no way to know what was actually assigned,
so it always showed the placeholder after a reload regardless of the real
persisted value. Fixed: `AdminUser` gained `departmentId` (server now joins
it in, see backend-go's "Tenth"); the `<Select>` falls back to
`user.departmentId` when there's no unsaved local pick, and
`handleDepartmentAssign` reloads the user list (clearing the stale local
pick) on success instead of leaving it in place until the next full
navigation.

## Sixteenth: folder browsing never worked for a dev-server-agent session

User reported three failures in the post-onboarding "Add a project" flow:
Browse folder, Clone from URL's parent-folder picker, and Create a new
project's "Choose parent folder...". Traced to the SAME component tree
(`AddRepoDialogStepContent.tsx` → `CloneStep`/`CreateStep`/
`AddRepoServerPathStartStep` → `CreateProjectLocationField`/
`RemoteFileBrowser`) the sidebar's own "+" Add Project button uses — the
"Add a project" title the user saw confirmed this, not the separate,
completely unused `onboarding/AddRepoStep.tsx` (dead code — zero importers
anywhere, confirmed via grep; ignored).

`RemoteFileBrowser.tsx` already had a `devServerId` prop variant (probably
scaffolded for this CR already), but almost nothing threaded a real
`devServerId` down to it:

- `AddRepoServerStartStep.tsx`: the primary "Browse host" tile already
  worked, but the SECONDARY Browse button (reached via "Or enter a host
  path manually") was disabled whenever `devServerId` was the only
  connection identifier available — its `disabled` condition simply never
  checked it.
- `AddRepoCloneStep.tsx` (`CloneStep`): had NO `devServerId` prop at all —
  `isRemoteClone`/`canBrowseRemoteDestination` only ever checked
  `runtimeEnvironmentId`/`sshTargetId`, so the parent-folder browse button
  was unconditionally disabled (or fell through to a native Electron
  folder-picker call that does nothing in a browser) for any dev-server
  session.
- `CreateProjectLocationField.tsx` / `AddRepoCreateStep.tsx` (`CreateStep`):
  same gap — `isRemoteHost` and the Browse button's disabled condition
  never checked `devServerId`, so "Choose parent folder..." could never
  open.
- `AddRepoDialogStepContent.tsx` already had `activeDevServerId` in scope
  (passed to `AddRepoServerPathStartStep` correctly) but never passed it to
  `<CloneStep>`/`<CreateStep>` at all.

Fixed by threading `devServerId` through all four files, mirroring the
existing `sshTargetId` handling exactly (a third branch alongside
`sshTargetId ? ... : runtimeEnvironmentId ? ... : devServerId ? ...`
everywhere `RemoteFileBrowser` is selected). The actual data-fetch half —
`devServer.browseDir` never existing as a backend channel at all — is
documented in backend-go's solutions/README.md, "Twelfth".

5 new/updated tests: `AddRepoCreateStep.test.tsx` (Browse button enabled
for a devServerId-only session), `AddRepoStartSteps.test.tsx` (2 new —
one static-markup, one DOM-interaction regression test clicking through to
the manual-entry Browse button, which also fixed a pre-existing `tsc` error
in that file — `AddRepoServerPathStartStep`'s test helper was missing the
now-required `devServerId` prop). `tsc --noEmit`/`vite build` clean; a
full `vitest run` across the whole suite confirmed zero new failures (the
suite has substantial pre-existing, unrelated failures from this session's
other in-progress work — diffed the failure list against files this fix
touched: zero overlap).

## Seventeenth: onboarding state persistence (see backend-go's "Thirteenth")

Purely a backend fix — `onboarding.get`/`update`/`markChecklistItem` now
persist for real (tenant-service's new per-user `onboarding_state_json`
column). No frontend changes needed; the frontend already called these
channels correctly and just needed the backend to stop discarding what it
was sent.

## Fifteenth: two more onboarding-completion errors (vapid-key + star-nag)

Same report, two separate console errors when finishing onboarding:

1. `GET /api/vapid-public-key` → `NOTIFICATION_NO_TENANT`. Purely a
   backend-go fix (soft-auth cookie fallback on the unauthenticated push
   routes) — see backend-go solutions/README.md, "Eleventh".
2. `Uncaught (in promise) RuntimeRpcCallError: channel
   "starNag.onboardingCompleted" is not yet implemented in backend-go`.
   Traced to `use-onboarding-flow-persistence.ts`'s post-completion
   `notifyRuntimeStarNagOnboardingCompleted(...)` call — genuinely
   fire-and-forget (scheduled in a `setTimeout` after `closeWith` already
   returned, `void`-called), but missing a `.catch`. For a caller whose
   `settings.activeRuntimeEnvironmentId` is set (a paired runtime
   environment), that channel isn't wired on the backend-go relay path yet
   — a real, documented gap, not a routing bug — so the promise rejects.
   Since this is a purely cosmetic "star this repo" nudge with zero
   functional effect on onboarding completion (confirmed: it fires strictly
   after `closeWith`'s `return true`), the fix is simply `.catch(() => {})`
   at the call site rather than chasing down every unimplemented
   `starNag.*` channel on the relay path.

## Sixteenth: cli.getInstallStatus error right after onboarding finishes (CRITICAL blast radius)

User reported this exact toast right after finishing onboarding: `channel
"cli.getInstallStatus" is not yet implemented in backend-go`. Root cause,
same family as the two fixes above but with a much bigger blast radius
(GitNexus flagged `getRuntimeCliInstallStatus` CRITICAL, 39 impacted
symbols — `impact()` run first per this repo's mandatory convention, and
the finding is called out here per that same convention):
`runtime-cli-client.ts`'s 6 functions
(`getRuntimeCliInstallStatus`/`installRuntimeCli`/`removeRuntimeCli`/
`getRuntimeWslCliInstallStatus`/`installRuntimeWslCli`/`removeRuntimeWslCli`)
all routed through `callRuntimeRpc({kind: 'local'}, 'cli.*')` —
i.e. `window.api.runtime.call`. On Electron desktop that's a same-machine
IPC round trip that happens to land on the exact same
`getCliInstallStatus()` (et al.) implementation `window.api.cli.*` reaches
directly, so the indirection was invisible there. On the web build,
`window.api.runtime.call` is a REAL network call to backend-go, which
rightly has no `cli.*` channels — installing a CLI binary on the user's own
machine is not backend-go's job, nor the browser's. Every `cli.*` call
always threw "not yet implemented", including `CliSection`'s status check
that fires right after onboarding closes.

`window.api.cli.*` was ALREADY correctly implemented on both platforms —
real IPC on desktop, an honest `{supported: false, state: 'unsupported',
detail: "CLI registration is managed on the Orca server, not in the web
browser."}` stub on web (`web-preload-api.ts`'s `createCliApi`) — and needs
no relay at all. Fixed by calling `window.api.cli.*` directly, dropping the
`callRuntimeRpc`/`LOCAL_TARGET` indirection entirely. Behavior-preserving
for Electron desktop (both paths reached the identical underlying
implementation there); on web, callers now get the correct "unsupported in
browser" status instead of an error.

Given the CRITICAL blast-radius flag, verification was extra-thorough: new
`runtime-cli-client.test.ts` (6 tests, one per function, each asserting
`window.api.runtime.call` is never invoked — a direct regression guard
against this exact bug reappearing); ran the two existing test suites with
the deepest fan-out per the impact report (`CliSection.test.tsx`,
`FloatingTerminalPanel.test.tsx` — 60 tests, all green); `tsc --noEmit`
clean for the touched file; full `vite build` clean.

## Eighteenth: "Starting terminal..." stuck forever on web (empty-string cwd was falsy)

User reported the exact terminal the Sixteenth fix's `cli.getInstallStatus`
fix depends on — Onboarding Checklist → Enable Orca CLI's inline command
terminal — never got past "Starting terminal..." on the web build.
Root cause in `OnboardingInlineCommandTerminal.tsx`: the render gate was a
truthy check, `cwd && tabId ? <TerminalPane .../> : <Loading/>`. `cwd` is
`string | null` (`null` = "not loaded yet"), and the web build's
`getFloatingTerminalCwd` (`web-preload-api.ts`) always resolves `''` — there
is no local filesystem to derive a path from in a browser. `''` is falsy in
JS, so once `cwd` finished loading the gate never passed, for every web
session, across every caller (CLI install, agent-skill setup, integrations
step onboarding terminals) — not specific to CLI install, just first
noticed there.

`cwd=''` flowing through as a real value is already safe and intentional —
`TerminalPane.tsx` treats it as a valid "no override" sentinel everywhere
(`fallbackCwd: cwd ?? ''`); only the gate's truthy check was wrong. Fixed by
switching to an explicit null check: `cwd !== null && tabId !== null`.

Kept the check INLINE in the JSX rather than extracting it to a called
helper — TypeScript's control-flow narrowing of `cwd`/`tabId` inside the
branch (both passed non-null to `<TerminalPane>`/`closeTab`) only works
against an inline condition, not a boolean returned from a function call;
extracting it first produced 4 new `tsc` errors, reverted. Added a separate
exported pure `isFloatingTerminalReady(cwd, tabId)` helper next to
`findTerminalTabElement`, used only by the test file, with a comment noting
both must be kept in sync manually.

4 new tests in `OnboardingInlineCommandTerminal.test.ts` covering
`isFloatingTerminalReady`, including the exact live bug (`cwd=''`,
`tabId` set → must be ready). `tsc --noEmit`/`vitest run` clean for both
touched files.

## Twentieth: the CLI-install terminal's real failure, once "Starting terminal..." stopped hanging — connectionId never reached terminal.create at all

Live-verified after deploying the Eighteenth fix (the render-gate hang):
the terminal now moves past "Starting terminal..." — but immediately shows
`rpc error: ... INFRA_TERMINAL_HOST_LOCAL_UNIMPLEMENTED: host-local
terminal sessions are not implemented`. Root cause, found by tracing the
full call chain from `OnboardingInlineCommandTerminal` down to the wire:
backend-go's `terminal.create` (`channels_terminal.go`'s
`terminalCreateArgs`) reads only `connectionId` — never `worktree` — and
treats an empty `connectionId` as "spawn a host-local PTY", which
`SpawnTerminalSession` documents as unimplemented outside desktop's local
IPC. `createRemoteRuntimePtyTransport` (`remote-runtime-pty-transport.ts`)
— the ONLY transport the web build can use, since `window.api.pty` is a
permanent web stub — computed and received a `connectionId` in its own
`opts` but never read it, and its `terminal.create` call never included
it. **This affects every dev-server-bound terminal on the web build, not
just the ephemeral onboarding one** — confirmed live via a direct `/ws`
RPC script (bypassing the UI) once real projects exist, and via a new
regression test exercising the same repo-bound-Dev-Server path an earlier
test (`"routes a Dev Server-owned repo..."`) already covered for transport
*selection* but never checked connectionId *forwarding* on.

A second, narrower gap sits on top for the ephemeral case specifically:
`OnboardingInlineCommandTerminal`'s worktreeId (`CliSkillSetupTerminal`,
`FeatureSetupInlineTerminal`, etc.) has no backing repo record at all, so
`getConnectionId(worktreeId)` — the repo-indexed lookup every other
terminal resolves its connectionId from — always returns
`null`/`undefined` for it, with no repo to fall back to.

Fixed both, additively:

- `createRemoteRuntimePtyTransport`: destructures `connectionId` from
  `opts` and includes it in the `terminal.create` params when present;
  `getConnectionId()` now returns the real value instead of a hardcoded
  `null` (other call sites already expected this, e.g. paste's remote-
  platform detection).
- `TerminalTab` (types.ts): new optional `connectionId?: string | null`
  field — an explicit dev-server binding for a tab whose worktreeId has no
  repo to resolve one from. `createTab`'s `options` gained a matching
  `connectionId` passthrough. `pty-connection.ts`'s connectionId
  resolution now prefers `tab.connectionId` over the repo-based
  `getConnectionId(worktreeId)` lookup — a no-op for every ordinary
  repo-backed tab, which never sets this field.
- `OnboardingInlineCommandTerminal`: new optional `devServerId` prop,
  passed through to `createTab`'s new `connectionId` option.
  `CliSkillSetupTerminal` (the "Enable Orca CLI" step) resolves it from
  the onboarding-selected dev server if connected, else the first
  connected one — the same fallback pattern this session's earlier
  folder-browsing/agent-detection fixes established.

5 new tests: 2 in `remote-runtime-pty-transport.test.ts` (forwards
connectionId when given one; omits it entirely when absent, preserving
host-local desktop terminals), 2 in `pty-connection.test.ts` (forwards a
repo's connectionId into the transport options on web; resolves
connectionId from the tab itself for a repo-less ephemeral terminal), and
the existing `OnboardingInlineCommandTerminal`/`CliSkillSetupTerminal`
suites re-run clean. Full regression sweep across
`terminal-pane`/`onboarding`/`feature-tips`/`store/slices` confirmed the
only failures are pre-existing ones already present without this change
(diffed via `git stash` against the same baseline — 4 unrelated failing
test files, 1 flaky assertion, none touching connectionId). `tsc --noEmit`
clean for every touched source file.

**Live-verified once more after v0.4.26's `terminal.multiplex` fix**: the
`terminal.subscribe` error was gone, real shell-prompt bytes were confirmed
flowing over the wire (decoded from the WS frame:
`ubuntu@tas-test:~$`) — but the terminal pane still never rendered a
prompt, and `terminal.inspectProcess` reported `INFRA_TERMINAL_NOT_FOUND`
~2s after creation. Root cause, found by re-reading (not re-deploying)
this session's OWN earlier edit: `OnboardingInlineCommandTerminal`'s
devServerId (resolved from `useActiveDevServer`/`useConnectedDevServers`,
which can start `null` and populate asynchronously) was listed in the
create-tab `useEffect`'s dependency array — the SAME class of bug the
persisted `bug-fe-pty-001-investigation` memory spent 13 layers fixing
(the create→destroy churn from a value flipping mid-mount): the first
`null`→real-value resolution tore the tab down and recreated it,
destroying the PTY that had just started streaming. Fixed with a ref
(`devServerIdRef`), matching the existing "set once, stable for this tab's
lifetime" treatment `worktreeId`/`shellOverride`/`title` already get —
`devServerId` deliberately removed from the dependency array. Added 2
render tests (`OnboardingInlineCommandTerminal.render.test.tsx`) —
**deliberately verified the first one FAILS without the fix**
(`createTab` called twice) before confirming it passes restored, per this
investigation's own established rigor.

**Live-verified again after v0.4.25's `terminal.create` handle fix**: the
frontend crash was gone, but the pane stayed blank with a
`terminal.subscribe is not yet implemented` error. Root cause and fix (a
frontend transport-selection correction plus a backend-go wire-key fix)
are documented in backend-go's solutions/README.md, "Twentieth" —
`remote-runtime-pty-transport.ts`'s `'session-auth'` branch now always
uses the binary `terminal.multiplex` protocol (which backend-go actually
implements) instead of the JSON `terminal.subscribe` fallback (which it
does not). New test:
`remote-runtime-pty-transport.test.ts`'s `subscribes via terminal.multiplex
for session-auth, not the JSON fallback`.

**Live-verified this fix incomplete on first deploy (v0.4.21):** the
actual "Enable Orca CLI" button in Settings (`AgentCapabilitiesSetupAction`,
feature-wall) renders `FeatureSetupInlineTerminal`, NOT
`CliSkillSetupTerminal` — a completely different, undecorated call site
that still had no `devServerId` at all, so it still hit
`INFRA_TERMINAL_HOST_LOCAL_UNIMPLEMENTED` identically. Traced every real
caller of `FeatureSetupInlineTerminal` (`impact()` run first, LOW risk, 6
impacted/3 direct — the connectionId-forwarding mechanism itself was
already proven correct by the tests above; this was purely a missed
wiring site): `AgentCapabilitiesSetupAction.tsx` (Settings' "Install CLI &
Skills", the one actually reached from Settings → Onboarding checklist),
`FeatureWallBrowserAction.tsx`'s `BrowserSkillInstallButton` (same feature
wall, browser-only variant) — both fixed with the identical
`useActiveDevServer`/`useConnectedDevServers` fallback pattern as
`CliSkillSetupTerminal`. The third caller, `onboarding/
AgentFeatureSetupStep.tsx`, is dead code (`impact()`: 0 callers anywhere —
confirmed via grep too) and was left alone.

`FeatureSetupInlineTerminal` itself gained the same `devServerId` prop and
passthrough. 2 new tests in `FeatureSetupInlineTerminal.test.tsx` (mocks
`OnboardingInlineCommandTerminal` to assert the prop actually reaches it,
both when set and when omitted). `tsc --noEmit` clean; full sweep re-run,
same pre-existing-only failure set as above.

## Nineteenth: two more `callRuntimeRpc({kind:'local'})` channels never wired on backend-go (same bug class as the Sixteenth cli.* fix)

User's browser console surfaced two more console errors on Settings load:
`channel "grokAccounts.getStatus" is not yet implemented in backend-go` and
`channel "minimaxCredentials.getStatus" is not yet implemented in
backend-go`. Exact same bug class as the Sixteenth fix, found by pattern-
matching: `runtime-grok-accounts-client.ts`'s `getGrokAccountStatus` and
`runtime-minimax-credentials-client.ts`'s `getMiniMaxCredentialsStatus`/
`saveMiniMaxCredentialsCookie`/`clearMiniMaxCredentialsCookie` all called
`callRuntimeRpc({kind: 'local'}, '<channel>')` — i.e.
`window.api.runtime.call`, a real network call to backend-go, which
rightly has no `grokAccounts.*`/`minimaxCredentials.*` channels (Grok's CLI
config and MiniMax's session cookie both live on the user's own machine,
not on the server). `window.api.grokAccounts`/`window.api.minimaxCredentials`
were already correctly implemented on both platforms — real IPC on
desktop, an honest "unsupported"/not-configured stub on web
(`web-preload-api.ts`'s `createGrokAccountsApi`/`createMiniMaxCredentialsApi`)
— and needed no relay at all, exactly like `window.api.cli.*` in the
Sixteenth fix.

`impact()` run first per this repo's mandatory convention on both
functions — both LOW risk (max 4 and 3 impacted respectively, single
frontend-runtime-client candidate each). Fixed by calling
`window.api.grokAccounts.*`/`window.api.minimaxCredentials.*` directly,
dropping the `callRuntimeRpc` indirection entirely — same fix shape as the
Sixteenth entry. Behavior-preserving for Electron desktop; on web, callers
now get the correct stub response instead of an error.

4 new tests: `runtime-grok-accounts-client.test.ts` (1),
`runtime-minimax-credentials-client.test.ts` (3) — each asserting
`window.api.runtime.call` is never invoked, mirroring
`runtime-cli-client.test.ts`'s regression-guard pattern. Ran
`AccountsPane.test.tsx` (the one existing test suite covering a consumer of
these clients) — 6/6 pass, zero regressions. `tsc --noEmit` clean for both
touched files.

## Twentieth (correction): the "always use terminal.multiplex" fix above was wrong — reverted to the JSON fallback, backend-go implements it instead

Live WS-frame capture (script-based, not the multiplexer's own logs) showed
zero binary frames ever crossed the wire for `'session-auth'` connections.
Reading `web-session-client.ts` confirmed why: `handleSocketMessage` drops
any non-string frame outright, and the `sendBinary` it hands back from
`subscribe()` unconditionally throws `'Binary frames not supported in
session mode over this channel'`. The interface types `sendBinary`/
`onBinary` as present (satisfying `RemoteRuntimeMultiplexedTerminalCallbacks`
structurally), but the implementation stubs them — the prior entry's premise
("terminal.multiplex is what backend-go actually implements") was
backwards for this transport. See backend-go's solutions/README.md,
"Twenty-first", for the matching correction on that side.

Reverted `remote-runtime-pty-transport.ts`'s `'session-auth'` branch back to
`subscribeTerminalViaJson` (the plain-JSON fallback). Since backend-go had
no `terminal.subscribe` implementation at all, this shifted the real fix
to backend-go: a new `channels_terminal_subscribe.go` implementing
`terminal.subscribe`/`terminal.unsubscribe`/`terminal.updateViewport` as
JSON channels (documented on the backend-go side). Also fixed, via direct
WS-frame capture of the real frontend (not guesswork): a systemic
`"ptyId"/"data"` vs `"terminal"/"text"` field-name mismatch across
`terminal.send`, `terminal.close`, `terminal.focus`, `terminal.agentStatus`,
`terminal.isRunningAgent`, `terminal.inspectProcess`, `terminal.wait`, and
`terminal.multiplex`'s subscribe payload — the real frontend uniformly
sends `{terminal: <ptyId>, text: <data>, ...}`; backend-go's json tags
expected `{ptyId, data}`. `terminal.stop` sends `{worktree: ...}` instead —
a different, deeper gap, deliberately left alone/tracked separately.

Test corrections: `remote-runtime-pty-transport.test.ts`'s
`subscribes via terminal.multiplex for session-auth...` test (itself wrong)
replaced with `subscribes via the plain-JSON terminal.subscribe fallback
for session-auth, not terminal.multiplex`. Live-verified end to end via a
direct Node.js script (no screenshots, per this session's standing
instruction): real shell prompt renders, no destroy/recreate churn, and
genuine 2-way interactivity (typed input echoed back correctly).

## Twenty-first: `AgentSkillSetupPanel` — the SAME devServerId gap, an 11th-hour missed call site

Bug report: Settings → Orchestration → "Not installed" → Install threw
`INFRA_TERMINAL_HOST_LOCAL_UNIMPLEMENTED` — the exact same failure class as
the Eighteenth/Twentieth entries, but through a call site the earlier sweep
never reached. `AgentSkillSetupPanel.tsx` is a generic, reusable settings
panel (renders `OnboardingInlineCommandTerminal`) with 11 real callers
across Orchestration, Browser Use, Computer Use, ephemeral VMs, CLI
settings, mobile-emulator, and Linear integration surfaces — none of them
had ever been threaded with `devServerId`, because the earlier passes
(Sixteenth/Eighteenth) only fixed `CliSkillSetupTerminal`/
`FeatureSetupInlineTerminal`, two entirely separate components.

Fixed by adding a `devServerId?: string | null` prop to
`AgentSkillSetupPanel` (passed straight through to
`OnboardingInlineCommandTerminal`) and threading the established
`useActiveDevServer`/`useConnectedDevServers` fallback
(`activeDevServer?.status === 'connected' ? activeDevServer.id :
(connectedDevServers[0]?.id ?? null)`) into all 11 callers:
`OrchestrationPane.tsx`, `OrchestrationSetupCard.tsx`,
`BrowserUseSkillSetupCard.tsx`, `FloatingTerminalOrchestrationDialog.tsx`,
`ComputerUseSkillSetupPanel.tsx`, `EphemeralVmsPane.tsx`,
`BrowserUseSkillStep.tsx` (a dumb wrapper — added a passthrough prop, its
caller `BrowserUsePane.tsx` resolves and passes it), `CliSection.tsx`,
`MobileEmulatorAgentControlRow.tsx`,
`MobileEmulatorAgentSetupGuideSteps.tsx`, and
`LinearAgentSkillSetupDialog.tsx` (dumb wrapper, same pattern — its caller
`LinearAgentSkillSetupPrompt.tsx` resolves and passes it).

While verifying this, `EphemeralVmsPane.test.tsx` broke:
`useConnectedDevServers` now runs for real inside that render test, and its
hand-rolled `mockStoreState` (mocking `@/store`) had no `devServers` field
— `s.devServers.filter(...)` threw on `undefined`. Fixed by adding
`devServers: []`/`activeDevServerId: null` to that test's mock state; no
other caller's existing test exercises the real component tree deeply
enough to hit the same gap (`CliSection.test.tsx`/
`LinearAgentSkillSetupPrompt*.test.tsx` mock `AgentSkillSetupPanel` itself
out, so they never call the real hook).

Also found, incidentally, while adding a 4th test file that imports
`store/slices/dev-servers`: a **pre-existing circular-import bug** —
`dev-servers.ts` is imported BY `store/index.ts` (for `createDevServerSlice`)
but also had its own top-level `import { useAppStore } from '../index'` for
its 4 convenience hooks (`useDevServers`, `useActiveDevServer`,
`useConnectedDevServers`, `useDevServerById`) — a genuine 2-way module
cycle. Any test that pulled in `dev-servers.ts` transitively for the first
time hit `TypeError: createDevServerSlice is not a function`. A namespace-
import attempt (`import * as storeIndex from '../index'`) did NOT fix it —
same error — proving the cycle blocks at module-evaluation time, not at
property-access time. Fixed by physically splitting the file: `dev-servers.ts`
now holds only the slice (`DevServerSlice`/`createDevServerSlice`, zero
`useAppStore` dependency), and a new `dev-servers-selectors.ts` holds the 4
hooks with their own top-level `useAppStore` import — safe, since
`store/index.ts` never imports the new file. 27 importing files bulk-updated
to import from `dev-servers-selectors` instead (one `vi.mock()` string
literal in `useActiveDevServerPlatform.test.ts` needed a manual fix the
bulk sed missed). `dev-servers.test.ts` needed no changes — it only tests
the slice reducer logic via a hand-rolled store, never the hooks.

`tsc --noEmit`: zero new errors in any touched file (one pre-existing,
unrelated `Cannot find module '.../shared/dev-server-types'` path error in
`dev-servers.test.ts` predates this fix). Full `vitest run` sweep across
every test file that actually renders one of the 11 touched callers or
`AgentSkillSetupPanel`/`dev-servers-selectors` — all pass. A broader sweep
across `store/slices`/`sidebar`/`onboarding` surfaced ~58 already-failing
test files unrelated to this change: traced to a separate, pre-existing
circular-import bug in `repos.ts` → `onboarding-project-checklist.ts` →
`store/index.ts` (same bug class, different slice, zero diff on any of
those three files against HEAD — confirmed present before this session's
edits) plus unrelated pre-existing assertion mismatches; none reference
`dev-servers`/`AgentSkillSetupPanel`. Left untouched — out of scope for this
fix, flagged for separate follow-up.

## Twenty-second: Automation page — `AUTOMATION_LIST_RUNS_FAILED`, then a masked `nextAutomations.some` crash

Full root-cause (a Postgres UUID-bind bug in `automation-service`, then a
proto3-omitempty gap `AUTOMATION_LIST_RUNS_FAILED` had been masking) is
documented in backend-go's solutions/README.md, "Twenty-second" — this
entry covers only the frontend-side defense-in-depth added alongside it.

`listAutomationsForTarget`/`listAutomationRunsForTarget`
(`automation-host-client.ts`) read `result.automations`/`result.runs`
directly with no fallback; a genuinely-empty tenant's response omits that
key entirely (proto3 `repeated` fields marshal with `omitempty`), so
`AutomationsPage.tsx`'s `refresh()` crashed on `nextAutomations.some(...)`
with `Cannot read properties of undefined`. Both functions now return
`result.automations ?? []`/`result.runs ?? []` — cheap insurance
independent of whatever the backend does, since this is the same "proto3
omits an empty repeated field" gap this session already hit once in
`specs/backend-go/bugs/missing-v2/BUG-005`, and nothing guarantees no wire
response ever regresses back to omitting the key. 2 new tests in
`automation-host-client.test.ts` assert the fallback for both functions.
`tsc --noEmit` clean; `automations/` test directory (16 files, 108 tests)
re-run clean.

**Noted, not fixed here**: `AutomationsPage.tsx`'s `refresh()` has no
`catch` at all — any rejection in its `Promise.all` (RPC timeout, network
blip, a future backend regression) surfaces as a raw unhandled-rejection
console error with no user-facing message and no retry affordance, exactly
what the original bug report showed. Flagged as follow-up scope, not
addressed in this pass since the two underlying causes (backend query,
proto3 omitempty) are now both closed and no live failure remains to
reproduce a `refresh()` error-handling fix against.
