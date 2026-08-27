# TASK-023: Document `accounts.*`'s agent-side gap and frontend `connectionId` prerequisite (blocked, no code)

**From Solution:** SOL-004
**Priority:** P3 — tracking only, blocks nothing in this repo's `backend-go` build
**Service:** `agent` (DONE this session) / `api-gateway` (DONE this session — `accounts.subscribe`) / `frontend` (still blocked — see below, narrower reason post-BUG-005)
**File:** `agent/src/relay/accounts-handler.ts` (+`getAccountsSnapshot`/`handleAccountsGetSnapshot`), `agent/src/relay/accounts-handler.test.ts`, `agent/src/relay/agent-rpc-dispatch.ts` (registers `accounts.getSnapshot`), `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go` (+`registerAccountsSubscribeChannel`), `channels_accounts_test.go` — no `frontend/` diff, see "Update (post-BUG-005)" below for why
**Depends on:** TASK-021
**Status:** `[x]` DONE — **agent-side gap CLOSED** (unchanged from prior session): `accounts.selectClaude`/`selectCodex`/`removeClaude`/`removeCodex`/`getSnapshot` are real, tested JSON-RPC methods on the Dev Server Agent's dispatcher (`agent-rpc-dispatch.ts`), `accounts.subscribe` is a real, registered `StreamHandler` in `wscompat` (`channels_accounts.go`, polling `accounts.getSnapshot`). **Frontend `connectionId` prerequisite is now CLOSED too** (see "Update (2026-08-27, picker + connectionId threading)" below): a real dev-server picker exists in `AccountsPane.tsx`, and `runtime-provider-accounts-client.ts`'s 5 `environment`-target call sites (4 mutations + `accounts.subscribe`) resolve the user's picked dev server to a live `connectionId` via a new `accounts.resolveDevServerConnection` `wscompat` channel before every call, failing fast with a clear error instead of ever sending a request `wscompat` would reject with `ACCOUNTS_NO_CONNECTION`. This closes the loop end-to-end for the transport that actually reaches `wscompat` today (`WebSessionClient`, session-cookie multi-user web — see "Update (post-BUG-005)" below); it does not and cannot touch `WebRuntimeClient`/desktop's separate pairing-protocol transport, which remains a distinct, pre-existing, out-of-scope architecture gap this task never owned (unchanged — see the "Frontend dispatch-model finding" section above).

---

## Frontend dispatch-model finding — RESOLVED, `connectionId` threading was based on a false premise

Traced `window.api.runtimeEnvironments.call` (what `runtime-provider-accounts-client.ts`'s
4 `target.kind === 'environment'` call sites actually invoke, via
`callRuntimeRpc` in `frontend/src/renderer/src/runtime/runtime-rpc-client.ts:85-90`)
all the way through the Electron main process, and confirmed it **never reaches
backend-go at all**:

- Preload bridge: `desktop/src/preload/index.ts:4033-4039` — `runtimeEnvironments.call`
  is a plain `ipcRenderer.invoke('runtimeEnvironments:call', args)`.
- Main-process handler: `desktop/src/main/ipc/runtime-environments.ts:134-148` registers
  `ipcMain.handle('runtimeEnvironments:call', ...)`, which calls
  `callRuntimeEnvironment()` in `desktop/src/main/ipc/runtime-environment-transport-routing.ts:95-152`.
- That function resolves a saved `KnownRuntimeEnvironment`'s **pairing offer**
  (`getPreferredPairingOffer`, `desktop/src/shared/runtime-environments.ts:104-116`) and
  opens a **direct WebSocket to `pairing.endpoint`**
  (`desktop/src/shared/remote-runtime-client.ts:144` and `:486`) — a raw
  device-token + Curve25519-ECDH end-to-end-encrypted JSON-RPC protocol
  (`desktop/src/shared/pairing.ts`'s `PairingOfferSchema`: `endpoint`,
  `deviceToken`, `publicKeyB64`), completely independent of any tenant/JWT
  concept. `mobile/src/transport/rpc-client.ts:152-350` and
  `frontend/src/renderer/src/web/web-runtime-client.ts:354` implement the exact
  same pairing protocol for the mobile and browser-pairing clients respectively
  — **all three of Orca's real client surfaces (desktop, mobile, web-pairing)
  use this one legacy peer-to-peer transport**, not `api-gateway`/`wscompat`.
- `backend-go`'s `wscompat` layer has **no concept of a pairing offer,
  device token, or E2EE handshake anywhere** (`grep`-confirmed zero hits for
  `deviceToken`/`PairingOffer`/`runtimeId`/`shared_control` in `backend-go/`).
  Its `Identity` (`registry.go:12-15`) is tenant/user JWT-derived, an entirely
  different auth model. `specs/backend-go/tdd/services/api-gateway.md`'s own
  "Replaces" line confirms why: `api-gateway` is a **migration target** for
  `runtime/rpc/dispatcher.ts`/`WsSessionRouter`, not something any current
  frontend build already calls — confirmed by a zero-hit grep for
  `tenantId`/`TenantID`/`grpc-gateway`/`/v1/projects` across all of
  `frontend/src` and `mobile/src`.
- Separately, this session's own reference implementation
  (`backend/src/main/runtime/rpc/methods/accounts.ts:71-92`) uses a
  `connectionId` that is a **third, unrelated concept**: `defineStreamingMethod`'s
  own per-WebSocket-connection bookkeeping key for `registerSubscriptionCleanup`,
  not `infrafleetv1.RelayRequest.ConnectionId`. Three different subsystems
  (`environmentId`, `backend/`'s per-socket `connectionId`, and
  `infra-fleet-service`'s connection-registry `connectionId`) happen to share
  vocabulary but share no state — aliasing any pair together would be a
  purely cosmetic edit that fixes nothing real.

**Conclusion:** this confirms Step 0's option (b), and goes further than SOL-004/this
task originally assumed — the issue is not merely "`accounts.*` calls are missing
a `connectionId` field," it is that **`runtime-provider-accounts-client.ts`'s
`environment` branch is wired to an entirely different backend
(the legacy pairing-protocol dev-server/agent listener) than the one
`channels_accounts.go` relays through (`infra-fleet-service` → agent's
`agent-rpc-dispatch.ts` JSON-RPC-over-WebSocket surface, reached only via
`infra-fleet-service`'s gRPC `Relay`/`Exec`, per `agent_proxy_routes.go`'s doc
comment)**. Threading a `connectionId` into `runtime-provider-accounts-client.ts`'s
params would produce a value the pairing-protocol IPC bridge has nowhere to
put and the receiving dispatcher has no code to read — indistinguishable from
not making the change at all. Making `accounts.*` actually reachable from a
real Orca frontend build requires ONE of: (a) a genuinely new `frontend/` code
path that opens a tenant/JWT-authenticated connection to `api-gateway`'s
`wscompat` WS endpoint (which does not exist in `frontend/`, `desktop/`, or
`mobile/` today, for ANY namespace, not just `accounts.*`) — a real feature,
not a one-line param add; or (b) `infra-fleet-service`/`wscompat` growing a
bridge into the legacy pairing protocol, which is a redesign of `Relay`'s own
connection-resolution model, also out of this task's (and SOL-004's) scope.
Neither is a "just thread a field through" fix, so no `frontend/` code was
touched — see TASK-036's identical honesty standard for precedent.

---

## Update (2026-08-27, post-BUG-005) — the "no client build ever reaches wscompat" premise is now partly outdated

The "Frontend dispatch-model finding" above was traced BEFORE
[BUG-005](../../api-v1/BUG-005-websessionclient-dialect-dropped.md) was
found and fixed. That finding's option (a) — "a genuinely new `frontend/`
code path that opens a tenant/JWT-authenticated connection to `api-gateway`'s
`wscompat` WS endpoint... does not exist in `frontend/`, `desktop/`, or
`mobile/` today, for ANY namespace" — is no longer accurate for one of the
three transports it grouped together:

- `frontend/src/renderer/src/web/web-session-client.ts`'s `WebSessionClient`
  (used by `getClientForEnvironment` for `ORCA_MULTI_USER` session-cookie web
  deployments) already opens exactly this connection — `ws(s)://<host>/ws`,
  the same endpoint `wscompat`'s `Handler.ServeHTTP` answers, authenticated
  via the `orca_session` cookie at WS-upgrade time. BUG-005 found it was
  sending a message shape (`{id,authToken:'cookie-auth',method,params}`)
  `wscompat` silently dropped, and fixed `wscompat` to recognize and bridge
  it (Phase 1: plain calls; Phase 2: streaming/subscribe pushes, both now
  merged). This was NOT a `frontend/` change — `WebSessionClient` was
  already doing the right thing; `wscompat` was the side that couldn't
  understand it.
- `frontend/src/renderer/src/web/web-runtime-client.ts`'s `WebRuntimeClient`
  (E2EE pair-code mode) and `desktop/`'s Electron-IPC pairing transport are
  UNCHANGED by BUG-005 — those really are a structurally different protocol
  (device-token + Curve25519-ECDH, no session cookie), and remain outside
  `wscompat` entirely. The dispatch-model finding's grep evidence
  (`deviceToken`/`PairingOffer` absent from `backend-go/`) still stands for
  these two.

**Net effect on this task:** for the `WebSessionClient` transport
specifically, `wscompat`'s `accounts.*` channels (including the now-real
`accounts.subscribe`, see below) are reachable end-to-end EXCEPT for the one
piece SOL-004 originally flagged: `runtime-provider-accounts-client.ts`'s
`environment`-target call sites still send `{accountId}` (or, for
`accounts.subscribe`, no params object at all) — no `connectionId`. That
frontend change is real, scoped, and no longer premised on a dead-end
transport — but it is still a `frontend/` product/engineering decision
(where does that `connectionId` come from — `getActiveRuntimeTarget`'s
`environmentId` is NOT the same value as `infrafleetv1.RelayRequest`'s
`connectionId`; resolving one to the other needs a real lookup this task
does not have the authority or context to invent) — not fabricated here,
per this session's standing honesty rule.

`accounts.subscribe` is now implemented for real (not left for "a third,
later item" as originally noted below): `channels_accounts.go` registers it
as a `StreamHandler` that fetches an initial snapshot via the agent's new
read-only `accounts.getSnapshot` method, then polls every
`accountsSubscribePollInterval` (30s) and pushes a `{type:'snapshot',...}`
update only when the snapshot actually changes — a poll-based bridge, per
SOL-004's own "or a poll-based wscompat bridge" alternative, since a bare
remote host has no change-notification/fs-watch infrastructure to drive a
real push. `agents/src/relay/accounts-handler.ts`'s `getAccountsSnapshot`
and `channels_accounts_test.go`'s 3 new tests cover this.

---

## Context

TASK-021 makes `accounts.*` relay correctly through
`infra-fleet-service`'s existing `Relay` RPC, but per SOL-004 two
prerequisites outside `backend-go`'s scope must land before those 4
channels do anything but fail:

1. **Agent-side companion work (small).** `Relay` only forwards
   `method`/`params` to whatever the Dev Server Agent's JSON-RPC dispatcher
   already implements. `accounts.selectClaude`/`accounts.selectCodex`/
   `accounts.removeClaude`/`accounts.removeCodex` do not exist as JSON-RPC
   methods on the agent yet — until they do, TASK-021's handlers return a
   "method not found" error from the agent's own dispatcher. Per
   `specs/backend-go/tdd/architecture/08-inter-service-communication.md`'s
   "Talking to the Dev Server Agent" section, `agent/` changes are
   explicitly out of scope for the Go rewrite of `backend/`. SOL-004 notes
   this is comparatively low-risk agent-side work: filesystem read/write
   against a known `~/.claude`/`~/.codex` config path, well within the
   agent's existing filesystem capability — not a new execution-plane
   capability class (contrast SOL-006's browser-driving gap, TASK-036).

2. **Frontend `connectionId` prerequisite.** `Relay` requires a
   `connectionId`, but every documented `accounts.*` call site passes only
   `{ accountId }`. `runtime-provider-accounts-client.ts` already calls
   `getActiveRuntimeTarget(settings)` client-side before each of these 4
   calls — the natural fix is threading that already-resolved
   environment's `connectionId` into the RPC params alongside `accountId`.
   This is a `frontend/` change, not a `backend-go` one:
   `wscompat`'s `Identity` (api-gateway/internal/usecase) carries only
   `TenantID`/`UserID`, with no session-scoped connection to derive a
   `connectionId` from, and inventing a resolution heuristic (e.g. "the
   tenant's only connection") would silently break multi-environment
   tenants.

This task exists so both gaps are tracked explicitly rather than
discovered later as "accounts.* still doesn't work after TASK-021/022
shipped" — it produces no `backend-go` diff.

---

## What to do

1. **agent-side — DONE this session:** `accounts.selectClaude`/`selectCodex`/
   `removeClaude`/`removeCodex` are implemented in
   `agent/src/relay/accounts-handler.ts` and registered in
   `agent/src/relay/agent-rpc-dispatch.ts`. Scope, per the module's own doc
   comment: a remote Dev Server host has no Electron-userData "managed
   accounts" store and no interactive-login capability (add/reauthenticate
   intentionally stay desktop-only, per `rpc/methods/accounts.ts`'s own
   comment), so this models the host's already-authenticated CLI session as a
   single pseudo-account (fixed id `'host'`) derived by reading
   `~/.claude/.credentials.json` + `~/.claude.json`'s `oauthAccount` and
   `~/.codex/auth.json` (JWT `id_token` claims) — no CLI subprocess spawn, no
   keychain access. Empty-config-file and remove-nonexistent-account cases are
   covered by `agent/src/relay/accounts-handler.test.ts` (21 tests).
2. **frontend-side — still blocked, scope now narrower (see "Update
   (post-BUG-005)" above):** for the `WebSessionClient` (session-cookie
   multi-user web) build target, `wscompat` is now a real, reachable
   backend — the remaining gap is exactly the `connectionId` SOL-004
   originally identified, in `runtime-provider-accounts-client.ts`'s 5
   `environment`-target call sites (4 mutations + `accounts.subscribe`).
   File (or link) a tracking issue titled "Thread a real `connectionId`
   into `accounts.*`'s `environment`-target call params" — scope: resolve
   `getActiveRuntimeTarget`'s `environmentId` to the `connectionId`
   `infrafleetv1.RelayRequest` needs (these are NOT the same value; the
   resolution path doesn't exist yet and isn't obvious from this task's
   context — a real design decision, not a mechanical rename). For
   `WebRuntimeClient`/desktop's pairing transport, the original, broader
   finding still holds unchanged: no bridge into `wscompat` exists for
   those at all — that remains the bigger, separate architecture question,
   track separately if/when it's picked up.

`accounts.subscribe` is DONE, not a future item — see "Update
(post-BUG-005)" above.

---

## Verify

Agent-side (done): `cd agent && npx vitest run src/relay/accounts-handler.test.ts`
— 21/21 passing. `cd agent && npx tsc --noEmit` — zero errors attributable to
`accounts-handler.ts` or the `agent-rpc-dispatch.ts` registration (pre-existing,
unrelated errors elsewhere in `agent/` are untouched by this change).

Frontend-side (was open, now closed — see below): the tracking-issue framing
below predates the "Update (2026-08-27, picker + connectionId threading)"
section; superseded, kept for history.

---

## Update (2026-08-27, picker + connectionId threading) — frontend prerequisite CLOSED

Implemented the resolution path this task's "Open prerequisite" section
flagged as a real design decision needing explicit product input: the user
now picks which dev server owns the `connectionId` `accounts.*` relays
against, rather than `wscompat` guessing.

**`backend-go` (api-gateway/wscompat):** `channels_accounts.go` gains
`accounts.resolveDevServerConnection` — given `{devServerId}`, calls
`client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerId})`
(the same RPC `registerBrowserRelay`/`channels_browser.go` already uses for
its `worktree_id` variant — no new proto/infra-fleet-service work) and
returns `{connected, connectionId}`. `Connected: false` is a normal,
non-error result (a picker needs to display "not connected right now", not
throw). 4 new tests in `channels_accounts_test.go`
(`TestAccountsResolveDevServerConnection_Connected` / `_NotConnected_NotAnError`
/ `_MissingDevServerID_FailsFastWithoutCallingResolve` /
`_UnknownDevServerID_ResolveErrorPassesThrough`), following the file's
existing `fakeAccountsRelayClient` pattern (extended with a
`resolveConnectionFunc` field, mirroring `channels_browser_test.go`'s
`fakeBrowserRelayClient`).

**`frontend`:** new `runtime/accounts-dev-server-connection.ts` owns (a) the
picked-`devServerId` preference — a plain localStorage read/write scoped per
`environmentId` (`orca.accountsDevServer.<environmentId>`), following
`landing-preflight-dismissal.ts`'s existing "module of plain functions
wrapping try/catch'd localStorage" convention (no new persistence mechanism,
no `GlobalSettings`/backend-go table — this is a local UI convenience, not
server-authoritative state), and (b) `resolveAccountsDevServerConnection`,
a thin `callRuntimeRpc` wrapper around the new channel. Resolution is
deliberately NOT cached: accounts.* mutations are infrequent, user-initiated
clicks, so a fresh resolve-per-call is the honest baseline — caching live
connection state invites staleness bugs for marginal benefit here.
`requireAccountsDevServerConnectionId` composes both steps and throws a
clear `Error` (no dev server picked / not connected) BEFORE any `accounts.*`
RPC is attempted.

`runtime-provider-accounts-client.ts`'s 4 mutation functions and
`watchProviderAccounts`'s remote branch all call
`requireAccountsDevServerConnectionId(target)` first and thread the
resolved `connectionId` into the RPC params (`{accountId, connectionId}` for
mutations, `{connectionId}` for `accounts.subscribe`, sent via `subscribe`'s
existing `params` field). A new `AccountsDevServerPicker.tsx` component
(rendered as the first section in `AccountsPane.tsx`, unconditionally
whenever `isRemoteAccountScope`, not gated by the settings search filter
since it determines whether every other remote account action even runs)
lists dev servers via the already-existing `devServer.list` channel (reusing
the exact `callRuntimeRpc(target, 'devServer.list', null)` call shape already
used by `CreateProjectDialog.tsx` — no new backend channel needed for
listing), persists the pick, and shows explicit "no dev servers available" /
"not currently connected" states. Its `onReadyChange` callback widens
`AccountsPane`'s existing `accountRuntimeUnavailable` flag — already the
single condition every select/remove button's `disabled` prop checks — so
picking no dev server (or an unconnected one) disables every account
mutation control via that one existing choke point, without threading a
second condition through each of the ~8 button call sites individually.
`StatusBar.tsx`'s account-switching UI needed no changes: it calls the same
`runtime-provider-accounts-client.ts` functions, so it inherits the same
fail-fast behavior and shares the same localStorage preference the
`AccountsPane` picker sets.

**Verify (this update):**
- `cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./...` — clean (all packages `ok`).
- `cd frontend && npx vitest run src/renderer/src/runtime/runtime-provider-accounts-client.test.ts src/renderer/src/components/settings/AccountsPane.test.tsx src/renderer/src/components/settings/AccountsDevServerPicker.test.tsx` — 27/27 passing.
- `cd frontend && npx tsc --noEmit -p tsconfig.json` — zero new errors in any file this update touched (verified by diffing the touched-file set against the full pre-existing error list, which has ~70 unrelated pre-existing errors across the codebase).
- `cd frontend && npx vitest run src/renderer/src/runtime/ src/renderer/src/components/settings/` — 968 passing / 3 pre-existing failures in `settings-setup-guide-progress-hook.test.tsx` (confirmed via `git stash` to fail identically on the unmodified branch tip — unrelated to this change, a stale `9` vs `8` setup-step count).

**What remains genuinely open (unchanged, out of this task's scope):**
`WebRuntimeClient` (E2EE pair-code mode) and desktop's Electron-IPC pairing
transport still have no bridge into `wscompat` at all — `accounts.*` calls
made over those transports still never reach `api-gateway`. This is the
same structurally-separate-protocol gap the "Frontend dispatch-model
finding" section above already identified and explicitly scoped out; nothing
in this update changes that, and it was never this task's prerequisite to
fix.
