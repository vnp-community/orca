# TASK-023: Document `accounts.*`'s agent-side gap and frontend `connectionId` prerequisite (blocked, no code)

**From Solution:** SOL-004
**Priority:** P3 — tracking only, blocks nothing in this repo's `backend-go` build
**Service:** `agent` (DONE this session) / `frontend` (still blocked — see below, now for a definitively-confirmed reason)
**File:** `agent/src/relay/accounts-handler.ts` (new), `agent/src/relay/accounts-handler.test.ts` (new), `agent/src/relay/agent-rpc-dispatch.ts` (registers the 4 methods) — no `frontend/` diff, see "Frontend dispatch-model finding" below for why
**Depends on:** TASK-021
**Status:** `[partial]` — **agent-side gap is now genuinely CLOSED**: `accounts.selectClaude`/`selectCodex`/`removeClaude`/`removeCodex` are real, tested JSON-RPC methods on the Dev Server Agent's dispatcher (`agent-rpc-dispatch.ts`), so `infra-fleet-service`'s `Relay` RPC no longer hits "method not found" for these 4 methods. **Frontend-side is still blocked, but the original "just thread `connectionId` into `runtime-provider-accounts-client.ts`" framing is now known to be the WRONG fix** — see "Frontend dispatch-model finding" below for the traced evidence. No `frontend/` code was changed; fabricating a `connectionId` param would not connect anything real (see below), so per this session's honesty standard (same as TASK-036's resolution) the correct outcome is leaving `frontend/` unchanged and documenting the real blocker here instead of a cosmetic no-op edit.

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
2. **frontend-side — still blocked, now for a proven reason (not a missing
   param):** see "Frontend dispatch-model finding" above. File (or link) a
   tracking issue titled "Give `accounts.*` (and `wscompat`'s other 261
   namespaces) a real frontend caller" — the actual gap is that no Orca
   client build (desktop, mobile, or web-pairing) opens a tenant/JWT WS
   connection to `api-gateway`'s `wscompat` server at all today; `accounts.*`
   is not a special case of this, it is the general case. This is a
   product/architecture decision (build the `wscompat` caller, or bridge
   `Relay` into the legacy pairing protocol), not an engineering task with an
   obvious scope — do not silently pick one on a future pass.

Also note for whoever picks up either: SOL-004 separately flags
`accounts.subscribe` (a streaming variant) as explicitly out of scope for
its own 4-method list — track that as a third, later item if/when it's
picked up, not part of either issue above.

---

## Verify

Agent-side (done): `cd agent && npx vitest run src/relay/accounts-handler.test.ts`
— 21/21 passing. `cd agent && npx tsc --noEmit` — zero errors attributable to
`accounts-handler.ts` or the `agent-rpc-dispatch.ts` registration (pre-existing,
unrelated errors elsewhere in `agent/` are untouched by this change).

Frontend-side (still open): N/A — no code produced, by design (see "Frontend
dispatch-model finding" above). "Done" for this half means a tracking issue
exists per item 2 above, so the real (architecture-level) blocker is
discoverable instead of a future pass silently re-attempting the
already-disproven "just add `connectionId`" fix.
