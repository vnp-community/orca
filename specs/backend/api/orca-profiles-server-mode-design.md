# `orcaProfiles.*` in server/web mode — investigation (not implemented)

## Status

**Design doc only — no code changed.** Written in response to an explicit
request to resolve the "Group C — needs a product decision" entry for
`orcaProfiles.*` in `desktop-only-rpc-parity-gaps.md`, following this
session's established pattern (`ephemeralVm.*`, `claudeAccounts.*`): read the
actual desktop source before guessing, and don't force a mechanical port onto
a namespace collision that turns out to be a coincidence.

## The naming-collision question, answered

**`orcaProfiles.*` and backend's `profile.*` are two unrelated concepts that
happen to share the English word "profile." They are not the same feature,
not aliases of each other, and neither is a superset of the other.**

| | `profile.*` (backend, already ported) | `orcaProfiles.*` (desktop, this doc) |
|---|---|---|
| Backing module | `backend/src/main/profile/{ProfileService,ProfileResolver,OrcaProfile}.ts` | `desktop/src/main/orca-profiles/*.ts` (40 files) |
| Storage | Postgres (`orca_companies`/`orca_departments`/`orca_user_profiles`/`orca_teams`, ADR-021) | Local JSON files under Electron's `app.getPath('userData')` (`profile-storage-paths.ts`) |
| What it models | One logged-in server user's **config cascade**: Company → Department → Team → User, merged into agent/editor/shell/security settings (`docs/guides/user-profile-team-department-rbac.md`) | Multiple **local app identities on one machine** (like Chrome/Firefox profiles), each with its own repos/sessions/settings, one of which is "active" in the current process |
| "org"/"member" | No such concept — `orca_companies`/`orca_departments` are admin-managed, no invite/role flow in this namespace | A **separate external product**: Orca's own cloud SaaS (`ORCA_CLOUD_API_URL`, PKCE OAuth to `/v1/desktop/auth/*`), with its own orgs, member roles (`owner`/`admin`/`member`), email invites |
| Switching identity | N/A — resolved per-request from `RpcContext.userId`, no "switch" verb | `orcaProfiles.switch` **kills and relaunches the entire process** pointed at a different local data folder |
| Type name collision | `backend/src/main/profile/OrcaProfile.ts` exports `type OrcaProfile` (a profile-settings **shape**: `agent`/`editor`/`shell`/`security` sections) | `desktop/src/shared/orca-profiles.ts` exports unrelated types (`OrcaProfileSummary`, `OrcaProfileIndex`, `OrcaProfileAuthStatus`, ...) describing a **local-identity record**, not a settings shape |

The `OrcaProfile` type name collision is confirmed coincidental, not the same
concept re-declared: `backend/src/main/profile/OrcaProfile.ts` (also mirrored
verbatim in `desktop/src/main/profile/OrcaProfile.ts` — desktop has **both**
systems side by side under different directory names, `profile/` vs.
`orca-profiles/`) is `{ agent?, editor?, shell?, mcp?, security?, envVars? }`
— a merged settings bag. The `orcaProfiles.*` RPC namespace never imports or
returns that type at all; its "profile" is `OrcaProfileSummary` from
`desktop/src/shared/orca-profiles.ts` — `{ id, name, avatar, kind: 'local' |
'cloud-linked', createdAt, updatedAt, lastOpenedAt, cloud? }`, an identity
record, not a settings bag.

## What `orcaProfiles.*` actually is (16 methods, read from source)

`desktop/src/main/runtime/rpc/methods/orca-profiles.ts` is one wrapper per
`orcaProfiles:*` ipcMain channel
(`desktop/src/main/ipc/orca-profiles.ts`,
`desktop/src/main/ipc/orca-profile-org-members-handlers.ts`), backed by
`desktop/src/main/orca-profiles/*.ts`. It is Electron's version of a browser
"profile switcher" (Chrome/Firefox multi-profile), **not** a multi-tenant
directory service:

- **`list`, `createLocal`, `switch`** — `profile-index-store.ts` maintains
  `orca-profile-index.json` under `app.getPath('userData')` (one Electron
  install's local app-data folder on **this one machine**), listing named
  local identities, each with its own subdirectory holding its own
  `orca-data.json` (repos, sessions, settings). `switch` writes the new
  `activeProfileId` to that file, then calls `scheduleProfileRelaunch()` —
  it **restarts the Electron main process** so the next boot loads the target
  profile's data directory. There is no per-request identity here; "the
  active profile" is a property of the running process, not of a caller.
- **`transferProject`, `findProjectProfiles`** — move/copy a repo's local
  state (`profile-project-transfer.ts`, `profile-project-presence.ts`)
  between two of those same-machine profile directories by reading/writing
  each profile's `orca-data.json` directly. Both take a `userDataPath`
  parameter that is always `getProfileUserDataPath()` — this machine's
  Electron user-data path — for both the "source" and "target" profile;
  there is no such thing as a *different machine's* profile here.
- **`authStatus`, `connectCurrent`, `createCloudLinked`, `refreshAuth`,
  `signOutCurrent`, `selectOrg`, `orgMembersList`, `orgMemberInvite`,
  `orgInviteRevoke`, `orgMemberChangeRole`, `orgMemberRemove`** — a real
  OAuth2/PKCE client (`profile-cloud-pkce.ts`, `profile-cloud-client.ts`) for
  **Orca's own separate cloud SaaS product**, confirmed by
  `profile-cloud-auth-config.ts`: `ORCA_CLOUD_API_URL` +
  `ORCA_CLOUD_CLIENT_ID` env vars, HTTPS-only endpoints
  (`/v1/desktop/auth/{authorize,session,refresh,capabilities,profile,org,logout}`).
  Every one of these functions starts with
  `ensureActiveOrcaProfile(userDataPath)` — "whichever local profile this
  Electron process currently has active" — then reads/writes that *specific
  local profile's* cloud session token from its own on-disk session store
  (`profile-cloud-session-store.ts`) before calling the cloud API with that
  token as the bearer credential. The "org" here is an org inside that
  external cloud product (invite by email, `owner`/`admin`/`member` roles) —
  unrelated to `orca_companies`/`orca_departments` in Postgres.

Confirms the task's hypothesis exactly: this is "one desktop app instance,
one local human, optionally linked to one cloud identity," where "switch
profiles" means "restart this machine's app pointed at different local
state." It is a real, working feature — not vestigial — but its unit of
identity is *the running desktop process*, not *the authenticated request*.

## Why it doesn't map onto server mode — the genuine blockers

Server mode's whole model (`ORCA_AUTH_MODE=local`/`ORCA_MULTI_USER=1`, this
branch's ADR-021 Postgres consolidation) is: one shared backend deployment,
many already-authenticated users, each request/process resolves identity from
`RpcContext.userId` → `TenantResolver`/`ProfileService.getCompanyIdForUser`.
`orcaProfiles.*` conflicts with that at three independent points, not one:

1. **Process-restart semantics have no server-mode counterpart.**
   `orcaProfiles.switch` relaunches *the entire process* to change *whose*
   data is active. In server mode there is no single process a single caller
   owns — restarting the process that handles this RPC would restart the
   session for every concurrently connected user this backend replica is
   serving (or, under `ORCA_MULTI_USER=1`'s one-fork-per-authenticated-user
   model, restarting "your" fork mid-request has no defined meaning for an
   RPC call that must return a response). This is not a missing-piece gap
   like `ephemeralVm.*`'s missing SSH client — it's a structural mismatch
   between "identity = which process is running" and "identity = which
   already-authenticated user made this request."

2. **Storage model actively regresses ADR-021.** Every method reads/writes
   flat JSON under one machine's `app.getPath('userData')`. Backend just
   finished consolidating server-mode state into one shared Postgres
   (`413f5c8da`, tip commit on this branch) specifically so multiple backend
   replicas/forks see consistent state. Reintroducing a parallel
   filesystem-JSON "profile" store — on a container filesystem that may not
   even persist across restarts, and is never shared across replicas — would
   both contradict that migration and silently lose data in exactly the
   deployment shape server mode targets.

3. **The cloud-linking half raises a product question, not an engineering
   one, and it's un-scoped by design here.** `createCloudLinked`/`authStatus`/
   `org*` are a real client for Orca's own external SaaS. Porting them
   mechanically begs the question the code never answers today: *which*
   identity should a shared multi-user backend deployment authenticate to
   Orca Cloud as — one operator-configured link shared by every backend
   user, or a separate PKCE flow per authenticated server user (who then
   needs their own browser-based OAuth redirect in a headless container
   context, unlike desktop's native browser-window PKCE flow via
   `profile-cloud-pkce.ts`)? Nothing in the existing code or specs answers
   this; it needs a product decision before any of the 11 cloud/org methods
   can be scoped, let alone implemented.

Unlike `ephemeralVm.*` (where a real Group 1/Group 2 split existed — some
methods were cheaply groundable today), no subset of these 16 methods is
safely portable in isolation: `list`/`createLocal`/`switch`/`transferProject`/
`findProjectProfiles` all depend on the same-process local-profile-index
concept from point 1–2 above; all 11 cloud/org methods depend on
`ensureActiveOrcaProfile` from the same index, so they inherit the same
blocker rather than avoiding it.

## Recommendation

**Do not port `orcaProfiles.*` to server mode.** Unlike `ephemeralVm.*`
(genuinely gate-able into a real v2), this is not "missing infrastructure" —
it's a feature whose entire premise (one human, one machine, switchable local
app identity) doesn't have a server-mode analogue to build toward. If Orca
Cloud org/team management is wanted for server-mode deployments, that is new
product surface — most naturally an admin-facing extension of backend's
existing `profile.*`/`ProfileService` (which already has the
company/department hierarchy and Postgres backing this would need), not a
port of desktop's per-machine profile switcher. That would be a separate,
explicitly-scoped feature request, not a "port `orcaProfiles.*`" task.

Classify `orcaProfiles.*` as permanently desktop-only (moved from Group C to
the same bucket as `mobile.*`/`updater.*`/`pet.*` — machine-identity
concepts, not missing infrastructure), and add it to the frontend's
desktop-only suppressor so its expected `method_not_found` errors don't spam
the console in server/web mode (see
`frontend/src/renderer/src/runtime/desktop-only-rpc-error-suppressor.ts`).

## Files read for this investigation

- `desktop/src/main/runtime/rpc/methods/orca-profiles.ts` — the 16-method RPC surface
- `desktop/src/main/orca-profiles/profile-storage-paths.ts`,
  `profile-index-store.ts`, `profile-project-transfer.ts`,
  `profile-project-presence.ts`, `profile-cloud-auth-config.ts`,
  `profile-cloud-client.ts`, `profile-cloud-service.ts`,
  `profile-cloud-org-members-service.ts`, `profile-ui-scope.ts`
- `desktop/shared/orca-profiles.ts` → actually `desktop/src/shared/orca-profiles.ts`
  — `OrcaProfileSummary`/`OrcaProfileIndex`/`OrcaProfileAuthStatus` etc.
- `backend/src/main/profile/{ProfileService,ProfileResolver,OrcaProfile}.ts`,
  `backend/src/main/profile/profile-rpc-handler.ts` — the unrelated `profile.*`
  namespace already live in server mode, compared field-by-field above
- `desktop/src/main/profile/OrcaProfile.ts` — desktop's own copy of the
  company/dept/user settings-cascade type (confirms desktop runs both
  systems side by side under different directory names)
- `specs/backend/api/desktop-only-rpc-parity-gaps.md` §C — the prior "needs a
  decision" classification this doc resolves
- `specs/backend/api/ephemeral-vm-server-mode-design.md` — the calibration
  precedent for when a Group C namespace turns out groundable vs. not
