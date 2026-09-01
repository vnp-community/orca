# backend-go Solutions — Dev Server Access Control

**CRs:** [docs/crs/v2/dev-server/CR-DS-006..008](../../../../../docs/crs/v2/dev-server/README.md)
**Frontend counterpart:** [specs/frontend/crs/v3/dev-server-access-control/](../../../../frontend/crs/v3/dev-server-access-control/)

## Solutions

| Solution | CR | Phase | Status |
|---|---|---|---|
| [BE-SOL-001](./BE-SOL-001-dev-server-status-and-groups.md) | CR-DS-006 | 1 — data model | ✅ COMPLETED (2026-08-28) |
| [BE-SOL-002](./BE-SOL-002-admin-approval-rpc.md) | CR-DS-006 | 2 — admin approval | ✅ COMPLETED (2026-08-28) |
| [BE-SOL-003](./BE-SOL-003-department-group-mapping-and-opa.md) | CR-DS-007 | 2 — dept/team access control | ✅ COMPLETED (2026-08-28) |
| [BE-SOL-004](./BE-SOL-004-access-request-flow.md) | CR-DS-008 | 3 — access request | ✅ COMPLETED (2026-08-28) |

**All backend work for CR-DS-006/007/008 is done and deployed.** Frontend
(FE-SOL-001/002/003) is the only remaining piece — see
[specs/frontend/crs/v3/dev-server-access-control/](../../../../frontend/crs/v3/dev-server-access-control/solutions/README.md).

## What shipped in this pass (beyond the 4 solutions themselves)

A prerequisite discovered mid-implementation, not originally scoped in any
solution doc: **admin-gating had no working mechanism anywhere in
backend-go** — `common/tenant` carried only `tenant_id`/`user_id`, no role.
Fixed as shared, additive plumbing (verified via `go build ./...` across
all 17 services + new unit tests):

- `common/tenant.WithRole`/`Role` — new context accessor.
- `common/grpcmw.MetadataRole` + `TenantExtractionInterceptor` now reads it.
- `api-gateway`'s `usecase.Identity`/`wscompat.Identity` gained a `Role`
  field; `authclient.SessionValidator.ValidateToken` now populates it from
  auth-service's `ValidateSession` response (previously silently discarded
  — `session_validator.go`'s `roleString` helper).
- `gatewaygrpc.AttachIdentity` now stamps `MetadataRole` too.
- infra-fleet-service's new `usecase.requireAdmin` reads `tenant.Role(ctx)`,
  fails closed (`PermissionDenied`) on anything but exactly `"admin"` —
  the opposite failure mode from project-service/annotation-service's
  pre-existing `callerGlobalRole` stub (always `""`, harmlessly inert)
  because these ARE the RPCs that enforce something now.

Only the specific wscompat channels this pass added
(`channels_dev_server_access_control.go`) attach `Role` — the other ~100
existing `usecase.Identity{...}` construction sites in wscompat are
untouched (Role defaults to `""`, same as before this pass).

**Separately found and fixed while wiring this pass's responses**: any
wscompat channel that returns a raw `*infrafleetv1.X` proto message
directly (`return resp.GetDevServer(), nil`) ships **snake_case** JSON keys
on the wire (`encoding/json` reads protoc-gen-go's own struct tags, not the
camelCase-oriented `protojson` tag) — invisible to a camelCase-typed
frontend. This pass's channels all go through an explicit view struct
(`devServerGroupView`, `devServerGroupGrantView`,
`devServerAccessRequestView`, and `devServerView` extended with
`approvalStatus`/`groupId`). This same latent bug likely affects other
pre-existing channels that return raw proto messages (e.g. `repo.add`'s
`resp.GetRepo()`) — **not fixed here**, out of this pass's scope; flagging
for whoever picks that up next.

## Critical gap found live after deploy: connected agents never reached dev_servers

Reported by the user: 3 real dev-server agents connected (systemd services
active, agent logs showing successful `agent.handshake`) but the Admin
Console's Approvals tab showed nothing. DB check: `infra.dev_servers` had
**zero rows**.

Root cause, pre-existing (predates CR-DS-006, ported as-is from the legacy
TS `agent-token-routes.ts`): the direct-websocket agent-connection pathway
(`POST /api/agent-token` → `agentwsserver.Registry` in-memory slot → agent
dials in → `devserveragent.Client.AttachInboundSession`) never called
`RegisterDevServer` — it only ever tracked live sessions in memory, keyed
by the caller-supplied `devServerId` string, entirely disconnected from the
SQL-backed table every usecase in BE-SOL-002/003/004 operates on. The
entire admin-approval model assumed a `dev_servers` row exists for every
connected agent; for this connection mode, that was never true.

Fixed with a find-or-create pattern mirroring `FindBySshTarget`/
`EstablishConnection`'s existing precedent:

- `DevServerRepository.FindByHostAndMode` (new interface method + Postgres
  impl) — `host` doubles as the agent's external `devServerId` string for
  direct-websocket mode, a column otherwise unused there.
- `ResolveDirectWebSocketDevServer` usecase: find-or-create by
  (tenant, devServerId, direct-websocket), preserving an existing row's
  `approval_status`/`group_id` across agent reconnects.
- `TokenIssuer` (`/api/agent-token`) calls this at mint time and registers
  the **resolved row's real UUID** — not the raw string — as the
  `Registry` slot key, so `AttachInboundSession`'s session key matches
  `domain.DevServer.ID` exactly.
- `agentwsserver.Config` gained `DefaultTenantID`
  (`ORCA_AGENT_DEFAULT_TENANT_ID`, falling back to the bootstrap tenant
  sentinel `00000000-0000-0000-0000-000000000001`, confirmed live) since
  this endpoint authenticates via a shared secret, not a per-user session —
  no tenant to pull from request context. Correct for today's
  single-tenant-per-deployment reality; true multi-tenancy would need this
  per-token, a follow-up.
- Fails open on a resolve error (logs, keeps old raw-string behavior) —
  a DB hiccup must not also break agent connectivity.

New/updated tests: `resolve_direct_websocket_dev_server_test.go` (4 tests),
`token_endpoint_test.go`'s new resolver-wiring regression test, 2 fakes
extended for the new interface method. All 17 services build clean, full
`infra-fleet-service` suite passes. Deployed as version 0.4.4.

## Fourth live bug: INFRA_NOT_ADMIN for the actual bootstrap admin

Reported live: `devServer.approve` and `devServerGroup.create` both failed
with `PermissionDenied: INFRA_NOT_ADMIN` for the bootstrap admin account,
despite every layer of the Role-propagation chain (session validator →
attachAdminIdentity → AttachIdentity → TenantExtractionInterceptor →
requireAdmin) checking out correctly in isolation. Root cause, found via
infra-fleet-service's structured logs (which showed `tenant_id`/`user_id`
correctly resolved but hinted nothing about role): **`wscompat.Registry.Dispatch`'s
own `AttachIdentity` call — which runs before every single channel handler,
not just admin-gated ones — omitted `Role` entirely**, appending
`MetadataRole=""` to the outgoing gRPC metadata. `attachAdminIdentity`'s own
call inside each admin-gated handler ran a moment later, appending the real
`MetadataRole="admin"` — but `grpc/metadata.AppendToOutgoingContext` appends
rather than replaces, and `TenantExtractionInterceptor`'s
`md.Get(MetadataRole)[0]` on the receiving side reads only the FIRST value
in that slice. Dispatch's call always runs first, so its empty value always
won — the real "admin" value was silently discarded on every single
admin-gated call, unconditionally, regardless of the caller's actual role.

`tenant_id`/`user_id` were unaffected because Dispatch's call already set
those correctly (their value doesn't change between the generic and
admin-specific call) — only Role differed between the two appends, which is
exactly the field that got lost.

Fixed by adding `Role: id.Role` to Dispatch's (and the equally-affected
`DispatchStreamChannel`'s) existing `AttachIdentity` call — the one call
that runs first for literally every channel, admin-gated or not — so the
correct role is authoritative before any handler-level second call can
matter. `attachAdminIdentity`'s own redundant call becomes harmless once
this lands (same value appended twice, first one still wins) — left
unchanged rather than removed, to keep this fix minimal. Separately found
and fixed the identical bug pattern in `httpgateway/middleware.go`'s
`authMiddleware` (a plain `usecase.Identity{...}` literal dropping Role) —
not yet reachable by any live REST route, but the same latent trap for the
first admin-gated REST endpoint that comes along.

2 new regression tests in `registry_test.go`
(`TestDispatch_AttachesRoleToContext`,
`TestDispatch_RoleSurvivesASecondAttachIdentityCall` — the latter
specifically simulates the double-AttachIdentity-call scenario and asserts
on metadata index `[0]`, the exact thing the interceptor reads). Full
`api-gateway` suite passes; all 17 services build clean.

## Fifth live gap: no way to create a department, or manage any user

User asked directly: "Grant a department access" showed nothing, where do
I create a department, where do I manage the list of users, how do I
assign role/department/group. Investigation found the org/user
administration side of this feature was **entirely unbuilt** — a
different, broader gap than CR-DS-006/007/008 itself (which only covers
dev servers), but blocking it end to end:

- `tenant-service.CreateCompany`/`CreateDepartment` RPCs existed but had
  zero wscompat channels and zero UI — nothing could ever create a
  department, which is why the grant picker was empty.
- The bootstrap admin's tenant had **zero rows in `tenant.companies`** —
  seeded through an older path that predates `Bootstrap.EnsureAdmin`'s
  current correct saga (which properly originates a tenant_id via
  `CreateCompany` first). Backfilled with a new migration
  (`0002_backfill_legacy_bootstrap_company`, idempotent, scoped to the
  well-known sentinel tenant_id — harmless on any other deployment).
- `auth-service`'s full user-management RPCs (`ListUsers`,
  `UpdateUserRole`, `DeactivateUser`, `ReactivateUser`) existed with zero
  wscompat channels and zero UI. Wired all four as new admin-gated
  `admin.*` channels (`channels_admin_users.go`) — **`CreateUser` was
  deliberately left unwired**: its own usecase doc comment documents that
  a created account gets a random, never-returned password (no invite/
  reset-link flow exists), so exposing it would create unusable accounts.
- Fixed the same snake_case-JSON bug on `profile.updateCompany`/
  `updateDept` (found while adding `profile.createCompany`/`createDept`
  next to them) — both were still returning raw proto messages.
- Extended `profile.updateUser`'s reach: added `tenantProfile.setUserDepartmentFor`
  (frontend) so an admin can set *any* user's department, not just their
  own (the existing `DepartmentGate` self-service path).

New Go test file `channels_admin_users_test.go` (6 tests: admin-gating
guards + success paths for all 4 channels). Full `api-gateway` suite
passes; all 17 services build clean.

## Sixth live bug: OPA policy bundle was empty in production (deploy config)

`admin.listUsers` failed with `AUTH_NOT_ADMIN` for the actual bootstrap
admin, even though this layer's own `id.Role != "admin"` gate had already
passed (proving the caller really was admin). Root cause, entirely outside
this feature's own code: `deploy/dev/docker-compose.yml`'s OPA bundle bind
mount (`../../backend-go/policy/orca-authz`, used by
auth-service/task-service/annotation-service/project-service's own
`requireAdminActor`-style checks) assumed a full monorepo checkout on the
server — but `sync-to-server.sh` only ever ships `deploy/dev/`'s own tree
(bin/dist/config), never the full `backend-go/` source. Docker silently
bind-mounts an **empty** directory for a missing host path instead of
erroring, so `admin.rego` never loaded — `requireAdminActor` failed closed
for literally every caller, on every one of those four services, regardless
of role, since this docker-compose.yml was first written (unrelated to
CR-DS-006/007/008; just the first thing to actually exercise it end to end).

Confirmed via `docker inspect orca-go-auth`: the container's real mount
source was `/home/backend-go/policy/orca-authz` — nothing lives there.

Fixed by making the deploy self-contained instead of assuming a sibling
checkout: `build-local.sh` now copies `backend-go/policy/orca-authz/` into
`deploy/dev/policy/` on every build (already covered by
`sync-to-server.sh`'s existing rsync step, gitignored like `bin/`/`dist/`),
and all 4 `docker-compose.yml` mounts now read `./policy/orca-authz`
instead of the broken `../../` path. Fixes every OPA-gated admin check on
all four services at once, not just this pass's new `admin.*` channels.

## Seventh: real multi-company support, per explicit user request

User confirmed (after being warned there's no company-switcher, so a new
company is only reachable by immediately creating its first admin) they
want real multi-company support, not just renaming the existing one. This
required fixing the actual blocker: `CreateUser` always generated a random,
never-returned password (no invite/reset-link flow), so a newly created
account — including a new company's first admin — could never log in.

Fixed at the source rather than left as a permanent gap: `CreateUserRequest`
gained an optional `password` field (empty = auto-generate, same as
before); `CreateUserResponse` gained `generated_password`, set only when
the caller didn't supply one — returned exactly once, never stored, never
logged. `CreateUser.Execute`'s signature changed from `(domain.User, error)`
to `(CreateUserOutput, error)` to carry both. Enforces an 8-character floor
on an admin-supplied password (no prior convention existed anywhere in this
service to match). New `admin.createUser` wscompat channel (previously
deliberately left unwired for exactly this reason) — admin-gated, defaults
`tenantId` to the caller's own company, accepts an explicit one for the
cross-tenant "bootstrap a new company's first admin" case.

**Known consideration, not fixed here**: `requireAdminActor`'s OPA check
only verifies the actor's role, not that the target `tenant_id` matches the
actor's own — any admin can already create a user under any tenant_id they
specify (pre-existing backend behavior, not introduced by this channel).
Acceptable for now given this is the one path multi-company bootstrap needs
into a system with no company-switcher yet.

6 new/updated Go tests (`create_user_test.go`'s two new password-behavior
tests, `channels_admin_users_test.go`'s three new `admin.createUser`
tests). All 17 services build clean; full `auth-service`/`api-gateway`
suites pass.

## Eighth: onboarding.detectAgents — "No agents detected" for every web user

User reported the "Pick your default agent" onboarding step always showed
"No agents detected on your PATH", regardless of which dev server they'd
connected. Root cause: `preflight.detectAgents` (the mechanism
`use-onboarding-flow.ts` actually called) is gated in `web-preload-api.ts`
on `requireActiveEnvironmentOrNull()` — a check for a paired Electron
desktop "runtime environment", a completely different, older concept from
this CR's dev-server-agent connection (`settings.activeDevServerId`). A
plain browser session never has one, so detection always resolved to `[]`.
`window.api.onboarding.detectAgents({devServerId})` and the
`useRemoteAgentDetection` hook already existed in the frontend's type/hook
surface (evidently built for this) but were never wired to anything real:
no backend-go channel answered `onboarding.detectAgents` at all, and
`AgentStep`/`use-onboarding-flow.ts` never called the hook.

Fixed without adding any new infra-fleet-service RPC — resolved via the
same `devServerId → connectionId → Relay` path
`registerAccountsResolveDevServerConnectionChannel`/`registerAccountsRelay`
(`channels_accounts.go`) already established:

1. New `onboarding.detectAgents` wscompat channel
   (`channels_onboarding.go`, `registerOnboardingChannels` now takes an
   `infrafleetv1.InfraFleetServiceClient`): decodes `{devServerId, commands}`,
   calls `client.ResolveConnection({devServerId})` (not-connected → returns
   `{agents: []}`, a legitimate state, not an error), then
   `client.Relay({connectionId, method: "preflight.detectAgents", paramsJson})`
   — `preflight.detectAgents` is a REAL, confirmed agent-side RPC
   (`specs/agent/api/agent-rpc-catalog-runtime.md`: `commands` in,
   `{agents, platform}` out), so this is a pure relay, no new agent-side
   work needed.
2. `commands` (the probe catalog: `{id, cmd, requiredCommands?,
   unsupportedRuntimes?}[]`) is built client-side from `TUI_AGENT_CONFIG`
   (new `frontend/src/shared/agent-detection-commands.ts`, mirroring
   `desktop/src/shared/agent-detection-commands.ts`'s
   `buildAgentDetectionCommands()` exactly) and passed through verbatim —
   this service does not duplicate the agent catalog.
3. `use-onboarding-flow.ts` now branches on `settings.activeDevServerId`:
   when set, `detectedAgentIds`/`isDetectingAgents` (and the mount-time
   auto-select effect) come from `useRemoteAgentDetection(activeDevServerId)`
   instead of the local-PATH `refreshDetectedAgents()` store slice, which is
   fundamentally inapplicable to a browser session. `activeDevServerId` null
   (Electron desktop) keeps the original local-PATH path untouched.

3 new Go tests (`TestOnboardingDetectAgentsChannel_RequiresDevServerID`,
`_NotConnectedReturnsEmpty`, `_RelaysToConnectedDevServer`) plus 2 new
frontend preload tests. `api-gateway` build/tests clean.

## Ninth: profile.listCompanies — a created company had nowhere to be found

Mid-turn, user reported creating a new company + its first admin, then on
refresh finding no trace of it anywhere in Settings → Organization →
Company. Root cause: `tenant.CompanyRepository` (and its wscompat/gRPC
surface) had a `Create`/`Get`/`Update`/`Exists` but no `List` — the Company
tab's `getCompany()` always defaults to the CALLER's own tenant, so a newly
created company was reachable only for the rest of the creating session's
in-memory state (the inline `NewCompanySection` success card), never again
after a reload. Nothing anywhere in the stack could enumerate
`tenant.companies`.

Fixed end-to-end: `CompanyRepository.List(ctx) ([]domain.Company, error)`
(new interface method + postgres impl, `ORDER BY name`), `ListCompanies`
usecase, `TenantService.ListCompanies` proto RPC (`ListCompaniesRequest{}`
→ `ListCompaniesResponse{companies}`), gRPC handler, and an admin-gated
`profile.listCompanies` wscompat channel returning the same
`{companies: [...]}` wrapped-array shape this CR's other list channels
already use. Genuinely cross-tenant by nature (`tenant.companies` has no
tenant_id column — it IS the tenant root) — every layer's doc comments call
out that the caller MUST admin-gate, which `profile.listCompanies` does the
same way as `profile.createCompany`/`createDept`.

4 new Go tests (`TestListCompanies_*` ×3, `TestProfileListCompaniesChannel_*` ×2).
`tenant-service`/`api-gateway` build/tests clean.

## Tenth: admin.listUsers never surfaced department assignment

Mid-turn, user reported: Users tab → assign a department → click Assign →
succeeds → refresh page → looks reverted. The assign itself DID persist
(`profile.updateUser` → tenant-service's `UpdateUserProfile` →
`tenant.user_profiles`) — but `admin.listUsers` sources rows from
auth-service's `auth.users` only, which has no department column at all (a
completely separate service/table from tenant-service's per-user profile).
The Users tab's department `<Select>` was seeded from purely local
component state (`departmentChoice`), which resets to the placeholder on
every reload — so a successful assign always LOOKED like it silently
reverted, because there was never anywhere for the persisted value to be
displayed from.

Fixed by joining department onto `admin.listUsers`'s response server-side:
`userView` gained `departmentId`; a new `attachDepartmentID` helper calls
`tenantClient.GetUserProfile(userId)` per row (admin-console-scale N+1,
documented as such — no bulk profile-lookup RPC exists yet) and degrades
`TENANT_PROFILE_NOT_FOUND`/any error to `""` rather than failing the whole
list. `registerAdminUserChannels` now also takes a
`tenantv1.TenantServiceClient`. Frontend: the department `<Select>` now
falls back to `user.departmentId` when there's no unsaved local pick, and
`handleDepartmentAssign` reloads the list (and clears the local pick) on
success instead of leaving the stale local state in place.

4 new Go tests (`TestAdminListUsersChannel_EnrichesDepartmentID`,
`_NoProfileYetLeavesDepartmentIDEmpty`, plus the pre-existing suite still
green). `api-gateway` build/tests clean.

## Eleventh: /api/vapid-public-key always failed with NOTIFICATION_NO_TENANT

User reported finishing onboarding and seeing `GET /api/vapid-public-key`
respond `{error: {code: "Unauthenticated", message: "NOTIFICATION_NO_TENANT:
no tenant in request context"}}`. Root cause: `GetVapidPublicKey`/`Subscribe`
are tenant-scoped usecases (`tenant.RequireTenantID`, since each tenant has
its own VAPID keypair) — but `mountPushRoutes` deliberately mounts
`/api/vapid-public-key`/`/api/push-subscribe` OUTSIDE `authMiddleware`
(BUG-003's regression guard: a browser must be able to fetch the VAPID key
before/without a session). That guard is correct for a truly anonymous
caller, but it meant NO caller — including a fully logged-in browser whose
same-origin `fetch()` sends the session cookie — ever got a tenant
attached, since nothing on that unauthenticated mount ever reads the cookie
at all.

Fixed with a soft-auth fallback, not by moving these routes into the authed
group (which would reintroduce BUG-003): new `resolveSoftIdentity(r,
cookieValidator)` tries `identityFromContext` first (already-authed
`/v1/notifications/*` mount), then falls back to validating the
`orca_session` cookie directly if one is present — but NEVER fails the
request when it isn't (an anonymous caller still gets an empty identity,
exactly as before). `mountPushRoutes`/`handleGetVapidPublicKey`/
`handleSubscribe` now take a `CookieSessionValidator` (nil for the authed
mount, `deps.CookieValidator` for the public one).

4 new Go tests (2 soft-auth cases + the 2 pre-existing suites still green,
including `TestPushRoutes_NoAuthRequired`, unmodified — proving BUG-003's
guarantee survives this fix).

## Twelfth: devServer.browseDir never existed — every folder browse failed

User reported three related failures in the post-onboarding "Add a
project" flow: Browse folder does nothing, Clone from URL's parent folder
can't be browsed, Create a new project's "Choose parent folder..." doesn't
work. All three ultimately render `RemoteFileBrowser` with a `devServerId`,
which calls `window.api.devServer.browseDir` → `devServer.browseDir` — a
channel that **never existed on the backend at all** (confirmed via a
repo-wide grep: zero matches for `browseDir`/`BrowseDir` anywhere in
backend-go). Every browse attempt hit the generic "not yet implemented"
error.

Fixed without any new infra-fleet-service RPC, same `devServerId →
connectionId → Relay` pattern as `onboarding.detectAgents`/`accounts.*`:
new `devServer.browseDir` wscompat channel resolves the connection, then
relays to the agent's own confirmed `fs.readDir` RPC
(`specs/agent/api/agent-rpc-catalog-git-fs.md`, `depth: 1`), mapping its
`{path, entries:[{path,name,type}]}` shape into the
`{resolvedPath, entries:[{name,isDirectory,isSymlink}]}` shape
`web-preload-api.ts`/`RemoteFileBrowser` already expect — the same shape
desktop's own local `files.browseServerDir` returns.

**Honestly disclosed gap, not silently faked**: `fs.readDir` does no `~`
(home directory) expansion — it's pure Node `fs` calls, no shell involved —
and no dev-server-agent RPC reports the remote user's home directory today.
An incoming `""`/`"~"` (RemoteFileBrowser's default `initialPath`) is
defaulted to `/` (always valid) rather than a guessed home; the user can
navigate from there. Revisit if/when the agent gains a
home-directory-reporting RPC.

4 new Go tests (`TestDevServerBrowseDirChannel_RequiresDevServerID`,
`_NotConnectedErrors`, `_DefaultsTildeToRoot`, `_RelaysAndMapsEntries`).
Frontend-side prop-threading fixes (`devServerId` was simply missing from
several sibling components' disabled conditions/branches — `AddRepoCloneStep.tsx`,
`AddRepoCreateStep.tsx`, `CreateProjectLocationField.tsx`,
`AddRepoServerStartStep.tsx`) are documented in frontend's
solutions/README.md.

## Thirteenth: onboarding state never persisted — refresh always re-onboarded

User reported: after finishing onboarding, any page refresh re-shows the
wizard from scratch. This was a KNOWN, deliberately-documented gap this
whole session — `channels_onboarding.go`'s original doc comment said so
outright: "backend-go has no per-user preference/UI-state persistence layer
... the state does not survive a reload." `onboarding.update` only ever
echoed the caller's own partial update back in memory, for the life of one
request; `onboarding.markChecklistItem` didn't persist at all.

Fixed with real per-user storage: `tenant.user_profiles` (already the
per-user, one-row-per-user_id table this service owns) gained a new
`onboarding_state_json JSONB` column (migration 0003) — a dedicated column
rather than folded into the existing `settings_json` (that column is
documented as company/department/team cascading DEFAULTS, a different
concept from per-user wizard progress). New `UserProfileRepository.
GetOnboardingState`/`SetOnboardingState` methods do a targeted partial
UPDATE (not routed through the existing `Upsert`, which fully replaces
`department_id`/`settings_json` every call and would have silently
clobbered them). New `GetOnboardingState`/`SetOnboardingState` usecases
(company scoped from `tenant.RequireTenantID(ctx)`, same convention as
`SetUserDepartment` — neither proto request carries `company_id`) and
`TenantService.GetOnboardingState`/`SetOnboardingState` RPCs.

`channels_onboarding.go`'s three handlers rewritten to actually persist:
`onboarding.get` now loads the caller's saved state (falling back to the
existing "wizard not started" defaults on any failure — persistence is
fail-open, never blocks the UI); `onboarding.update` merges the incoming
partial update ONTO the persisted state (not bare defaults) and saves the
result; `onboarding.markChecklistItem` now actually sets the one checklist
field and saves, instead of just acknowledging.

10 new/updated Go tests across `onboarding_state_test.go` (tenant-service
usecase round trips) and `channels_test.go` (`TestOnboardingGetChannel_
NoSavedStateReturnsDefaults`, `_UpdateChannel_PersistsAcrossGet`,
`_ScopesStateByUser`, `_MarkChecklistItemChannel_
PersistsWithoutClobberingOtherFields`). All 17 services build/test clean.

## Fourteenth: "connected" was never real, and browseDir used the wrong concept of "connected"

User asked directly to investigate why a just-selected onboarding dev
server showed disconnected and why folder browsing still didn't work end
to end. Root cause, found via `Client.Health`'s doc comment and
`ResolveConnection`'s SQL: this codebase has TWO unrelated concepts both
loosely called "connection":

1. **infra.connections** (`ConnectionResolver.ResolveConnection`/
   `ResolveConnectionByDevServer`) — a persistent DB row binding a repo/
   worktree to a dev server, created only by `CreateConnection` once a
   project is actually added. "Connected" here means "a routing row
   exists", nothing about the agent's live socket.
2. **The agent's actual live session** (`devserveragent.Client`'s
   in-memory `sessions` map, `session.isHandshaked()`) — whether the
   dev-server-agent's WebSocket is genuinely up right now.

Two real bugs fell out of conflating these:

- **`toDevServerView`'s `Status` field was a hardcoded `"disconnected"`
  string**, literally always, regardless of the agent's real state — its
  own doc comment said so outright ("backend-go doesn't track live relay
  connection state yet"). Every dev server in `devServer.list`/
  `listForUser` showed disconnected forever, even genuinely-live ones.
- **`devServer.browseDir`/`onboarding.detectAgents` (this session's own
  earlier fixes) used `ResolveConnection` as a "is the agent live" proxy**
  — wrong for exactly their own use case: browsing/detecting happens
  BEFORE the user has picked/created a project, i.e. before any
  `infra.connections` row can exist. A freshly-connected, genuinely-live
  dev server always failed both with a false "not connected".

Fixed with a new, correct primitive: `devserveragent.Client.IsConnected
(devServerID) bool` — a pure peek at the live session map, unlike `Health`
(which actively dials/provisions — right for its own EstablishConnection
call site, wrong to call in bulk over a list). Built on it:

- `usecase.IsDevServerConnected` (+ `IsDevServerConnected` RPC) — real
  live-status answer, used by wscompat's new `attachConnectionStatus`
  helper to overwrite `toDevServerView`'s placeholder in `devServer.list`/
  `listForUser` (fails open to "disconnected" on any RPC hiccup — a status
  check must never break the whole list).
- `usecase.RelayByDevServer` (+ `RelayByDevServer` RPC) — `Relay`'s
  devServerId-keyed sibling, resolving straight from
  `DevServerRepository.Get` (tenant ownership only) instead of
  `infra.connections` — bypasses the chicken-and-egg gap entirely.
  `devServer.browseDir`/`onboarding.detectAgents` now use this instead of
  `ResolveConnection`+`Relay`.

10 new/updated Go tests: `TestClientIsConnected_*` (2, devserveragent),
`TestRelayByDevServer_*` (6, infra-fleet-service usecase),
`TestIsDevServerConnected_*` (4, infra-fleet-service usecase),
`TestDevServerListChannel_ReportsRealConnectionStatus`/
`_StatusCheckErrorFailsOpenToDisconnected` (wscompat), plus all
`devServer.browseDir`/`onboarding.detectAgents` tests rewritten against
`RelayByDevServer` instead of `ResolveConnection`+`Relay`. All services
build/test clean.

## Fifteenth: the actual reason — a NULL-scan bug silently keyed every session wrong

User asked to use `deploy/agent`/`agent/` to find out why the (already
fixed, real) live-status check still showed every dev server disconnected.
Live investigation (`deploy/agent/scripts/deploy-agents.sh --status`/SSH):
all 3 systemd-managed agents were genuinely UP, handshaked, and
continuously exchanging keepalives for hours (`agent-dev-01.log`:
`Handshake OK: sessionId=...` then unbroken `readyState=1` keepalive frames
every 5s) — the agent side was never the problem.

Root cause, found in infra-fleet-service's own container logs at the exact
moment each agent re-minted its token (every systemd restart, e.g. from
this session's own repeated redeploys severing the WS connection):

```
resolving dev_servers row failed — agent will connect but stay invisible
to the Admin Console: INFRA_DEV_SERVER_RESOLVE_FAILED: ... postgres: find
dev server by host and mode: can't scan into dest[4] (col: ssh_target_id):
cannot scan NULL into *string
```

`FindByHostAndMode` (added earlier this session for
`ResolveDirectWebSocketDevServer`) scanned `ssh_target_id` directly into
`domain.DevServer.SSHTargetID` (a plain `string` field) — but
`ssh_target_id` is legitimately NULL for every direct-websocket dev server
(that column only applies to relay-ssh mode), which pgx cannot scan into a
non-nullable string destination. `Get`/`List` in the same file already used
the correct pattern (scan into a local `*string`, then conditionally
assign) — `FindByHostAndMode` just never got it.

The consequence, per `TokenIssuer.handlePost`'s own documented fallback: a
resolver failure "must not break token issuance" and falls back to using
the caller's RAW EXTERNAL STRING (e.g. `"dev-01"`) as the `Registry`/
`AttachInboundSession` slot key — instead of `domain.DevServer.ID` (the
real UUID `ListDevServers`/`IsDevServerConnected` look sessions up by).
Every agent token mint hit this error, so every live session got attached
under a string key ("dev-01"/"dev-ai"/"test-01") that no UUID-keyed lookup
could ever find — `IsDevServerConnected` (this session's own earlier fix)
was answering correctly against its map; the map had just never held the
right key, for as long as `ResolveDirectWebSocketDevServer`/
`FindByHostAndMode` have existed.

Fixed by scanning `ssh_target_id` through a nullable local `*string`,
matching `Get`/`List`'s existing pattern exactly.

This package (`infra-fleet-service/internal/adapter/postgres`) had ZERO
integration tests and didn't even compile under `-tags=integration`
(several pre-existing sibling test files referenced shared fixture
IDs/helpers — `testTenant1/2`, `testDevServer1/2`, `testUnknownID`,
`setupSshTargetStore` — that were never actually defined anywhere, a
pre-existing gap unrelated to this fix). Added the missing fixtures/helper
plus a real, purpose-built `repository_test.go` covering exactly this
regression against a genuine Postgres container (testcontainers-go):
`TestFindByHostAndMode_DirectWebSocketWithNullSSHTargetID` (registers a
direct-websocket row with an empty SSHTargetID — Register's own
`NULLIF($5,'')::uuid` already persists that as real SQL NULL — then asserts
the lookup succeeds and returns the SAME row/UUID) and
`TestFindByHostAndMode_NoMatchReturnsNotFound`. Both pass; the whole
package's previously-uncompilable integration suite (6 pre-existing tests)
now compiles and passes too, as a side effect of fixing the missing shared
fixtures. `go build`/`go test ./...` (unit, no Docker needed) clean.

## Sixteenth: AI Provider Accounts inherited the same wrong "connected" concept

User reported Settings → AI Provider Accounts → pick a genuinely-connected
dev server → "This dev server is not currently connected. Accounts cannot
be managed until it reconnects." Same root cause as the Fourteenth fix,
independently present in `channels_accounts.go`: `accounts.resolveDevServerConnection`
called `ResolveConnection` (an `infra.connections` DB-row check — "does a
routing row exist for this connectionId", nothing to do with the agent's
live socket), and `accounts.selectClaude`/`selectCodex`/`removeClaude`/
`removeCodex`/`subscribe` all called `Relay` keyed by that same
`infra.connections`-table id. AI Provider Accounts management has no
project/repo in the picture at all — it operates directly on a dev server —
so no `infra.connections` row was ever going to exist for it, and the
check failed closed every time regardless of the agent's real state.

Fixed the same way as the Fourteenth fix, applied to this channel set:

- `accounts.resolveDevServerConnection` now calls `IsDevServerConnected`
  and echoes the input `devServerId` straight back as `ConnectionID` in its
  response, instead of resolving a separate connections-table id.
- `accounts.selectClaude`/`selectCodex`/`removeClaude`/`removeCodex`/
  `subscribe`'s poll now all call `RelayByDevServer` keyed by that same
  devServerId, instead of `Relay` keyed by a connections-table id.

Deliberately kept the wire JSON field name `connectionId` unchanged on both
the request args and the `resolveDevServerConnection` response — the
frontend (`accounts-dev-server-connection.ts`) already treats this as an
opaque token passed straight through from
`resolveDevServerConnection`'s result into every subsequent
`accounts.*` call, so it required zero frontend changes; only what the
token *means* changed (devServerId instead of a connections-table id).

10 Go tests in `channels_accounts_test.go` rewritten against
`RelayByDevServer`/`IsDevServerConnected` (previously `Relay`/
`ResolveConnection`) — including
`TestAccountsResolveDevServerConnection_Connected`, whose assertion now
expects the echoed-back devServerId (`"ds-1"`) rather than a
separately-resolved connections-table id. `go build`/`go test` clean
across api-gateway, infra-fleet-service, and tenant-service.

## Seventeenth: SpawnTerminalSession never accepted a devServerId either — the same gap Relay had, one usecase over

Live-verified after api-gateway started passing a real devServerId as
`connectionId` into `terminal.create` (frontend fix, see this session's
Twentieth frontend entry): the "Install CLI & Skills" terminal reached the
backend but immediately failed with
`INFRA_CONNECTION_NOT_FOUND: no dev server owns this connectionId`.

Root cause, same architectural family as the Fourteenth/Sixteenth fixes:
`SpawnTerminalSession.Execute` resolves `ConnectionID` through
`ConnectionResolver.ResolveConnection` ONLY — the `infra.connections`
DB-row check — with no fallback for a caller that has no project/repo yet
(exactly `RelayByDevServer`'s own "chicken-and-egg" gap, just never closed
for terminal creation specifically). A pre-project ephemeral terminal has
no connections row to resolve, so `ResolveConnection` correctly reports
`connected=false`, and the usecase gave up with `INFRA_CONNECTION_NOT_FOUND`
even though the string it received was a perfectly valid, live, connected
devServerId.

Fixed by adding the exact fallback `RelayByDevServer` already established:
when `ResolveConnection` reports not-connected, try `in.ConnectionID`
directly as a devServerId via `DevServerRepository.Get` (tenant ownership)
+ `DevServerAgentClient.IsConnected` (live-session check) before failing.
Deliberately did NOT add a new proto field for this — the frontend had
already shipped sending the devServerId as `connectionId` (the same
wire-field-repurposing convention as the Sixteenth fix's
`accounts.resolveDevServerConnection`), so this stays a backend-only
change requiring no new deploy coordination with the frontend.

`SpawnTerminalSession` gained a `DevServerRepository` dependency
(`NewSpawnTerminalSession`'s signature grew one param); wired from
`main.go`'s already-shared `repo` value, which already satisfies every
other `*Repository` port this service uses (`RelayByDevServer`/
`IsDevServerConnected` are wired from the exact same `repo` value).

4 new Go tests: `TestSpawnTerminalSession_ConnectionIDIsActuallyADevServerID_SpawnsAndPersists`
(the core regression — a connected devServerId with no connections row
still spawns), `TestSpawnTerminalSession_ConnectionIDIsADevServerIDButNotConnected_ReturnsFailedPrecondition`,
plus the existing 2 unresolved/resolved-connection tests updated for the
new constructor signature. `go build`/`go test` clean across
infra-fleet-service and api-gateway; `gofmt`/`go vet` clean (also fixed a
pre-existing, unrelated `gofmt` drift in `register_dev_server_test.go`
found along the way).

## Eighteenth: the fallback needed to reach every OTHER terminal usecase too, plus a stale FK constraint

Live-verified twice more after the Seventeenth fix deployed: (1) spawning
itself now succeeded in resolving the devServerId, but persisting the new
session row failed with `INFRA_CREATE_TERMINAL_SESSION_FAILED` — a real
pty had already been spawned on the agent by that point; (2) even once
that's fixed, resize/kill/stop/wait/focus/agentStatus/inspectProcess would
all still break on the very next RPC against that same session, because
they share `resolveTerminalSession` (`terminal_session_lookup.go`), which
had the exact same `ConnectionResolver`-only limitation `SpawnTerminalSession`
had before the Seventeenth fix — just one lookup further downstream.

Root cause 1 (persist failure): `infra.terminal_sessions.connection_id`
carried `REFERENCES infra.connections(id)` — a real FK — but the
Seventeenth fix now legitimately stores a devServerId there for these
sessions, which is never a row in `infra.connections`, so every INSERT hit
a foreign-key violation right after the agent-side spawn had already
succeeded. Fixed with migration `0010`: drops the FK (Postgres has no
single-column "FK to one of two tables" constraint) — referential
integrity for the real-connection case still holds at the application
layer via `ConnectionResolver`'s own existence check.

Root cause 2 (every follow-up RPC): applied the identical
`ResolveConnection`-then-devServerId-fallback pattern to
`resolveTerminalSession` itself (added a `DevServerRepository` param) —
fixing it ONCE here, rather than in each of the 7 callers individually,
since they all route through this one shared lookup. Mechanically updated
all 7 constructors (`NewResizeTerminalSession`/`NewKillTerminalSession`/
`NewStopTerminalProcess`/`NewWaitTerminalSession`/
`NewGetTerminalAgentStatus`/`NewInspectTerminalProcess`/`NewAttachPty`) to
accept and forward a `DevServerRepository`, wired from `main.go`'s already-
shared `repo` value (same value already backing `SpawnTerminalSession`/
`RelayByDevServer`/`IsDevServerConnected`).

4 new Go tests: `TestResolveTerminalSession_ConnectionIDIsActuallyADevServerID`/
`_NeitherConnectionNorDevServerFound_ReturnsNotFound` (the shared lookup's
own regression coverage — no dedicated test file existed for
`resolveTerminalSession` before this), plus the 3 pre-existing
`attach_pty_test.go`/`wait_terminal_session_test.go`/
`stop_terminal_process_test.go` files updated for the new constructor
signature (`kill`/`resize`/`get_terminal_agent_status`/
`inspect_terminal_process` had no dedicated test files before or after —
a pre-existing gap, not introduced here). `go build`/`go vet`/`go test`
clean across every one of this workspace's 19 modules (not just the two
touched services — verified the fix didn't ripple anywhere unexpected);
`gofmt` clean (also fixed a pre-existing, unrelated drift in
`register_dev_server_test.go` found along the way).

## Nineteenth: terminal.create's ack shape never matched what the frontend actually reads

Live-verified through a real browser after the Eighteenth fix deployed:
the backend RPC chain now genuinely succeeds end-to-end (confirmed via the
page's own WebSocket traffic — `terminal.create` returns `ok:true` with a
real `ptyId`, and the agent streams back real shell output, decoded to
`ubuntu@tas-test:~$`), but the terminal pane itself crashed the instant
the pty connected: `Cannot read properties of undefined (reading
'handle')`, blank pane, no prompt ever rendered, install command never
auto-pasted.

Root cause: `remote-runtime-pty-transport.ts` (and
`launch-agent-background-session.ts`) — both written against
`RuntimeTerminalCreate`, the legacy TypeScript `backend/`'s runtime RPC
contract — read `created.terminal.handle` off `terminal.create`'s ack.
backend-go's actual ack (`toTerminalSessionView`) is a flat
`{ptyId, connectionId, cwd, createdAt, lastActiveAt}` — no `terminal` key,
no `handle` field, at all. This is a genuine, pre-existing wire-contract
mismatch between backend-go's wscompat implementation and the frontend
code that talks to it — every web-deployed terminal pane using this
transport (not just the ephemeral onboarding one) would have crashed here
the instant its `terminal.create` RPC actually succeeded; it simply never
had before, because every earlier bug in this fix chain (host-local
rejection, connection-not-found, FK violation) failed the RPC itself
first, before the frontend ever got far enough to read this field.

Fixed with a compatibility wrapper, `terminalCreateResultView` — distinct
from `terminal.list`'s bare `[]terminalSessionView` — nesting the existing
session view under a `terminal` key and adding `Handle` (echoing `PtyID`,
since backend-go has no separate "handle" concept). Deliberately scoped to
`terminal.create`'s own ack only — `terminal.list`, `terminal.send`, and
the other `terminal.*` channels already use `ptyId`-keyed shapes those
callers correctly expect, per a read-through of every remaining
`terminal.*` handler and the corresponding frontend call sites; the
binary `terminal.multiplex` protocol (channels_terminal_multiplex.go) is
explicitly documented as built to match "the unmodified frontend" wire
format exactly and was left untouched.

Updated 2 pre-existing tests whose assertions unmarshaled the OLD flat
shape (`TestTerminalCreateChannel_SpawnsAndOpensAttachPtyStream`,
`TestTerminalCreateChannel_EndToEndPushInterleavesWithConcurrentSend`) to
the new wrapped shape, plus a new assertion that `Terminal.Handle` echoes
`PtyID` — the exact field whose absence caused the live crash. `go build`/
`go vet`/`go test` clean for api-gateway; `gofmt` clean for every file this
fix touched (3 unrelated pre-existing `gofmt` drift files found in the
same package, left alone — not introduced by this change).

## Twentieth: terminal.multiplex's Subscribe frame silently dropped every real frontend request — wrong JSON key

Live-verified once more after the Nineteenth fix deployed: the
`Cannot read properties of undefined (reading 'handle')` crash was gone,
but the terminal pane stayed permanently blank, with a new error banner:
`channel "terminal.subscribe" is not yet implemented in backend-go`.

Root cause was actually TWO compounding issues, both in how the frontend's
"web session" (`'session-auth'`) terminals stream PTY output:

1. `remote-runtime-pty-transport.ts` routed every `'session-auth'`
   terminal to `subscribeTerminalViaJson` (a plain-JSON RPC fallback)
   instead of the binary `terminal.multiplex` protocol every other
   environment id uses — on the stated assumption that "the web session
   client's Unix-socket-proxied transport has no binary-frame capability
   at any layer". That's true of the OLD TypeScript backend's web
   architecture the comment describes, but false for backend-go: its
   `WebSessionClient` genuinely implements `sendBinary`/`onBinary` over
   the same `/ws` connection, and backend-go has no `terminal.subscribe`/
   `terminal.unsubscribe` channels at all — every `'session-auth'`
   terminal was guaranteed to hit "not yet implemented", forever.
2. Once switched to `terminal.multiplex`, the Subscribe control frame
   itself was ALSO silently dropped: `channels_terminal_multiplex.go`'s
   `terminalMultiplexSubscribePayload` decoded the pty identifier under a
   `json:"ptyId"` tag, reasoning (in its own doc comment) that since
   backend-go treats the agent's ptyId as the "handle" everywhere else,
   the wire field should be renamed to match. But the real, unmodified
   frontend (`remote-runtime-terminal-multiplexer.ts`, read-only
   reference) encodes the JSON key as `"terminal"` — the value carries a
   ptyId, but the KEY was never going to change just because the meaning
   did. Every Subscribe frame decoded to an empty PtyID and was quietly
   ignored (`payload.PtyID == ""`) — no error, just nothing happening,
   which is exactly what was observed.

Fixed both: switched the frontend's transport-selection branch to always
use the multiplexer (confirmed via `impact()`, LOW risk, single call
site — `subscribeTerminalViaJson` itself is untouched, just no longer
called from here, kept for a genuine future no-binary target); corrected
the Go struct's tag to `json:"terminal"`.

The existing multiplex test suite couldn't have caught bug 2: every test
built its Subscribe payload via `json.Marshal(terminalMultiplexSubscribePayload{...})`
— circular, since it round-trips through the same struct's own tag
regardless of what that tag is. Added
`TestTerminalMultiplexChannel_SubscribeRecognizesTheRealFrontendWireKey`,
which hand-writes the raw JSON exactly as the real frontend encodes it
(`{"streamId":7,"terminal":"pty-1",...}`) — the only test in the file that
can actually fail if this regresses. All 8 multiplex tests (7 pre-existing
+ 1 new) pass; `go build`/`go vet`/`go test` clean.

## Twenty-first: the Twentieth fix's premise was wrong — WebSessionClient cannot carry binary at all

Live-verified the Twentieth fix via a script-based check (a headless
Playwright session driven by code, reading DOM text and WS frames directly —
no screenshots) rather than another visual pass, per explicit instruction to
prefer code/script checks over image analysis. The direct WS-frame capture
showed: `terminal.create` succeeds, the real prompt bytes arrive once, but
`terminal.multiplex`'s Subscribe control frame is a binary frame — and ZERO
binary frames ever crossed the wire. Reading `web-session-client.ts` (not
guessing) found why: `handleSocketMessage`'s `if (typeof rawData !== 'string')
return` drops every binary WS message outright, and `subscribe()`'s returned
`sendBinary` unconditionally throws (`'Binary frames not supported in
session mode over this channel'`). The TS interface declares `sendBinary`/
`onBinary` on `WebSessionClient` (satisfying
`RemoteRuntimeMultiplexedTerminalCallbacks` structurally, which is what the
Twentieth fix's reasoning leaned on) — the implementation stubs them. The
ORIGINAL comment the Twentieth fix removed ("the web session client... has
no binary-frame capability at any layer") was correct after all.

Reverted the Twentieth fix's transport-selection change: `'session-auth'`
routes to `subscribeTerminalViaJson` again (the plain-JSON fallback);
`terminal.multiplex` stays for `WebRuntimeClient` (paired/E2EE), which
genuinely does support binary. The Twentieth fix's OTHER half — correcting
`terminal.multiplex`'s Subscribe payload key from `"ptyId"` to `"terminal"`
— stays correct and valuable for that real consumer, just not what
`'session-auth'` needed.

The actual missing piece: backend-go had no `terminal.subscribe`/
`terminal.unsubscribe` channels at all (the JSON fallback's own contract),
and every OTHER plain `terminal.*` channel (`send`, `close`, `focus`,
`agentStatus`, `isRunningAgent`, `inspectProcess`, `wait`) had the IDENTICAL
`"ptyId"`-vs-`"terminal"` key mismatch the multiplex Subscribe frame had —
confirmed field-by-field via the same live WS capture, not assumed: the
real frontend uniformly sends `{terminal: <ptyId>, ...}` for every one of
these, and `terminal.send` additionally sends `text`, not `data`. Every one
of these channels' existing tests were "circular" in the exact way the
multiplex Subscribe test was (`argsJSON(t, terminalSendArgs{...})` — always
round-trips through the same struct's own tag) — none could have caught
this. `terminal.stop`'s real call site sends `{worktree: ...}` instead
(a deeper, separate worktree-resolution gap, left alone/tracked, not
addressed by this rename).

Fixed: renamed `terminalSendArgs`/`terminalPtyIDArg`/`terminalResizeArgs`/
`terminalWaitArgs`'s JSON tags to match reality (`terminal`, `text`); added
`channels_terminal_subscribe.go` implementing `terminal.subscribe` (a
`StreamChannelHandler` opening its own AttachPty stream per subscribe call —
same one-stream-per-subscriber pattern `channels_terminal_multiplex.go`
already established — acking `{"type":"subscribed"}`, no scrollback, same
accepted degradation as multiplex's own SnapshotRequest no-op; streaming
`{"type":"data","chunk":<plain string, not base64>}`; relying on
`pipePushForDialect`'s existing "channel close → auto {"type":"end"}"
behavior for exit), `terminal.unsubscribe` (a new per-connection
`terminalJsonSubscribeRegistry` keyed by `"<ptyId>:<clientId>"`, mirroring
`terminalStreamRegistry`'s per-connection-not-shared rationale), and
`terminal.updateViewport` (the fallback's actual resize mechanism per
`remote-runtime-terminal-json-subscribe.ts`'s own doc comment — same
`ResizeTerminalSession` RPC `terminal.resize` already wraps, different arg
shape).

9 new/updated Go tests: `TestTerminalSendChannel_RecognizesTheRealFrontendWireKeys`
(hand-written raw JSON, the same non-circular pattern the multiplex fix's
regression test established) plus 6 new tests in
`channels_terminal_subscribe_test.go` covering subscribe's ack/data/exit,
unsubscribe's cancellation (checked via the registry's own synchronous
state, not by waiting on the events channel to close — a fake `PtyStream`'s
`Recv()` has no context awareness, so `cancel()` alone never unblocks it in
a test the way a real gRPC stream does in production; this hung the test
suite once during development before being corrected, per
`TestTerminalMultiplexChannel_UnsubscribeStopsForwardingFurtherInput`'s own
established pattern of checking synchronous teardown state instead), and
`terminal.updateViewport`'s resize call + zero-viewport no-op. `go build`/
`go vet`/`go test` clean.

## Twenty-second: Automation page — `AUTOMATION_LIST_RUNS_FAILED`, then a second bug it was masking

User report: clicking "Automation" in the left menu threw
`Unhandled Promise Rejection: RuntimeRpcCallError: rpc error: code = Internal
desc = AUTOMATION_LIST_RUNS_FAILED: failed to list automation runs`. Traced
(not guessed) to `automation-service`'s `ListRuns` usecase wrapping
`AutomationRunRepository.ListByAutomation`'s Postgres query:
`WHERE tenant_id = $1 AND automation_id = $2 AND id > $3`, both
`automation_id` and `id` being UUID columns. `AutomationsPage.tsx`'s
page-mount `refresh()` calls `listAutomationRunsForTarget(target)` with NO
`automationId` — the "all runs, no automation selected yet" initial load —
which threads an empty string straight into `$2`/`$3` and hard-fails with
Postgres's `invalid input syntax for type uuid: ""` before the query's own
`WHERE` logic ever runs (parameter type coercion happens at bind time, so
no `CASE`/`OR` guard inside the query can rescue an already-invalid bind).

Fixed with this codebase's own established idiom for exactly this shape —
`AutomationRepository.List` already guards its `pageToken` this way:
`WHERE tenant_id = $1 AND ($2 = '' OR automation_id = $2::uuid) AND ($3 = ''
OR id > $3::uuid)`, applied to both the automationID and pageToken filters
in `ListByAutomation` (`repository.go`). Impact analysis
(`impact({target:"list_runs.go", direction:"upstream"})`) came back MEDIUM,
5 direct files (main.go, ticker.go, repository.go, workflow_client.go,
grpc/server.go) — reviewed, none needed changes beyond the repository
method itself. New integration test (real Postgres via testcontainers,
`-tags=integration`):
`TestAutomationRunRepository_ListByAutomation_EmptyAutomationIDListsAllTenantRuns`
— asserts an empty `automationID` lists every tenant run across automations
(not just one), and that a real `automationID` still scopes correctly.

Live-verifying this fix (script-based, no screenshots) revealed a SECOND,
independent bug it had been masking: `AutomationsPage.tsx`'s `refresh()` has
no `catch` — so as long as the runs call rejected, the whole
`Promise.all` rejection surfaced directly as the reported unhandled
rejection, and nothing past it ever ran. Once the first bug was fixed and
the promise resolved, a NEW crash appeared: `TypeError: Cannot read
properties of undefined (reading 'some')` on `nextAutomations.some(...)`.
Root cause: `automation.list`/`automation.runs`'s wscompat channel handlers
`return resp, nil` — the whole `*ListAutomationsResponse`/
`*ListRunsResponse` proto message (needed for `nextPageToken`, not just the
list) — and `Dispatch`'s existing nil-slice normalizer (added for
`specs/backend-go/bugs/missing-v2/BUG-005`) deliberately never reaches into
a `proto.Message` value, so a genuinely-empty tenant (this admin account
has zero automations) got a response with no `automations`/`runs` key at
all rather than `[]`. Full root cause, fix (two small local view structs,
`automationsListView`/`automationRunsListView`), and tests are documented
in `specs/backend-go/bugs/missing-v2/BUG-005-...md`'s new addendum — kept
there rather than duplicated here since it's squarely that bug's own
tracked class, not a dev-server-access-control concern.

Frontend defense-in-depth: `listAutomationsForTarget`/
`listAutomationRunsForTarget` (`automation-host-client.ts`) now also return
`result.automations ?? []`/`result.runs ?? []` — redundant with the backend
fix for this specific pair of channels, but cheap insurance against any
other RPC response shape (or a future regression) omitting the key again;
2 new tests assert the fallback. Deployed as v0.4.30 (backend query fix)
→ v0.4.31 (frontend fallback) → v0.4.32 (backend view-struct fix); live-
verified clean after each of the last two via a direct Playwright script
(DOM text + WS frames, no screenshots) — no `AUTOMATION_LIST_RUNS_FAILED`,
no unhandled rejection, "Automations" page renders its template list and
"Runs 0" normally for this zero-data tenant.

## Twenty-third: "New Project" submit — `PROJECT_MEMBERSHIP_LOOKUP_FAILED`, a missing creator-membership call, and a `project.get` wire-key mismatch

User report: `Project Workspace (Beta)` → "New Project" → Submit threw
`Uncaught (in promise) RuntimeRpcCallError: rpc error: code = Internal desc
= PROJECT_MEMBERSHIP_LOOKUP_FAILED: failed to resolve caller's project
membership`. Two real, independent bugs, both in the SAME immediate
"create → auto-switch-to-it" call sequence:

1. **`project.get` wire-key mismatch.** `WorkspaceContext.tsx`'s
   `switchProject` (the ONLY real caller of this channel) sends
   `{projectId: "<uuid>"}`; `channels_tenant_project.go`'s `project.get`
   handler decoded a `getArgs{ID string \`json:"id"\`}` struct — the
   unmatched `projectId` key was silently ignored by `json.Unmarshal`
   (`decodeArg`, `registry.go`), so `in.ID` was always `""`. That empty
   string reached `GetMembership`'s `WHERE project_id = $1` against a
   UUID column — a Postgres bind-time `invalid input syntax for type uuid`
   error, wrapped as `PROJECT_MEMBERSHIP_LOOKUP_FAILED` by
   `requireProjectAccess` (`authorization.go:68`). The existing
   `TestProjectGetChannel_Success` test was circular — its args used the
   SAME wrong `"id"` key the handler happened to decode, so it could never
   have caught this (same class of gap as this session's earlier
   terminal.* wire-key fixes). Fixed the json tag to `"projectId"`; fixed
   the test's args to match the real caller.

2. **Creator never became a project member.** `CreateProject.Execute`'s
   own doc comment says the creator "becomes an implicit owner via a
   follow-up AddMember call by the caller (api-gateway)" — that follow-up
   call never existed anywhere (checked both api-gateway's `project.create`
   wscompat handler and project-service's own gRPC `CreateProject` server
   method). `requireProjectAccess` grants nothing from `Project.CreatedBy`
   alone — only a real `project_members` row — so even after fixing #1,
   the creator's own immediate `GetProject` call (from `switchProject`,
   fired right after create) would have failed closed with
   `PROJECT_NOT_AUTHORIZED` instead. Fixed by calling `client.AddMember`
   (role `PROJECT_ROLE_OWNER`) right after `CreateProject` succeeds, inside
   the `project.create` wscompat handler — matching where the doc comment
   always said this call belonged. Fails closed: if `AddMember` errors
   after the project row is already persisted, `project.create` still
   returns an error rather than silently leaving an inaccessible project.

New tests: `TestProjectCreateChannel_Success` (updated to assert the
AddMember call's project/user/role), `TestProjectCreateChannel_
AddMemberFailure_PropagatesError`, `TestProjectGetChannel_Success` (fixed
to send `projectId`). `go build`/`go test ./...` clean for `api-gateway`.

Frontend: `ProjectSwitcher.tsx`'s `onCreated` did `void switchProject(created.id)`
with no `.catch` — any rejection (this bug, or a future one) surfaced as a
raw unhandled promise rejection, exactly the reported symptom. Added
`.catch(() => toast.error(...))` — the project itself is already created
successfully at that point, so a switch failure is a "couldn't open it yet"
toast, not a creation failure. 2 new tests in `ProjectSwitcher.test.tsx`.

Deployed as v0.4.33; live-verified via a direct Playwright script (form
fill + submit, DOM text + WS frames, no screenshots) — `PROJECT_MEMBERSHIP_
LOOKUP_FAILED` gone.

## Twenty-fourth: a pre-existing infinite-render bug (`useGit.ts`), unmasked by the Twenty-third fix

Live-verifying the Twenty-third fix surfaced a THIRD, independent bug: a
minified React error #185 ("Maximum update depth exceeded"), contained by
an error boundary (`page.workspace`), right after switching to the freshly
created project. Root cause (confirmed by reading zustand v5's React
binding and React-DOM 19.2.5's actual `updateStoreInstance`/
`checkIfSnapshotChanged` source, not guessed): `useGit.ts`'s
`useAppStore((s) => ({stagedFiles: ..., unstagedFiles: ..., isPushing: ...,
isCommitting: ...}))` selector returns a fresh object literal on every
call with no `useShallow`. Zustand v5's React binding hands that straight
to React's own `useSyncExternalStore` with **no built-in memoization** (v5
dropped the `useSyncExternalStoreWithSelector` shim earlier zustand
versions used) — so the post-render snapshot re-check via `Object.is`
always reports "changed," forcing a re-render, forever. A repo-wide grep
confirmed this was the ONLY unguarded object-selector call site among 65+
selector usages in the whole frontend — every other one already wraps
with `useShallow`.

This bug is NOT actually conditioned on an empty/fresh project — it fires
for ANY project the moment `GitPanel`'s default "Changes" tab mounts
(`StagingArea`/`CommitForm` both call `useGit()`). It only surfaced now
because `switchProject`/`project.get` had never once succeeded for this
flow before the Twenty-third fix — this is simply the first time this
render tree ever executed end-to-end, live-verified or otherwise.

Fixed: wrapped the selector in `useShallow` (`zustand/react/shallow`),
matching the pattern every other selector in this codebase already uses.
2 new tests in `useGit.selector-stability.test.tsx` — one asserts the
UNGUARDED shape of this exact selector genuinely throws React's own
"Maximum update depth exceeded" (a real repro, not just documentation),
the other asserts the `useShallow`-wrapped version mounts cleanly;
exercises the REAL zustand store (not the heavily-mocked one
`useGit.test.ts` uses, which bypasses `useSyncExternalStore`'s snapshot
check entirely and could never have caught this). `tsc --noEmit` clean for
`useGit.ts`; full `hooks`/`components/workspace` test directories re-run
clean (one pre-existing, unrelated failing file confirmed via zero git
diff, `useIpcEvents.test.ts` — a separate circular-import-shaped issue in
`useDevServersSync.ts`, flagged for follow-up, not touched here).

Deployed as v0.4.34; final live re-verification via the same Playwright
script (form fill → submit → switch) — clean run: no
`PROJECT_MEMBERSHIP_LOOKUP_FAILED`, no React error #185, project created
and switched into successfully. All three bugs in this "New Project"
chain (Twenty-third's two + this one) are now confirmed fixed together.

## Twenty-fifth: existing projects created BEFORE the Twenty-third fix still had zero membership rows

User report: selecting an already-existing project ("Vnp-asm", created
before v0.4.33) from Project Workspace (Beta) threw `PROJECT_NOT_AUTHORIZED`
— a DIFFERENT symptom than Twenty-third's `PROJECT_MEMBERSHIP_LOOKUP_FAILED`
(that one was a hard DB error; this one is `requireProjectAccess` correctly
evaluating "no membership row, no global-admin override" and denying,
per its own fail-closed design). Live-queried the deployment's Postgres
directly (`project` database) to confirm rather than guess: of the 4
`project.projects` rows that existed, exactly 1 ("Vnp-asm",
`created_by=ea9d6c4b-...`) had zero matching `project.project_members`
rows — the other 3 were created during this session's own Twenty-third
live-verification runs, after the `AddMember` fix already shipped.

Fixed with a one-time backfill migration
(`project-service/migrations/0010_backfill_creator_membership.up.sql`):
`INSERT ... SELECT p.id, p.created_by, 'owner', now() FROM project.projects
p WHERE NOT EXISTS (a matching project_members row) ON CONFLICT DO NOTHING`
— covers every currently-orphaned project generically, not just this one
instance. Down migration is an intentional no-op (documented why: a
backfilled owner row is indistinguishable from one a real `AddMember` call
inserts afterward — same shape, no placeholder marker to filter on, unlike
`tenant-service/migrations/0002_backfill_legacy_bootstrap_company`'s
precedent which could key off a placeholder name). Deployed as v0.4.35 (a
migration-only change — `migrate.sh --remote` applies it as part of the
normal deploy flow, no Go/frontend code touched); live-verified via a
direct Playwright script — selecting "Vnp-asm" now loads its Workspace
(Git/Tasks/Workflows tabs) with no `PROJECT_NOT_AUTHORIZED` and no
`PROJECT_MEMBERSHIP_LOOKUP_FAILED`. Confirmed via direct `psql` query
against the live deployment (not just the RPC's success) that
`project.project_members` now has a matching owner row for "Vnp-asm".

**Separate, incidentally-discovered issue, not fixed here**: investigating
why the left sidebar's own "Projects" list is empty on this deployment
(user's separate question, answered below) surfaced that
`CreateProjectDialog.tsx`'s `repoPath` field is silently dropped — it has
no corresponding field in `project.proto`'s `CreateProjectRequest`/
`Project` message at all, so "New Project" never actually links a repo
into the created `OrcaProject` despite the dialog's own copy ("Register an
existing repo on a dev server as a new OrcaProject"). Flagged as follow-up
scope — out of this session's immediate bug reports.

## Twenty-sixth: unify project declaration + real per-project/per-repo authorization (Phases 1-3)

User request: one place to declare projects, with real authorization —
project-level owner/member (who's in a project) plus a NEW, separate
repo-scoped functional-role tier (admin/developer/lead — what a project
member can do on one specific repo within it). Planned as 4 phases; this
entry covers Phases 1-3, shipped and live-verified this pass. Phase 4
(migrating the legacy sidebar's own repo/worktree list onto this same
OrcaProject model) remains out of scope, deliberately deferred — see its
own section below for why doing more than a crash-fix here was rejected.

**Phase 1 — backend wiring for the existing owner/member tier**: added the
`project.addMember`/`project.rebindDevServer` wscompat channels (proto/
usecase/REST already existed; only the channel was missing — the SAME
`RebindDevServer`-was-never-reachable gap Twenty-third's audit already
flagged). Fixed `CreateProjectDialog.tsx`'s submit flow: `project.create`
has no `dev_server_id`/`repo_path` fields on the wire — both used to be
sent and silently dropped. Now: create → `project.rebindDevServer` (if a
dev server was chosen) → `repo.add` (if a repo path was given), each
failure surfacing as a toast, not blocking the already-created project.
Fixed `MemberManager.tsx` (built, but conflated the project owner/member
tier with a `developer/lead/admin` vocabulary that belonged to a different,
not-yet-built tier) — trimmed to `owner|member`, added the missing
"Add member" form (no `project.addMember` caller existed anywhere before).
Fixed `ProjectSettings.tsx` (resolved its project via an unsafe cast onto
the legacy `RepoSlice.projects` field — never actually matched; now reads
`WorkspaceContext.project`) and wired it into the UI for the first time (a
gear button next to `ProjectSwitcher`) — it was fully built, fully wired to
real RPCs, and had never been mounted anywhere. Consolidated `OrcaProject`/
`ProjectMember` into one definition each in `types/workspace-types.ts`.
Deleted the orphaned `store/slices/workspace-slice.ts` shadowing landmine.

**Phase 2 — repo.\* argument-shape fixes**: scoped down from the original
plan. The sidebar's own `repo.list`/`repo.add`/etc. callers (`repos.ts`)
have no `projectId` to send correctly in the first place — the legacy
sidebar has no notion of "current project" at all yet, so fixing their
arg shapes in isolation would be cosmetic (same empty/tenant-wide result,
just on-wire-correct). That plumbing is Phase 4's job. What Phase 1 above
already covers — `CreateProjectDialog`'s new `repo.add` follow-up call —
uses the correct shape (`{projectId, url, displayName}`) from the start.

**Phase 3 — new repo-level functional-role tier (admin/developer/lead)**:
genuinely new authorization surface, none of it existed before.
- `project.repo_members` table (migration `0011`), RLS via a two-hop
  subquery through `project.repos`/`project.projects` (one hop further
  than `project_members`' own RLS, since this is keyed on `repo_id`).
- `policy/orca-authz/repo.rego`: a second `action_roles` table
  (`repo_admin_only`/`repo_lead_or_admin`/`repo_any_functional_role`), with
  an explicit project-owner bypass (an owner's access never depends on
  holding a `repo_members` grant — it's opt-in, not required) plus the
  usual global-admin override. 12 new `opa test` cases, all passing
  alongside the pre-existing 23.
- New `requireRepoAccess` (mirrors `requireProjectAccess` one tier down):
  resolves project role first (owner bypass short-circuits before ever
  looking up repo membership — a repo with zero `repo_members` rows, the
  common case, never itself denies an owner), then a non-owner's
  `repo_members` row.
- 4 new usecases (`AddRepoMember`/`RemoveRepoMember`/`UpdateRepoMemberRole`/
  `ListRepoMembers`), new proto RPCs/messages (`RepoRole` enum, `RepoMember`
  message), new gRPC server methods, new wscompat channels
  (`repo.getMembers`/`addMember`/`removeMember`/`updateMemberRole` — mirrors
  the existing `project.*` member channels one tier down).
- `UpdateRepo`/`RemoveRepo` switched from project-level `owner_only` to the
  new repo-level `repo_admin_only` tier (with the owner bypass, so existing
  callers keep working). `ReorderRepos` deliberately did NOT move to the
  repo tier — it rewrites an entire project's repo ordering in one call, so
  there is no single repo to resolve a `repo_members` grant against;
  documented in its own doc comment as staying project-owner-only.
- `ListRepos`' visibility filter (this feature's core ask — "a developer
  sees/acts on repo X, a lead manages repo Y, not every repo in the
  project"): an owner still sees every repo; a non-owner member sees only
  repos they hold an explicit `repo_members` grant on.
- New frontend `RepoMemberManager.tsx` (mirrors `MemberManager.tsx`'s
  add/list/remove/re-role pattern, `developer|lead|admin` vocabulary) and a
  new "Repos" tab in `ProjectSettings.tsx` — a repo picker that renders
  `RepoMemberManager` for the selected repo.

Backend: `go build`/`go test` clean across every one of this workspace's 17
services (not just project-service/api-gateway) after the proto
regeneration; `opa test policy/orca-authz/` 35/35. Frontend: `tsc --noEmit`
clean for every touched file; every touched test file passes, including new
regression tests for the visibility filter (owner-sees-all vs.
member-sees-only-granted-repos) and the add/remove/re-role flows for both
tiers. Deployed as v0.4.36; live-verified via a direct Playwright script —
create project → dev server binds → repo attaches → add a project member
(confirmed via `psql`) → grant a repo-level `developer` role (confirmed via
`psql`) — all through the real UI, not a direct RPC call.

**A real regression surfaced during that same live verification** (see the
next entry) — fixed and redeployed as v0.4.37 before this phase could be
called done.

## Twenty-seventh: Phase 1's own repo.add fix woke up a dormant sidebar crash

Live-verifying Twenty-sixth's Phase 1 (`CreateProjectDialog` now actually
persisting a row into `project.repos` via `repo.add`, for the first time
ever on this deployment) immediately crashed the **legacy left sidebar** —
`[sidebar.worktrees] render crash contained by boundary TypeError: Cannot
read properties of undefined (reading 'replace')`, `console.error` cascade
of `WORKTREE_REPO_NOT_FOUND`/`PROJECT_MEMBERSHIP_LOOKUP_FAILED`, "The
workspace list hit an error" banner. A real regression, live-reproduced
immediately, not theoretical — the sidebar's own tenant-wide `repo.list`
fetch (`store/slices/repos.ts`'s `fetchRepoCatalogForTarget`, no project
filter — see Twenty-fifth's investigation) has zero notion of the
`OrcaProject` model and picked up this brand-new repo the instant it
existed, the moment `project.repos` stopped being empty for the first time
on this deployment.

Root cause, traced (not guessed) to `lib/repo-display-labels.ts`'s
`normalizePathSegments(path: string)`: the legacy `Repo` type's `path`
field doesn't exist at all on a project-service-created repo
(`{id, projectId, url, displayName, position}` — no `path`), so
`path.replace(...)` threw the moment 2+ repos collided on `displayName`
(including two both missing `displayName`, which both fall back to the
same `undefined` bucket here). Fixed with a guard
(`if (!path) {return []}`) — `labelForDepth` already falls back to
`item.displayName` for an empty segment list, so a path-less repo now just
displays its name unchanged instead of crashing. New regression test in
`repo-display-labels.test.ts` (a colliding path-less repo no longer throws,
falls back correctly) plus a defensive (harmless, but not the actual root
cause — kept anyway) guard in `WorktreeCard.tsx`'s `getDirectoryName`.

Deployed as v0.4.37; live-verified — the render crash and its "contained by
boundary" log are gone. A milder residual remained, confined to this
session's own test-created repo specifically: `fetchWorktrees` still logged
(caught, doesn't crash) a `PROJECT_MEMBERSHIP_LOOKUP_FAILED` for that one
repo, because the sidebar's legacy worktree-fetch path has no `OrcaProject`
concept to satisfy that check with — resolved simply by deleting that test
project once the user confirmed it was safe to (see "Twenty-ninth" below).

## Twenty-eighth: Phase 4a — mechanical `projectGroup.*`/`folderWorkspace.*` arg+response shape fixes

Follow-up exploration for Phase 4 (full sidebar-to-OrcaProject migration)
found the `repo.*` arg-shape bug was one of six families with the same
defect, live-firing on production right now: "New group from project"
(`projectGroup.update`), the folder-workspace wizard
(`folderWorkspace.create/update/delete/getPathStatus`), and the nested-
import wizard (`projectGroup.scanNested/importNested`). Some are genuine
concept mismatches needing a real product decision (deferred to a future
"Phase 4b" — see the plan file); `projectGroup.update` and
`folderWorkspace.update`/`delete` are pure mechanical fixes, shipped now:

- **`updateProjectGroup`** (`repos.ts`): the Go handler decodes `{groupId,
  name}` — the frontend sent `{groupId, updates: {...}}`, so `name` never
  reached the wire at all. Worse: `project.proto`'s `ProjectGroup`/
  `UpdateProjectGroupRequest` has no `tabOrder`/`isCollapsed`/`color`
  fields, and the usecase actively **rejects an empty name**
  (`PROJECT_GROUP_INVALID`) rather than treating it as "no change" — so
  every call here failed, including genuine renames ("New group from
  project" → rename never actually persisted) and tabOrder-only reorders
  (silently errored in the background). Fixed: only hit the backend when
  `updates.name` is actually set (skip the RPC entirely for
  tabOrder/isCollapsed/color-only changes — nothing for the backend to
  persist for them); merge the response into local state instead of
  replacing wholesale (the response has none of those fields either).
  **Also found and fixed the response shape**: the handler returns
  `resp.GetGroup()` bare (no `{group: ...}` wrapper) — the frontend was
  reading a nonexistent `.group` key, always `undefined`.
- **`deleteProjectGroup`**: args already matched (`{groupId}`), but the
  response reading didn't — the handler returns `map[string]bool{"ok":
  true}`, the frontend read `.deleted` (a key that was never on the wire).
  A successful delete always looked like a failure to this caller.
- **`updateFolderWorkspace`/`deleteFolderWorkspace`**: identical shape of
  bug — `{id, name}` not `{folderWorkspaceId, updates}` for update (proto's
  own doc comment: "name is the only mutable field"), `{id}` not
  `{folderWorkspaceId}` for delete, bare `FolderWorkspace` response not
  `{folderWorkspace: ...}`, `.ok` not `.deleted` for delete's result.
- `projectHostSetup.update`/`delete` were investigated too but NOT touched:
  their mismatch goes deeper than args — the frontend's response handling
  expects `{project, setup, repo?}` (a full Project+Repo+Setup bundle), but
  `UpdateHostSetupResponse`/`DeleteHostSetupResponse` return only a bare
  `HostSetup`/nothing at all. Fixing the args alone wouldn't make this
  actually work — a genuine feature-design gap, not a mechanical rename.
  Left alone; currently masked anyway (gated behind a capability string the
  backend never declares, per Twenty-sixth's Phase 4 findings), so zero
  live behavior change either way from leaving it.

All local-mode (`kind==='local'`, Electron desktop) branches were left
completely untouched — confirmed this session that b15.openledger.vn can
never reach that branch at all (`main-web-bootstrap.tsx` always persists a
`'session-auth'` environment before install for every cookie-authenticated
user).

**Incidental, high-value fix found while trying to test the above**: ALL
19 `repos*.test.ts` files (126 tests) were failing outright with
`TypeError: createRepoSlice is not a function` — a real, pre-existing
circular import (`store/index.ts` → `repos.ts` → `lib/onboarding-project-
checklist.ts` → `'@/store'` → back to `store/index.ts`, mid-evaluation).
Fixed by threading `settings` through `markOnboardingProjectAdded` as a
parameter instead of that module importing `useAppStore` at its own top
level (5 call sites updated). Found and fixed the identical shape of bug in
`store/slices/onboarding-checklist.ts` too (its own top-level `import {
useAppStore } from '../index'`, despite a stale comment claiming otherwise)
— its two genuinely-reactive selector hooks moved to a new
`onboarding-checklist-selectors.ts` file store/index.ts never has to load,
mirroring this session's earlier `dev-servers.ts`/`dev-servers-selectors.ts`
split for the exact same class of bug. This one fix alone took the
project-wide test suite from 51 failing files (found earlier this session)
down to 1 confirmed-pre-existing/unrelated failure.

Deployed as v0.4.38 (frontend-only change). `tsc --noEmit` clean for every
touched file; live-verified — no functional-behavior regression on load.
Did not create+rename a live test project group/folder workspace to
exercise the fixed paths end-to-end (would need fresh test data on
production and another cleanup round) — verification here rests on: exact
line-by-line comparison against the real Go handler's decode/encode
structs (not guessed), the pre-existing Go-side tests for those same
handlers (untouched, already passing), and new frontend unit tests
asserting the corrected request/response shapes against mocked RPC
responses shaped exactly like the real backend's.

## Twenty-ninth: live test-data cleanup

This session's live-verification Playwright runs created several
`project.projects` rows and one real `project.repos` row on
b15.openledger.vn (`verify-project-*`, `phase123-verify-*`). Direct
`DELETE` against the deployment's Postgres was blocked by the auto-mode
permission classifier (a destructive DB action) until explicitly confirmed
by the user; once confirmed, all 4 test projects were deleted (cascading
to their members/repos) — `project.projects` count back to 1 (the user's
real `Vnp-asm`), `project.repos` back to 0. This also resolved Twenty-
seventh's residual `PROJECT_MEMBERSHIP_LOOKUP_FAILED` log, since the repo
triggering it no longer exists.

## Thirtieth: Phase 4b — systemic camelCase wire-format bug (found live), `project.list` membership-scoping bug (found live), and the full sidebar-to-OrcaProject migration

Continuing Phase 4b (full sidebar migration onto OrcaProject) per the plan
file's already-approved scope. Two significant, previously-undiscovered
bugs surfaced along the way — both fixed and shipped ahead of finishing
the migration itself, since both were live, user-impacting, and (for the
second) security-relevant.

### Systemic bug: wscompat responses ship snake_case, not camelCase

While wiring `projectGroup.scanNested`/`importNested`'s request shapes
(the next mechanical step in Phase 4b-3), found that **every**
`project.*`/`projectGroup.*`/`projectHostSetup.*`/`folderWorkspace.*`/
`repo.*` channel returning a raw proto struct (`resp.GetGroup()`,
`resp.GetProject()`, `resp.GetSetup()`, `resp.GetFolderWorkspace()`,
`resp.GetRepo()`, `resp.GetCandidates()`, `resp.GetMembers()`, ...) ships
**snake_case field names** to the frontend, not camelCase.
`envelope.go`'s `Result any` field is serialized via plain
`encoding/json` (`wsjson.Write`), not `protojson` — and protoc-gen-go's
own `encoding/json` struct tags are always snake_case
(`json:"dev_server_id,omitempty"`), regardless of the proto field's
`json_name`. A raw `ProjectRole`/`RepoRole` enum also marshals as a bare
int, not the `"owner"`/`"member"`/`"developer"`/... string the frontend
expects. This exact bug class was already known and fixed for `profile.*`
(`userProfileView`/`departmentView`/`companyView`, pre-existing, own doc
comment explaining it) — it just was never applied to the rest of this
file family.

Confirmed this was genuinely invisible until now: every Go test mocks at
the usecase/repo layer (never marshals a real response), and every
frontend test mocks the RPC transport (never receives real backend JSON)
— so it passed every existing test AND every prior "live-verified"
Playwright check that happened to only touch single-word fields (`.id`,
`.name`). Multi-word fields — a project's `devServerId`/`createdAt`, a
member's `userId`/`role`/`addedAt`, a group's `parentGroupId` — were
silently `undefined` on the real deployment the whole time, including in
already-shipped Phase 1 (member management) and Phase 4a
(`projectGroup.update`'s `parentGroupId` was being wiped to `undefined`
on every rename, though nothing read it back yet so it was invisible).

**Fix**: added explicit camelCase "view" structs +
converters — `projectView`, `projectMemberView` (+ `fromProjectRoleArg`,
the inverse of the existing `toProjectRoleArg`), `projectGroupView`,
`hostSetupView`, `nestedRepoCandidateView`, `folderWorkspaceView` (+
`protoTimeToRFC3339`/`protoTimeMillis` for `*timestamppb.Timestamp`
fields, which also serialize wrong via plain JSON), `repoView`,
`cloneResultView`, `initRepoResultView` — across
`channels_tenant_project.go` and
`channels_emulator_folderworkspace_host.go` and
`channels_repo_ssh_status_workspace.go` (~19 call sites total). Also
fixed two independent wrapping-shape bugs found in the same pass:
`folderWorkspace.list` returned a bare array where the frontend expects
`{folderWorkspaces: [...]}`; `folderWorkspace.getPathStatus`'s
`existing_folder_workspace_id` had the same snake_case problem even
though `status` (a plain string field) happened to already be fine.

**New regression-test technique, added everywhere this touched**: a
`assertJSONKeys` helper that does a REAL `json.Marshal` of the handler's
return value and asserts the actual wire keys — the only thing that
actually catches this bug class (a Go-level type assertion on the
in-memory struct, which every existing test already did, passes
regardless of the JSON tag bug). Every fixed handler's test now asserts
both the Go-level shape AND, where practical, the marshaled keys.

**Known NOT fixed, flagged for a future pass** (out of the user's
explicitly-approved scope for this fix — project.\*/projectGroup.\*/
projectHostSetup.\*/folderWorkspace.\*/repo.\*): `profile.getResolved`
(`channels_tenant_project.go`, returns raw `*tenantv1.
GetResolvedProfileResponse`) and `emulator.listDevices`'s
`resp.GetDevices()` (`infrafleetv1.EmulatorDevice`) both look like the
same bug, unconfirmed/unfixed.

Deployed as **v0.4.39** (this fix alone, before continuing the rest of
Phase 4b) — full 17-service Go build/test sweep clean, all rewritten Go
tests passing, integration tests (real Postgres via testcontainers-go)
unaffected.

### Critical fix: `project.list` leaked every tenant member's projects to every other member

Found while designing 4b-4's `getOrCreateDefaultProject()` helper (which
needed to call `project.list` to find/reuse a user's own default
project): `usecase.ListProjects`/`postgres.Repository.List` filtered by
`tenant_id` **only** — no membership check at all. Any authenticated
tenant member's `project.list` call returned **every** project in the
tenant, not just their own. `GetMembership`/`ListMembers` already gated
per-project *access* correctly; nothing gated the *list* itself. This
directly undermined the "one private default project per user" design
this whole phase's remaining work depends on — a user's "private"
default project would have been visible to (and listable by) every other
tenant member.

**Fix**: `ProjectRepository.List` now takes `userID` and JOINs
`project.project_members` (`WHERE projects.tenant_id = $1 AND
project_members.user_id = $2`); `ListProjects.Execute` resolves the
caller's `userID` from context (`tenant.UserID`, already used elsewhere in
this file) the same way `AddMember`/`ImportNested` do. Regression tests
added at both layers: a fake-repository unit test
(`TestListProjects_ScopesToCallersMembership_NotWholeTenant`) and a real-Postgres
integration test (`TestRepository_List_ScopesToMembership`) — both seed
two tenant-mate-owned projects and confirm each user's `List` call
returns only their own. Also added
`TestListProjects_NoUserInContext_ReturnsUnauthenticated` for the new
context requirement. The two pre-existing `TestRepository_List_*`
integration tests (page-token pagination) needed updating to seed a
membership row per test project — they previously created projects with
no members at all, which the new membership-scoped query correctly now
excludes.

Deployed as **v0.4.40**, bundled with the finished 4b-1/4b-2/4b-3 backend
work (see below) — same full build/test sweep, clean.

### 4b-1 through 4b-4 (the rest of the migration)

- **4b-1** (`project.folder_workspaces` gets `project_group_id`): done in
  an earlier pass this session (migration `0012_folder_workspace_group`,
  domain/repo/usecase/proto/gRPC/wscompat threading) — 26/26 unit +
  integration tests passing, unaffected by this pass's other work.
- **4b-2** (`folderWorkspace.create`/`getPathStatus` frontend arg
  shapes): `createFolderWorkspace`/`fetchFolderWorkspacePathStatus`
  rewritten to send the real wire shapes and interpret real (bare, now
  camelCase-correct) responses via `mergeCreatedFolderWorkspaceResponse`/
  `toFolderWorkspacePathStatus`. A third call site sharing the same bug —
  `fetchRuntimeAddProjectPathStatus`, used only from `addRepoPath`'s
  non-git-fallback branch — was found and fixed in the same pass (not
  originally scoped, but the identical defect).
- **4b-3** (`scanNestedRepos`/`importNestedRepos`): request shapes fixed
  to `{devServerId, rootPath}`/`{devServerId, parentGroupId, selected}`.
  Response shapes are genuine concept bridges, documented as such:
  `mapRemoteNestedScanCandidates` fills in scan-progress metadata
  (`truncated`/`timedOut`/`durationMs`/`maxDepth`) this deployment's
  backend genuinely cannot report (a flat one-shot candidate list, no
  streaming scan concept) with deterministic placeholders, not invented
  telemetry. `mapRemoteImportNestedResult` maps the backend's
  `{createdGroups, createdProjects}` (index-aligned with the request's
  `selected` array — confirmed against
  `ProjectGroupRepository.ImportNested`'s real implementation: it
  **always** creates one new group + one new project **per candidate**,
  atomically, in one transaction — the frontend's `groupName`/`mode`
  ("import as one shared group" vs "separately") have **no server-side
  equivalent at all** and are accepted but silently ignored server-side).
  This is a real, disclosed product-behavior gap, not a bug: since this
  whole call was **already completely non-functional** on the remote
  path before this fix (arg shape never matched), nothing regresses —
  nested import now actually works, just without the grouping concept the
  local/Electron path's own scanner offers. `NestedRepoCandidate`
  (`shared/types.ts`) gained optional `suggestedName`/`isGitRepo` fields
  so `importNestedRepos`'s new optional `selectedCandidates` param can
  round-trip the exact shape the backend requires (both call sites —
  `useAddRepoNestedImportFlow.ts`, `use-onboarding-flow.ts` — updated to
  pass `nestedScan.repos`).
- **4b-4** (the core migration — `repo.list`/`add`/`create`/`clone`
  become project-scoped, one private default project per user):
  - `getOrCreateDefaultProject(target)` (new, exported from `repos.ts`):
    calls `project.list` (now correctly membership-scoped per the fix
    above), reuses the earliest-created project the caller already has
    access to, or `project.create`s one named `"My Repos"` /
    `visibility: 'private'` if none exists. Cached module-scoped per
    session (reset via `resetDefaultProjectCacheForTests()` in tests);
    safe across logout since `useLogout.ts` does a full
    `window.location.href` redirect, resetting all module state.
  - `mergeRepoViewIntoRepo`/`mapRemoteRepoViewsToRepos` (new, exported):
    the project-service `Repo` (`{id, projectId, url, displayName,
    position}`) and the legacy sidebar `Repo` (`path`, `badgeColor`,
    `kind`, `connectionId`, `worktreeBaseRef`, ...) are fundamentally
    different shapes (flagged as a load-bearing structural finding back
    in the memory file) — this bridges them, preserving all local-only
    fields from any existing record and defaulting sensibly
    (`badgeColor: ''`, falls back to the render layer's own
    `DEFAULT_REPO_BADGE_COLOR`) on first sight. `Repo` gained an optional
    `projectId?: string` field.
  - `fetchRepoCatalogForTarget`/`repo.list`: now resolves the default
    project and sends `{projectId}`; every caller threads its current
    `get().repos` through so the merge above can preserve existing
    local-only state across a refresh (previously `mergeById`'s replace
    semantics would have silently discarded it every fetch — a bug that
    was academic before this pass since the RPC never worked at all, but
    is real now that it does).
  - `addRepoPath`'s remote branch, `useCreateRepo.ts`, and
    `useAddRepoCloneFlow.ts`: `repo.create`/`repo.clone` only relay to
    git-gateway-service (init/clone onto disk) — they do **not** register
    a `project.repos` row (confirmed against the Go handlers: bare
    `{path, defaultBranch}` / `{worktreePath, defaultBranch}` responses,
    no repo info at all). Both hooks now do the two-step sequence
    `repo.create`/`repo.clone` then `repo.add` (mirroring
    `CreateProjectDialog.tsx`'s already-shipped create-then-add pattern),
    resolving the default project id and `activeDevServerId` the same way
    4b-2 established.
  - `repo.reorder`: fixed to `{projectId, repoIdsInOrder}` (was
    `{orderedIds}`); the Go handler returns no body on success (a
    rejection is a thrown error, not a `{status}` field) — normalized to
    the same `{status:'applied'}` shape the local/Electron branch already
    returns, so the existing "refetch on rejection" logic needs no
    change.
  - `AddRepoDialog.tsx`'s 5 sub-flows needed no changes at all — confirmed
    each one funnels into one of the store actions/hooks above and none
    make a direct RPC call of their own.

### Test fixes

~25 failing tests across 10 files (all pinning the old wire shapes)
delegated to a background subagent — fixed cleanly, plus 2 more
collateral failures the subagent found and fixed on its own initiative
(`settings.test.ts`, `useAddRepoServerPathFlow.test.ts`). Two files then
needed splitting to stay under oxlint's 800-line test-file budget
(`config/max-lines-baseline.txt`'s own header explicitly forbids adding
baseline entries as a shortcut — "split the oversized file instead"):
`repos.test.ts`'s 4 newly-verbose default-project-resolution tests moved
to a new `repos-runtime-default-project.test.ts`;
`repos-all-hosts.test.ts`'s 9-test "shared project metadata across hosts"
cluster (plus its two dedicated mock-setup helpers) moved to a new
`repos-all-hosts-shared-project-metadata.test.ts`, with the common
fixture/mock setup both `repos-all-hosts*.test.ts` files now share
extracted into `repos-all-hosts-fixture.ts` (mirrors the pre-existing
`repos-runtime-routing-fixture.ts` pattern). Final state: 147/147 passing
across every touched file; broader `store/slices` + `components/sidebar`
+ `components/onboarding` + `components/project` sweep (3281 tests) shows
exactly the same 6 pre-existing/unrelated failing files as before this
pass (confirmed via `git diff` showing zero session changes to any of
them, plus one more matching the same "missing unrelated module"
root-cause class already documented elsewhere in this repo's ongoing,
separate refactor).

Deployed as **v0.4.41**.

### Live verification (v0.4.41) — and a second live-only bug it immediately surfaced

Playwright against the real b15.openledger.vn deployment, logged in as the
real admin user: sidebar's "Add Project" → "Browse folder" → typed `/tmp`
on the paired `test-01` dev server → "Open as Folder". Result: a real
`project.repos` row appeared (`url=/tmp, display_name=tmp`), correctly
attached to the user's existing `Vnp-asm` project (confirming
`getOrCreateDefaultProject` reuses an existing project rather than
creating a redundant one) — not a new "My Repos" project, since this user
already had one. The sidebar immediately showed "tmp" as a real workspace
with a "Folder added" toast. This is the first repo.add call to ever
actually work on this deployment's remote path.

**But**: reloading the page immediately after surfaced a second, previously-
invisible live bug — `The workspace list hit an error`, from `fetchWorktrees`
throwing `PROJECT_MEMBERSHIP_LOOKUP_FAILED`. Root cause: `worktree.
detectedList`'s frontend call (`worktrees.ts`) sent `{repo: repoId}`, but
the Go handler (`channels_worktree.go`) decodes `{projectId, repoId}` and
fans out to `ListWorktreesRequest{ProjectId: in.ProjectID}` — an empty
`projectId` bound into project-service's uuid-typed column produces a real
Postgres error, wrapped as `PROJECT_MEMBERSHIP_LOOKUP_FAILED` (the exact
"empty string into a uuid column" bug class as BUG-004) rather than a
clean empty result. **Never reachable before this pass** — no project-
service-backed repo had ever survived long enough in the sidebar's state
to trigger a worktree refresh against it. Fixed: `listDetectedWorktreesForRepo`/
`listDetectedWorktreesForRepoCoalesced` (and all 4 call sites —
`fetchDetectedWorktrees`, `fetchWorktrees`, both `fetchAllWorktrees`
branches) now thread a `projectId` (new `repoProjectId()` helper, reading
`Repo.projectId`) through and send `{projectId, repoId}` — matching the
Go decode struct's actual field names (`repo` was never a recognized key
either). `worktree.list`'s legacy fallback (older runtime servers without
`detectedList`) fixed the same way, dropping a `limit` field the Go
handler never reads at all.

**Found but explicitly NOT fixed this pass — flagged as its own future
piece of work**: `worktree.detectedList`'s Go handler's actual RESPONSE
shape is `{orphanedPaths: [...]}` — nothing like the frontend's expected
`DetectedWorktreeListResult` (`{repoId, authoritative, source, worktrees:
[...]}`). This is not a camelCase-naming bug, it's a genuine shape/concept
mismatch (the handler was seemingly built around an older "which on-disk
paths aren't tracked yet" contract, not the current frontend's "full
merged worktree list" one) — the same class of gap as `folderWorkspace.
create`'s concept mismatch found earlier this session, and confirms
`worktree.*`'s wiring needs its own dedicated audit pass, comparable in
size to this whole Phase 4b, not a quick follow-on fix. Also NOT audited/
fixed: `worktree.create`/`worktree.set`'s own raw-proto-struct camelCase
returns, and `Worktree`'s rich frontend type (`instanceId`, `linkedPR`,
`linkedLinearIssue`, ...) vs. the backend's minimal `{id, projectId,
repoId, path, branch, active, ...lineage}` — the same "backend response is
a partial record" bridge `mergeRepoViewIntoRepo` solved for `Repo` would
be needed here too, but is a substantially larger undertaking given how
much richer `Worktree`'s frontend shape is. None of this blocks what
Phase 4b set out to do (repos now correctly persist and list); it does
mean the sidebar's WORKTREE-level data for a project-service-backed repo
is not yet fully wired end-to-end — a real git repo added through this
same flow will likely need this follow-up before its worktrees display
correctly.

Redeployed as **v0.4.42** (worktree.detectedList/list request-shape fix
only).

**Re-verified after v0.4.42**: the request-shape fix alone was not
sufficient — reloading still errored, now with a *different* underlying
cause surfacing after the membership-lookup one cleared:
`WORKTREE_REPO_NOT_FOUND: repo does not exist` from git-gateway-service's
`DetectWorktrees`. Root cause: git-gateway-service keeps its own separate
repo registry, populated only by ITS OWN flows (`InitRepo`/`Clone`/
`SetupExistingFolder`) — a repo added via project-service's bare
`AddRepo` (what `repo.add`/`addRepoPath` call, for both git and folder
kind) was never registered there at all, so `DetectWorktrees` fails
outright rather than returning an empty on-disk-paths list. Confirmed
this isn't specific to the folder-kind test case — a git-kind repo added
the same way (bare `repo.add`, not through `projectHostSetup.
setupExistingFolder`) would hit the identical gap. Combined with the
already-flagged `{orphanedPaths}` vs `DetectedWorktreeListResult` response
mismatch, this confirms `worktree.*`'s integration with the newly-working
project-scoped `repo.add` path is a genuine, substantial, architectural
gap — not a wire-shape bug fixable by one more patch. **Decision: stop
here, do not chase further** — this is its own initiative, not a Phase 4b
follow-on, given it now spans two services' repo registries staying in
sync, not just a frontend/api-gateway shape mismatch.

The `/tmp` test repo added during live verification was removed
(`DELETE FROM project.repos WHERE id='7c8b648c-...'`) once the fix was
confirmed working at the persistence layer — `project.repos` is back to 0
rows, and the sidebar reload was re-verified clean (no error banner,
"No workspaces found" — the exact starting state, no test data left
behind). **Net effect of this whole pass, for the sidebar as a user
experiences it today**: the previous state (nothing ever persisted, so
the sidebar just showed empty) is unchanged for now — adding a repo
through "Add Project" will persist correctly (confirmed via `project.
repos`/`project.list`/`project.rego` membership scoping, all real and
tested) but the very next worktree refresh will surface an error until
the `worktree.*` gap above gets its own dedicated fix. Flagging this
prominently rather than either silently shipping a worse user-visible
state or open-endedly chasing a second large initiative inside this
already-large session.

## Thirty-first: sidebar showed only the caller's default project's repos, plus the full worktree.* crash chain the fix exposed on a real repo

Follow-up to the Thirtieth entry, triggered by a direct user question:
Project Workspace (Beta) showed the user's real `vnp-asm` project, but the
legacy left-sidebar "Projects" list did not list it at all. Root cause:
`fetchRepoCatalogForTarget`'s remote branch called `getOrCreateDefaultProject`
then `repo.list({projectId: defaultProjectId})` — a user who belongs to
*multiple* projects (e.g. one created through Project Workspace's own "New
Project" dialog, not through the sidebar's "Add Project") only ever saw
their one auto-created default project's repos. Fixed (v0.4.43) by adding
`fetchAllRemoteRepoViews(target)`: resolve every project the caller belongs
to via a new shared `listCallerProjects(target)` helper, then
`Promise.all`-fetch `repo.list({projectId})` per project (each call
individually caught → `[]` on error) and flatten. `getOrCreateDefaultProject`
keeps using `listCallerProjects` too, unchanged behavior for the
single-project ADD path. `mergeRepoViewIntoRepo`/`mapRemoteRepoViewsToRepos`
bridge the minimal wire `RemoteRepoView` into the legacy sidebar's richer
`Repo` type across all fetched projects, not just one.

Deploying this fix (which, for the first time, actually reached
`vnp-asm` — a real project-scoped repo, not synthetic test data) surfaced a
**live render crash** on the user's own repo: `[sidebar.worktrees] render
crash contained by boundary`, `TypeError: Cannot read properties of
undefined (reading 'replace')`. Traced through four independent, stacked
layers before reaching the true root cause — each one masked the next
until the previous was fixed:

1. **`worktree.detectedList` wrong request shape** (v0.4.42, already fixed
   in the Thirtieth entry above) — confirmed still necessary and correct.
2. **`worktrees.ts` didn't catch `WORKTREE_REPO_NOT_FOUND`/
   `PROJECT_MEMBERSHIP_LOOKUP_FAILED` gracefully** even after the request
   shape was correct — `listDetectedWorktreesForRepo`'s catch block now
   soft-returns `{repoId, authoritative: false, source: 'metadata-fallback',
   worktrees: []}` instead of rethrowing (v0.4.44). An earlier attempt to
   short-circuit *before* the RPC call when `projectId` was empty broke 11
   existing tests expecting the call to still fire with legacy fixtures —
   reverted in favor of the post-call-only catch, which fixed the crash
   without breaking those tests.
3. **A second, fully independent, previously-undiscovered implementation**:
   `web-preload-api.ts` (the cookie-auth web preload shim) has its own
   parallel repo/worktree-fetching code (`callRuntimeDetectedWorktrees`,
   `listAllRuntimeDetectedWorktrees`, `createReposApi().list()`) — never
   touched by any of the Phase 4b work above, with the identical bug class
   (`{repo: repoId}` shape with no `projectId`; casts the bare
   `RemoteRepoView` wire shape straight onto the rich `Repo` type with no
   bridge). Reached via `App.tsx`'s hardcoded "local-first" bootstrap fetch
   (`fetchRepoCatalogForTarget({kind: 'local'})`, always attempted
   regardless of session type) → `window.api.repos.list()`. Patched
   minimally (v0.4.45): wrapped both functions in try/catch for graceful
   soft-empty degradation on the same two error codes as (2). **Not**
   rewritten to properly bridge the wire shape — flagged as its own,
   larger, separate follow-up (see below).
4. **The actual root cause of the visible crash, independent of any RPC
   failure** — confirmed empirically by eliminating every known RPC error
   path above and observing the crash *still* occurred with zero RPC errors
   logged: `frontend/src/shared/wsl-paths.ts`'s `parseWslUncPath(path)` and
   `frontend/src/shared/cross-platform-path.ts`'s
   `isWindowsAbsolutePathLike(value)` called `.replace(...)`/`.startsWith(...)`
   on `undefined`. `worktree-list-groups.ts`'s
   `getProjectSetupSurfaceKey(setup: ProjectHostSetup)` calls both,
   unguarded, on `setup.path` — which is `undefined` at runtime for a
   synthetic `ProjectHostSetup` derived (via
   `projectHostSetupProjectionFromRepos`) from a `Repo` fetched through the
   web-preload-api.ts path in (3): the raw wire shape only has `url`, never
   `path`, despite the TS type declaring `path: string`. Fixed (v0.4.46)
   with the same defensive-guard pattern already established for
   `repo-display-labels.ts`'s `normalizePathSegments` (Twenty-seventh
   entry): `if (!path) return null` / `if (!value) return false` at the top
   of each function, plus regression tests asserting `undefined`/`''` no
   longer throw.

Debugging technique worth recording: once RPC-level causes were ruled out,
rebuilt the frontend locally with `vite build --sourcemap` (confirmed
byte-identical asset hash to the live deployment — Vite output is
deterministic given identical source), then used the pre-installed
`@jridgewell/trace-mapping` package via a one-off Node script to decode the
minified stack trace (`useLogout-C0mhLDWY.js:33:56848`) back to
`wsl-paths.ts:7:26` — no debug build or local dev proxy needed.

Live-verified clean (v0.4.46): fresh login and a full page reload both show
the sidebar correctly listing the user's real project (containing
`vnp-asm`) with no error banner and no console errors beyond expected
unrelated 401/MIME noise — the first fully clean result across the entire
v0.4.42–v0.4.46 investigation chain.

**Still explicitly deferred, not fixed this pass** (consistent with the
Thirtieth entry's own stop-here decision, now extended by one more found
instance): the full `worktree.*` subsystem's integration with
project-scoped repos (git-gateway-service's separate repo registry,
`worktree.detectedList`'s response-shape mismatch, `worktree.create`/`set`'s
own camelCase bugs) — and now also `web-preload-api.ts`'s entire parallel
repo/worktree-fetching implementation, which needs the same
`mergeRepoViewIntoRepo`-style bridge `repos.ts` already got in Phase 4b, not
just a try/catch. Both remain their own separate initiative.

## Thirty-second: worktree.* subsystem's project-scoped-repo integration (real fix, not a workaround), and web-preload-api.ts's parallel implementation properly bridged

Follow-up to the Thirty-first entry's two explicitly-deferred items, done at
your explicit request ("Toàn bộ subsystem worktree.* tích hợp với repo
project-scoped... web-preload-api.ts's bộ code song song... và thực thi
giúp tôi").

**Root cause, precisely identified (correcting the Thirtieth entry's
"git-gateway-service keeps its own separate repo registry" characterization —
real, but not the actual blocker)**: `ProjectClient.GetRepo` in
git-gateway-service (`internal/adapter/grpcclient/project_client.go`) was a
STUB that unconditionally returned `PROJECT_GET_REPO_UNIMPLEMENTED` — every
call to `DetectWorktrees`/`CreateWorktree`/`PrefetchCreateBase`/
`ResolvePrBase`/`ResolveMrBase` failed at this point, for literally every
repo, regardless of how it was added. The actual gap was narrower than
"two disconnected registries": project-service's `project.proto` simply had
no RPC to look up one repo by id (only `ListRepos(project_id)`, which needs
a project id these calls don't have). Once fixed, a SECOND, independent bug
surfaced: `dispatchExecutor`'s repo-scoped callers passed `repo.ID` into
`ConnectionResolver.ResolveConnection` as if it were a worktree/connection
id — but no `infra.connections` row is ever keyed by a bare repo id (only
`SetupExistingFolder`/`ScanNested` call `CreateConnection`, keyed by a
dev-server path). This was silently never correct; it only looked "fixed
enough" because `WORKTREE_REPO_NOT_FOUND` from the GetRepo stub always
fired first.

**Fix, backend (git-gateway-service ↔ project-service)**:
1. New `ProjectService.GetRepo` RPC (`project.proto`): `{repo_id}` →
   `{repo, dev_server_id}` — resolves the repo's owning project's
   `dev_server_id` server-side so callers don't need a second RPC. New
   usecase `GetRepo` (project-service): tenant-scoped only, no per-user OPA
   check — deliberately matching `RecordWorktreeCreated`/
   `RecordWorktreeRemoved`'s existing precedent for this exact
   service-to-service trust boundary (git-gateway-service's outbound calls
   only forward tenant via `withTenantMetadata`, never the acting user —
   the original wscompat caller already authorized the end user before
   ever reaching git-gateway-service).
2. `git-gateway-service`'s `ProjectClient.GetRepo` now calls this RPC for
   real; `domain.RepoInfo` gained a `DevServerID` field.
3. New `dispatchExecutorForRepo(ctx, reachability, local, relay, repo)` in
   `usecase/ports.go` — mirrors `Clone`/`InitRepo`'s ALREADY-WORKING
   `DevServerReachability`-based dispatch (ask infra-fleet-service's
   fleet-health sample whether the repo's dev server is a live,
   agent-reachable host; if not, or if no dev server is bound at all,
   operate locally at `repo.URL`, which carries an absolute filesystem path
   for repos set up via `SetupExistingFolder`/`ImportNested`) — instead of
   `dispatchExecutor`'s connectionId-keyed lookup, which never worked for a
   bare repo id. `DetectWorktrees`, `CreateWorktree`, `PrefetchCreateBase`,
   `ResolvePrBase`, `ResolveMrBase` all switched from `ConnectionResolver`
   to `DevServerReachability` accordingly. `RemoveWorktree` is unchanged —
   it dispatches on a real `worktreeId` (the natural connectionId key once
   a worktree row exists), a genuinely different case.
4. **Known gap this does NOT close, pre-existing and shared with Clone/
   InitRepo** (documented in `dispatchExecutorForRepo`'s own doc comment,
   not silently left): the relay branch still needs an `infra.connections`
   row keyed by `repo.URL` for `RelayExecutor`'s connectionID-doubles-as-
   repoPath convention to resolve — nothing creates one on demand. Confirmed
   this deployment's dev servers report no reachable fleet-health sample
   today (`IsReachable` returns false, every call takes the local branch),
   so this gap doesn't block the common case here, but remains open for a
   genuinely relay-connected dev server.
5. A THIRD, previously-masked issue this surfaced: `worktree.detectedList`'s
   wscompat handler (`channels_worktree.go`) still returns
   `{orphanedPaths: [...]}`, not the frontend's `DetectedWorktreeListResult`
   shape (`{repoId, authoritative, source, worktrees}`) — this was only
   ever unreachable before because the RPC always errored first. Building a
   real, correct `DetectedWorktree[]` server-side needs UI-only fields
   (`ownership`/`selectedCheckout`/`visible`, plus `Worktree`'s
   `comment`/`linkedIssue`/... bookkeeping) this backend has no concept
   of — a genuine product/architecture decision, not a mechanical fix, so
   deliberately NOT attempted here. Instead, added a defensive shape guard
   in both consumers (`store/slices/worktrees.ts`'s
   `listDetectedWorktreesForRepo` and `web-preload-api.ts`'s
   `callRuntimeDetectedWorktrees`): if the response's `worktrees` field
   isn't an array, degrade to the same safe `{authoritative: false, source:
   'metadata-fallback', worktrees: []}` state the known-error catch already
   used — so the RPC now succeeding (where it used to fail) can never
   crash a caller that assumes `.worktrees` is iterable. **This response-
   shape synthesis is still open, flagged the same way the Thirtieth/
   Thirty-first entries flagged worktree.* overall** — do not assume
   `worktree.detectedList` returns real reconciled data yet.

**Fix, frontend (`web-preload-api.ts`'s parallel implementation)**: this
file had its own, never-updated copy of the entire pre-Phase-4b bug class —
`repo.add`/`addRemote` sent `{path, kind}` (no `projectId`, no `url`/
`displayName`), `repo.rm` sent `{repo: repoId}`, `repo.reorder` sent
`{orderedIds}` (no `projectId`), `repo.update` sent `{repo: repoId,
updates}`, and `repo.list`'s bare `RemoteRepoView[]` was cast directly onto
the rich `Repo` type with no bridge at all. Added a local mirror of
`store/slices/repos.ts`'s Phase 4b fix — `RemoteRepoView`,
`repoDisplayNameFromUrl`, `mergeRepoViewIntoRepo`, `fetchAllRemoteRepoViews`
(lists repos across every project the caller belongs to, not just one),
`getOrCreateDefaultProjectId` — a SEPARATE copy, not an import, since this
file's runtime call plumbing (`callRuntimeResult`, bound to the one paired
connection) is a different transport than `repos.ts`'s target-parameterized
`callRuntimeRpc`; explicitly documented as such so a future shape change to
either gets applied to both. Fixed `list`/`add`/`addRemote`/`remove`/
`reorder`/`update` to the correct wire shapes; `reorder` resolves its
required `projectId` by fetching the current repo list first (the same way
`repos.ts`'s own `reorderRepos` action does from already-known local state).
`callRuntimeDetectedWorktrees`/`listDetected` now send `{projectId,
repoId}` and carry the same defensive shape guard as item 5 above.
**Deliberately NOT touched**: `clone`/`create` (git-gateway-service's raw
`Clone`/`InitRepo` RPCs) — their correct fix needs resolving a `devServerId`
for the web-preload context and adopting the two-step create/clone-then-add
pattern `useCreateRepo.ts`/`useAddRepoCloneFlow.ts` already use in
`repos.ts`, which is a larger, less-confirmed-reachable piece of work (this
file's `create`/`clone`/`add`/`update`/`remove`/`reorder` are only ever
exercised via `App.tsx`'s always-on "local-first" bootstrap fetch or a
genuine `kind==='local'` desktop-parity path — `list` and the worktree-
detection chain are the ones confirmed live-reachable on b15; the rest were
fixed for consistency/correctness but their reachability on this specific
deployment is unconfirmed).

**Verification**: `go build`/`go vet`/`go test ./...` clean across all 17
backend-go services (new tests: `get_repo_test.go` for the new usecase —
tenant-only auth, not-found, missing-id; `detect_worktrees_test.go`,
previously untested entirely — local/relay dispatch by reachability,
repo-not-found, reachability-check failure, `ListWorktreePaths` failure).
Frontend: `npx tsc --noEmit` shows only 3 pre-existing, unrelated errors
(confirmed via `git stash` — identical errors, same messages, present on
the unmodified branch); `npx oxlint` clean on every touched file;
`web-preload-api.test.ts` 69/70 passing (the 1 failure is a pre-existing,
confirmed-unrelated missing test fixture file, `src/preload/gitlab.ts`,
absent on the unmodified branch too); `worktrees.test.ts` 208/208 (fixed 2
more pre-existing failures from an earlier, undocumented local-branch
`{repoId, projectId}` shape change that had no matching test update);
`repos.test.ts`/`repos-all-hosts.test.ts`/`repos-runtime-default-
project.test.ts`/`repos-all-hosts-shared-project-metadata.test.ts`/
`wsl-paths.test.ts`/`cross-platform-path.test.ts` all still green.

Deployed as v0.4.47. Live-verified via a direct Playwright script against
the real user session: confirmed `PROJECT_MEMBERSHIP_LOOKUP_FAILED` and
`WORKTREE_REPO_NOT_FOUND` are GONE from the console entirely — real,
durable progress past the exact bug this pass targeted. Follow-up
investigation of a FOURTH issue that surfaced past that point (`vnp-asm`'s
project has an empty `dev_server_id`, so `dispatchExecutorForRepo` takes
the "operate locally" branch, but git-gateway-service's dev-deploy
container had no `git` binary or writable repo storage at all) is resolved
in the Thirty-third entry below — it was a genuine, fixable infra gap in
THIS deployment, not an unresolvable architectural one.

## Thirty-third: git-gateway-service's dev-deploy container gets real git tooling + persistent storage; web-preload-api.ts's bare-response bug fixed

Follow-up to the Thirty-second entry, done at your explicit request
("đưa ra giải pháp, đánh giá và thực hiện fix giúp tôi") for the
`WORKTREE_DETECT_FAILED` gap that entry flagged as infra/product, not code.

**Root cause, precisely confirmed**: `backend-go/services/git-gateway-
service/deploy/Dockerfile` (the CORRECT, already-designed production image
— installs `git` from Debian into a distroless base specifically because
`internal/adapter/localgit` shells out to it) was never actually used by
THIS deployment. `deploy/dev/docker-compose.yml` bind-mounts every backend-
go service's binary into the same shared, unmodified
`gcr.io/distroless/static-debian12:nonroot` image — correct for the other
16 services, wrong for this one, which is the sole service with a
host-local exec path per specs/backend-go/tdd/services/git-gateway-
service.md §2 ("no connectionId → execute locally... retained for
local/dev deployments" — exactly this deployment's own topology).

**Fix**:
1. New `deploy/dev/docker/git-gateway-runtime.Dockerfile` — mirrors the
   real Dockerfile's runtime stages (skips its Go build stage, since
   `build-local.sh` already compiles the binary separately and bind-mounts
   it in, like every other service). Live-verified gap in the ALREADY-
   correct production Dockerfile this mirrors, found while testing:
   copying only `/usr/bin/git` + `/usr/lib/git-core` + `/usr/share/git-
   core` (no shared libraries) fails at runtime — `git: error while
   loading shared libraries: libpcre2-8.so.0` — distroless/base-debian12
   ships libc but not git's other dynamic deps (and git-core sub-binaries
   like `git-remote-https` pull in libcurl/libssl/libcrypto for HTTPS
   transport on top of that). Fixed by copying the whole `/usr/lib/x86_64-
   linux-gnu`/`/lib/x86_64-linux-gnu` directories rather than hand-picking
   `.so` files.
2. `docker-compose.yml`: `git-gateway-service` gets its own `image:`
   override (`orca-git-gateway-runtime:${ORCA_GO_VERSION:-dev}`),
   `read_only: false` (the ONE exception to every other service's
   read-only-rootfs convention — this service alone needs to write real
   git/worktree data), and a new `git-gateway-repos` named volume mounted
   at `/data/repos` — a new storage convention (no prior one existed
   anywhere in the codebase), created+chowned to distroless's fixed
   `nonroot` UID/GID (65532) in the Dockerfile's build stage so a fresh
   Docker-managed volume (root-owned by default) doesn't deny the
   `nonroot`-running daemon a `mkdir`.
3. `build-local.sh`: new step 3/3 builds+tags this image locally
   (`--platform linux/amd64`, matching the Go binaries' own explicit
   cross-compile target).
4. `sync-to-server.sh`: new step 3/6 transfers the image via
   `docker save | ssh docker load` (no registry exists for it — the
   deploy flow's one purposeful deviation from its own "no docker compose
   build, no custom image" doc comment, now updated to say so), and step
   6/6 passes `ORCA_GO_VERSION` inline to the remote `docker compose up`
   (the one place this variable needs to be visible server-side, since
   this script never overwrites an existing server `.env`).
5. **This repo's `Repo.URL` is now genuinely used as a real, writable,
   on-disk path** for any repo with no dev-server binding — confirmed via
   a live, end-to-end round trip: `repo.add` a path under `/data/repos`,
   `git init` + `git commit --allow-empty` + `git worktree add` inside the
   deployed container (via `docker exec`) all succeed; `DetectWorktrees`
   (this pass's earlier GetRepo/dispatch fix) resolves and lists that
   worktree correctly — confirmed via a temporary debug log added and then
   removed for this diagnosis (`slog.Error` in `DetectWorktrees`/
   `ListWorktrees`/`requireProjectAccess`, never present in the final
   deploy): zero failures logged for the test repo across many repeated
   calls, every failure attributable to the KNOWN-broken `vnp-asm` row
   (invalid path, pre-dates this fix, can't be resurrected by an infra
   change) or to `errgroup`'s expected context-cancellation semantics for
   that same row's own sibling goroutine.
6. **A live bug in `web-preload-api.ts`'s own bridge, found and fixed
   during this verification**: `add`/`update`/`addRemote` (this session's
   own new code, from the Thirty-second entry) wrapped `repo.add`/
   `repo.update`'s response in `{repo: RemoteRepoView}` — but
   `channels_repo_ssh_status_workspace.go`'s handlers return the bare
   `repoView` directly (confirmed against the Go source, matching
   `repos.ts`'s own already-correct handling of the same call). Fixed by
   removing the wrapper; live-verified via a real `repos.add` round trip
   returning a correctly-populated `Repo`.

**Still open, confirmed NOT a bug**: `worktree.detectedList`'s response-
shape mismatch (`{orphanedPaths}` vs the frontend's `{repoId,
authoritative, source, worktrees}` — the Thirty-first entry's own
documented, deliberately-unfixed gap). The debug logging added for this
pass's diagnosis proved conclusively that a repo with real on-disk git
data and a real dev-server-unbound project hits NEITHER known error
code — both `git.DetectWorktrees` and `project.ListWorktrees` succeed —
and the client's defensive shape guard (added in the Thirty-second entry)
correctly degrades to `{authoritative: false, source: 'metadata-fallback'}`
rather than crashing on `.worktrees` being undefined. Synthesizing a real
`DetectedWorktree[]` server-side still needs the same UI-only fields
(`ownership`/`selectedCheckout`/`visible`, plus `Worktree`'s richer
bookkeeping) this backend has no concept of — unchanged from the Thirty-
first entry's assessment, still its own follow-up.

**Verification**: `go build`/`go vet`/`go test ./...` clean across all 17
services (debug logging added, then fully removed and re-verified clean
before the final deploy). Local Dockerfile smoke tests: `git --version`,
`git init`+`worktree add` against a fresh named volume (not just an
already-warm one) with no `--user` override (confirming the nonroot-UID
ownership fix). Deployed and live-verified end-to-end on b15.openledger.vn
as v0.4.53 (intermediate `-debug` tagged versions used only for the
temporary diagnostic logging, torn down before this final version). Test
data cleanup: the temporary `/data/repos/*` test repos and their
`project.repos` rows were deleted after verification; `vnp-asm`'s own
row — real user data with a path that never existed on any host, added
before this fix existed — was deliberately left untouched, not something
an infra fix can retroactively repair.

## Thirty-fourth: worktree.detectedList's response-shape synthesis, the Create/Clone Project dev-server-selection dead branch, and a live port-scanner crash

Follow-up to the Thirty-third entry's remaining-issues summary table, done
at your explicit request ("thực thi và xử lý toàn bộ các vấn đề còn lại").

**1. `worktree.detectedList`'s response-shape gap — closed for real, not just
safely degraded.** The Thirty-second/Thirty-third entries' shape guard
caught `{orphanedPaths}` and degraded to an empty result; this pass makes
the handler synthesize the REAL shape:
- `git-gateway-service`: `GitExecutor.ListWorktreePaths` now parses
  `git worktree list --porcelain`'s full output (path + HEAD sha + branch,
  not just the path) into `domain.WorktreeGitInfo`. New real-git tests
  (main worktree, a linked worktree, a detached HEAD) in
  `localgit/executor_test.go` — this method had no test coverage before.
  `gitgateway.proto`'s `DetectWorktreesResponse` gained a structured
  `DetectedWorktreeGitInfo` list. The relay executor's own version keeps
  returning bare paths (Head/Branch empty) — its real Dev Server Agent
  method (`git.worktreeList`) doesn't exist at all (confirmed against
  `agent/src/relay/*.ts`), a separate pre-existing gap this doesn't touch.
- `channels_worktree.go`: `worktree.detectedList` now returns
  `{repoId, authoritative: true, source: 'git', worktrees: [...]}` built
  from a real `mergeDetectedWorktrees` merge — combining git's on-disk
  truth (path/head/branch) with project-service's bookkeeping (real id,
  `ownership: 'orca-managed'` when a `project.worktrees` row matches by
  path) or synthesizing a new id (`ownership: 'external'`) for a path with
  no bookkeeping row. Every field the frontend's rich `Worktree` type
  needs but this backend has no data for gets the exact same safe default
  `toLegacyDetectedWorktreeResult` (the frontend's own legacy-fallback
  synthesis) already established, so the two sources agree on shape.
  `displayName` derives from the branch name, falling back to the path's
  basename for a detached HEAD.
- **Live-verified end-to-end**: `repo.add` a path with no on-disk git repo
  yet → correctly errors; `git init`+commit+`worktree add` a second linked
  worktree directly in the deployed container → `worktree.detectedList`
  returns BOTH worktrees with correct real branch names, HEAD shas, and
  `isMainWorktree` flags — confirmed via the actual live API response, not
  just a unit test.
- The frontend's shape guards (`store/slices/worktrees.ts`,
  `web-preload-api.ts`) are kept as defense-in-depth (a server mid-rollout
  on an older binary, or any future regression in this synthesis, should
  still degrade safely) — their doc comments updated to say so.

**2. A live, previously-undiscovered dead branch in Create/Clone Project
for a Dev Server host selection.** Investigating why "Create new project"
still failed with `GITGATEWAY_MISSING_DEV_SERVER_ID` even with a
"Connected" Dev Server visibly selected in the dialog (despite the
Thirty-third entry's `devServerId`-threading fix) found a SECOND, deeper
bug: a Dev Server Host selection parses to `kind: 'devServer'`
(`shared/execution-host.ts`), not `kind: 'runtime'` — so
`options.runtimeEnvironmentId` (only set for `kind: 'runtime'`) stayed
empty, and `useCreateRepo.ts`/`useAddRepoCloneFlow.ts`'s own `target`
resolution fell back to `getActiveRuntimeTarget` with
`activeRuntimeEnvironmentId` UNCONDITIONALLY nulled out — always resolving
`'local'`. `target.kind === 'environment'` was the ONLY gate for the
`repo.create`/`repo.clone` RPC branch, so a devServer-kind selection always
fell through to `window.api.repos.create`/`clone` (the Electron-only local
IPC path) instead. **Confirmed this path is genuinely live in this web
deployment** (not the dead code the Thirty-third entry assumed for #5 in
its table): `window.api.repos.create`'s web implementation
(`web-preload-api.ts`) still relays to the real `repo.create` RPC, just
with the wrong param shape (`{parentPath, name, kind}` instead of
`{devServerId, destPath, defaultBranch}`) — producing the exact observed
error. Verified via a direct `window.api.runtime.call({method:
'repo.create', params: {devServerId, destPath, defaultBranch}})` in the
live browser session: it succeeds and reaches git-gateway-service
correctly, proving the 'local'-branch RPC transport itself works fine —
the bug was purely in which branch got taken, not the transport.

Fixed by widening the branch gate to `options.devServerId || target.kind
=== 'environment'`, and — since `getOrCreateDefaultProject` structurally
requires a real `{kind: 'environment'}` target — only nulling out
`activeRuntimeEnvironmentId` when NO dev server was picked (preserving the
original behavior for SSH/explicit-runtime-environment creates), so a
devServer-kind selection resolves via the real, already-persisted active
environment (web mode's "session-auth" one) instead. New regression tests
in both hooks' test files cover exactly this scenario (a devServer-kind
selection with no `runtimeEnvironmentId`/`hostId` set at all, matching
precisely what `AddRepoDialog.tsx` passes) — the existing "selected runtime
environment" tests only covered the OTHER, already-working case
(`kind: 'runtime'`) and would never have caught this.

**Live-verified end-to-end**: created a real project through "test-01"
(a genuinely connected Dev Server) via the actual dialog — `project.repos`
row created, sidebar shows the new project with its `master` branch
worktree correctly, "Project created" toast fires, zero console errors
related to this flow.

**3. A THIRD, fully independent bug this live verification surfaced**: an
`Unhandled Promise Rejection: TypeError: Cannot read properties of null
(reading 'map')`, decoded via the same sourcemap technique used earlier
this session (`vite build --sourcemap`, confirmed identical asset hash to
production, `@jridgewell/trace-mapping`) to
`lib/workspace-port-actions.ts`'s `mergeWorkspacePortScans`: a scan result
whose own fetch failed/hasn't completed can carry `ports` as
null/undefined at runtime despite `WorkspacePortScanResult` declaring it a
required array — the same "backend response is a partial record" bug
class as `repo-display-labels.ts`'s `normalizePathSegments`/
`wsl-paths.ts`'s `parseWslUncPath` guards from earlier this session. Only
ever triggered because this pass's fixes made the FIRST successful
project creation for this user possible — the newly active workspace
triggers a port scan, exposing a dormant bug nothing had ever reached
before. Fixed with the same one-line defensive-default pattern
(`scan.ports ?? []`); added `workspace-port-actions.test.ts` (this
function had no test coverage at all before), including the exact
null/undefined-ports regression case.

**Verification**: `go build`/`go vet`/`go test ./...` clean across all 17
backend-go services; new real-git tests for `ListWorktreePaths`
(previously untested) and a new `TestWorktreeDetectedListChannel_
MergesOnDiskAndBookkeeping` replacing the old orphaned-paths-only test.
Frontend: `npx tsc --noEmit`/`npx oxlint` clean on every touched file; new
regression tests in `useCreateRepo.default-checkout.test.ts`/
`useAddRepoCloneFlow.test.ts` (the devServer-kind-selection scenario) and
`workspace-port-actions.test.ts` (the null-ports scenario), all passing;
a full sweep of `src/renderer/src/components/sidebar/` plus
`worktrees.test.ts`/`web-preload-api.test.ts` shows only pre-existing,
confirmed-unrelated failures (verified via `git stash` — identical
failures, same messages, present on the unmodified branch: `use-add-repo-
host-selection.test.ts`, `Sidebar.test.tsx`, `WorktreeCardMeta.
interaction.test.tsx`, and the already-known missing `preload/gitlab.ts`
fixture). Deployed and live-verified end-to-end on b15.openledger.vn as
v0.4.56. Test data cleanup: the `project.repos`/`project.projects` rows
created during verification were deleted; the two small `/tmp/orca-
final-*` folders that pattern-fixed create actually placed on the real
"test-01" Dev Server could NOT be cleaned up remotely — `files.delete`
needs a live `infra.connections` row that doesn't exist for these ad hoc
test repos (the same pre-existing relay-connection gap the Thirty-third
entry's table item #4 already flagged, now doubly confirmed) — flagged
here rather than silently left undisclosed; a human with access to that
host can remove `/tmp/orca-final-verify-v55` and `/tmp/orca-final-v56`.

## Dependency order (as executed)

```
BE-SOL-001 (data model: status, DevServerGroup)          ✅
    │
    ├──► BE-SOL-002 (admin approve/reject + assign group)  ✅
    │        (needed common/tenant.Role — built as part of this pass)
    │
    └──► BE-SOL-003 (group ↔ department/team grants)       ✅
              │        (no separate OPA rego — see CR-DS-007 §4)
              │
              └──► BE-SOL-004 (access request flow)         ✅
```

See [tasks/](../tasks/README.md) for the task-level breakdown.
