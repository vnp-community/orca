# Execution Plan — From Scaffold to Production

**Status of this document:** grounded in the actual code in this repository
as of the scaffold build — every task below comes from a real gap found
while implementing a service (recorded in that service's own README's
"Known gaps" section), not a generic template. Cross-reference
[`specs/backend-go/migration/ts-to-go-migration-strategy.md`](../../specs/backend-go/migration/ts-to-go-migration-strategy.md)
for the TS-cutover framing this plan's phase numbers reuse; this document
is the scaffold-aware continuation of that plan, not a replacement for it.

## How to use this document

1. **Cross-cutting epics** (§2) block multiple services at once — do these
   first, in the order listed, or a service-by-service push will keep
   re-discovering the same missing piece (the Dev Server Agent relay client
   alone blocks 3 services).
2. **Per-service task lists** (§3) are grouped by the same 5 migration
   phases the strategy doc defined, since the dependency reasoning ("do
   `auth`/`tenant` last, everything depends on them") still holds for
   finishing the scaffold, not just for a TS cutover.
3. Every task lists which service's README it came from, so you can go
   read the exact stub/comment in the code before starting.
4. A service isn't done until it clears
   [`standards/production-readiness-checklist.md`](../../specs/backend-go/standards/production-readiness-checklist.md)
   — §3 tracks feature-completeness, not that checklist's operational bar
   (observability, chaos testing, etc.), which every service still needs
   regardless of which phase it's in.

---

## 0. Frontend compatibility layer (`wscompat`) — added post-deploy, 2026-08-17

**Why this exists:** deploying `backend-go` to `172.20.2.39` (replacing the
TS backend) surfaced that the deployed `frontend/` build is **unmodified**
— its RPC client
(`frontend/src/platform/adapters/web/rpc-client.ts`) speaks a legacy
channel-based protocol over `wss://<host>/ws`, not `api-gateway`'s REST/gRPC
surface. `api-gateway/internal/adapter/wscompat/` is a compatibility shim
that speaks that exact same wire protocol and translates each named
channel into a real backend-go gRPC call, so the existing frontend keeps
working without modification while backend-go services gain real
implementations behind it.

**Wire protocol** (verified against the real client, not guessed):
`{id,type:"invoke",channel,args}` → `{type:"result"|"error",id,...}`,
plus `{type:"send",channel,data}` (fire-and-forget) and
`{type:"push",channel,args}` (server→client, not wired to any event source
yet). Auth: `orca_session` cookie, validated once at WS-upgrade time via a
**real** `auth-service.ValidateSession` call (`internal/adapter/authclient`)
— not the placeholder unverified-JWT `AuthValidator` used elsewhere. A new
`POST /auth/local` route (real `auth-service.Login` call, sets the cookie)
makes login possible at all — it didn't exist before this pass.

**Channel coverage — 262 methods across 36 namespaces exist
(`specs/frontend/api/rpc-catalog.md`). This pass wires real handlers for
9**: `annotation.create/list/update/delete`, `task.create/get`,
`git.status/diff`, `automation.runNow`. Every other channel returns a
clean, protocol-correct `{type:"error",message:"channel %q is not yet
implemented..."}` instead of the pre-fix behavior (nginx's SPA fallback
silently returning `200 text/html` to a WebSocket upgrade request — the
originally-reported bug). **Arg shapes for the 9 wired channels are
best-effort** (decoded as a single JSON object per proto request fields) —
not verified against every real frontend call site's actual argument
marshaling; verify before depending on one in production.

**Update, same day — bootstrap admin + real cookie auth on REST routes,
both closed and verified end-to-end through the live domain:**
- `auth-service` now has a first-boot bootstrap (`internal/usecase/bootstrap.go`,
  runs once at startup, not an RPC): if `BOOTSTRAP_TENANT_ID`+`BOOTSTRAP_ADMIN_EMAIL`
  are set and zero users exist anywhere, creates the first admin —
  auto-generates and logs the password once if `BOOTSTRAP_ADMIN_PASSWORD`
  is unset, mirroring the old TS backend's `ORCA_ADMIN_EMAIL`/`ORCA_ADMIN_PASSWORD`
  behavior. Verified: `POST /auth/local` → `200` + real session cookie →
  `GET /ws` with that cookie → **`101 Switching Protocols`** (was `200`
  pre-fix, then `401` after the first fix, now a real success) —
  through `https://b15.openledger.vn`, not just direct-to-container.
- `api-gateway`'s REST `authMiddleware` (`/v1/*` routes) had the *same* bug
  wscompat did — `usecase.AuthValidator` parsed the cookie as an unverified
  JWT, which a real raw session token can never satisfy, so every
  cookie-authenticated REST call was silently 401ing. Now tries the real
  `authclient.SessionValidator` first, falling back to the JWT placeholder
  only for bearer tokens (mobile/CLI — still not production-safe, see
  `usecase.AuthValidator`'s doc comment). Verified: `/v1/usage/sessions`
  with a real cookie now passes auth (reaches the handler) instead of 401.

**Known gap this doesn't close:** `auth-service.CreateUser` (the *ongoing*
admin-console path, as opposed to the one-time bootstrap above) still
requires an existing admin caller — correct by design, not a gap. What IS
still a gap: no self-service signup, invite, or password-reset flow exists
at all past the single bootstrap admin — every additional user still needs
the bootstrap admin to call `CreateUser` on their behalf and hand them a
generated password out-of-band (`CreateUser`'s own doc comment already
flags this — the proto has no password/invite field). Needs either a
first-run-setup RPC (matches the old TS backend's `first-run-setup.ts`,
never ported) or a real invite/reset flow.

**Second update, same day — the ACTUAL root cause, found by reading
frontend/'s bootstrap sequence line by line, not guessed:** the two fixes
above were necessary but not sufficient — the browser was still stuck in
an infinite WS-retry loop after them. Root cause: `wscompat.Handler`
rejected an unauthenticated `/ws` upgrade with an **HTTP 401 before the WS
handshake completed**. That's incompatible with how
`frontend/src/renderer/src/web/main-web-bootstrap.tsx`'s `bootstrapWebApp()`
works: it `await`s `WebSocketRpcClient.connect()` — which resolves on
`ws.onopen`, i.e. on a successful handshake — **before ever rendering
`WebRootBoundary`**, the component that checks `/auth/me` and shows
`LoginPage`. Rejecting the handshake itself made `connect()` reject,
`bootstrapWebApp` retries 3 times then renders a hard "Cannot connect to
Orca backend" screen — **the user could never reach the login page at
all**, regardless of credentials.

The old TS backend never had this problem because
`backend/src/main/session/ws-session-router.ts` doesn't do this either:
`WsSessionRouter.handleConnection` **always completes the WS handshake**,
then closes with WebSocket application close code **`4401`**
(`ws.close(4401, 'Authentication required. Please log in first.')`) if
unauthenticated — an app-level close, not an HTTP-level rejection.
`wscompat.Handler` now does the same (`wsCloseAuthRequired = 4401` in
`handler.go`, with the exact matching reason string) — verified: `GET /ws`
without a cookie now returns `101 Switching Protocols` *then* a close
frame carrying `4401`/"Authentication required...", both through
`https://b15.openledger.vn` directly, not a shortcut test.

**Same investigation also found `/auth/local`'s response shape was wrong**
— it nested the user under `{"user": {...}}` and omitted `role`/`provider`,
but `frontend/src/renderer/src/auth/auth-types.ts`'s `AuthUser` (what
`loginLocal()` does `return body as AuthUser` against) is a **flat**
object with those two fields required. Fixed, and `GET /auth/me` /
`GET /auth/config` / `POST /auth/logout` — previously nonexistent, silently
falling through to the SPA catch-all — are now real routes matching
`specs/frontend/api/http-endpoints.md` exactly. Verified end-to-end through
the real domain: `/auth/config` → real JSON; `/auth/me` → clean `401`
without a cookie, the correct flat `AuthUser` shape with one; login →
same shape, cookie set; that cookie then works for both `/auth/me` and the
`/ws` upgrade.

**Lesson for whoever wires the next channel/route in `wscompat`/`httpgateway`:**
matching a backend contract by proto-field-name convention (what the
earlier passes did) isn't enough when a real, already-built frontend
client exists — read that client's actual source before assuming a
request/response shape or a protocol-level behavior (HTTP status vs.
WS close code) is "close enough." Both bugs here were exactly this kind of
mismatch, not missing functionality.

**Remaining work to reach real frontend parity:** wire the other ~253
channels, in priority order matching §3's phase table below (a channel is
only worth wiring once the backend-go service behind it has moved past
"stub" — wiring a channel to a stub just changes the error message, not the
outcome). `internal/adapter/wscompat/channels.go`'s doc comment is the
single source of truth for current coverage — update it as channels are
added, don't let this section drift from the code.

## 1. Current state

19/19 modules (`common`, `proto`, 17 services) build, vet, and test clean
under one `go.work` workspace. See [`../README.md`](../README.md)'s "What's
genuinely real" / "What's intentionally stubbed" sections for the full
current-state summary — not repeated here.

---

## 2. Cross-cutting epics (do these first — they unblock multiple services)

### Epic A — Dev Server Agent relay client (`infra-fleet-service`)

**Why first:** the single most-repeated stub in this codebase. Blocks:
`git-gateway-service` (relay-path Push/Pull/Commit when `connected=true`),
`workflow-service` (Agent/Shell/Notification step executors), and indirectly
`task-service`'s `SimpleExecutor`/`ComplexExecutor` and `project-service`'s
future `agentSpawn`-equivalent path.

- [x] Decide Option A vs. B — confirmed Option A (preserve the TS wire
      protocol), consistent with `wscompat`'s earlier decision for the
      frontend-facing side.
- [x] **relay-websocket mode implemented for real** (2026-08-17) — see
      "Epic A, relay-websocket pass" below for the full writeup.
- [x] **Background auto-reconnect for relay-websocket sessions** (2026-08-17,
      second pass — see "Epic A, second pass" below).
- [x] **`infra.connections`/`infra.port_forwards`/`infra.provider_registry_entries`
      tables** (2026-08-17, second pass) — `connections` is a real routing
      model with a write path (`CreateConnection`); the other two are schema
      only, no consumer yet.
- [x] **Wire `git-gateway-service`'s `grpcclient.RelayExecutor` +
      `ConnectionResolver`** (2026-08-17, second pass) to the now-real
      `infra-fleet-service.ResolveConnection` + a new generic `Relay` RPC.
- [x] **Wire `workflow-service`'s `AgentStub`/`ShellStub`/`NotificationStub`**
      (2026-08-17, second pass) to the same `Relay` RPC — `stub.go` deleted.
- [x] **Wire `devServer.list`/`devServer.add`/`fleet.health.checkAll`
      channels into `api-gateway`'s `wscompat` registry** (2026-08-17, second
      pass) — the subset with a real backing RPC;
      `devServer.connect`/`disconnect`/`remove`/`testConnection` still have
      no backing RPC and remain `notImplementedHandler`.
- [x] **`direct-websocket` mode implemented for real** (2026-08-17, third
      pass — see "Epic A, third pass" below) — inbound `AgentWebSocketServer`-
      equivalent (`internal/adapter/agentwsserver`), `POST/GET /api/agent-token`
      HTTP endpoint, SHA-256-hashed single-use token slots.
- [x] **`relay-ssh` mode implemented for real** (2026-08-18, fourth pass —
      see "Epic A, fourth pass" below) — closes the "no `relay.js` build
      artifact reachable from backend-go's build" gap the third pass left
      open, by adding a THIRD connection mode (`stdio`) to `agent/` itself
      (this repo's actual buildable Dev Server Agent — `agent/out/agent.js`,
      the same artifact `direct-websocket`/`relay-websocket` already use)
      rather than chasing the originally-spec'd, separately-built
      `relay.js`/Unix-socket-daemon design, which has no buildable
      counterpart anywhere in this repo. `internal/adapter/sshrelay`
      (new) SFTP-deploys `agent/out/agent.js` over a real
      `sshconn`-established SSH connection, launches it as
      `node agent.js --stdio`, and completes the receiver-side
      `agent.handshake` exchange — `devserveragent.Client` gained a
      `Transport` interface (relay-websocket/direct-websocket's existing
      WebSocket transport generalized behind it, zero behavior change) so
      `Exec`/`Health` are now genuinely uniform across all three modes, no
      relay-ssh-specific branch left in either.

### Epic B — Wire `credential-broker-service` into its 4 consumers ✅ done 2026-08-17

**Why:** `credential-broker-service` itself is real (Vault-backed, audited,
tested). What's stubbed is everyone else's *client* to it.

- [x] `ai-provider-service`: replaced `internal/adapter/grpcclient/credential_broker_client.go`'s
      local-only `CredentialRef` synthesis with a real
      `credentialbrokerv1.NewCredentialBrokerServiceClient` call. This is
      where TS Gap 2's actual fix lands.
- [x] `scm-integration-service`: replaced `internal/adapter/credentialbroker/stub.go`
      (deleted, `client.go` in its place).
- [x] `issue-tracking-service`: replaced `internal/adapter/credential/stub_resolver.go`
      (deleted, `client.go` in its place).
- [x] `notification-service`: routed VAPID Transit signing through
      `credential-broker-service` (new `SignVapidPayload` RPC) instead of
      calling `common/secrets` directly — closes the real design-doc
      inconsistency this item flagged (see §9 below for the full story,
      including the design doc's own contradiction this surfaced).

See §9 below for what each fix required in practice — two of the four
consumers needed real proto/broker-side additions, not just a client swap.

### Epic C — Proto surface gaps found while implementing (not just "add more RPCs later") ✅ done 2026-08-17

These are cases where a service's real logic needed a cross-service RPC
that doesn't exist yet — discovered by actually writing the code, not
guessed in advance:

- [x] **`workflow-service` + `task-service`: add `HasActiveExecutions`.**
      `project-service.RebindDevServer`'s active-execution guard (the fix
      for the TS `PROJECT_HAS_ACTIVE_WORKFLOWS` gap) is no longer a no-op.
      Both protos gained the RPC, both services implement it for real
      (workflow-service's is fully accurate; task-service's has one
      documented, honest limitation — see §10 below), and
      `project-service/internal/adapter/grpcclient/`'s two stub checkers
      now call the real RPCs.
- [x] **`orchestration-service`: extend the proto.** `CreateDispatchContextRequest`
      now carries `orchestration_task_id`, `CreateGateRequest`/`DecisionGate`
      now carry `question`/`options` — `CreateGate` succeeds for real once
      a dispatch context is created with a task id, instead of always
      returning `FailedPrecondition`. The `handle`-field claim in this
      item's original wording was already stale by the time this pass
      started: `DispatchContext`/`CreateDispatchContextRequest` already had
      `handle` — only `orchestration_task_id`/`question`/`options` were
      actually missing. See §10 below for what else this surfaced.
- [x] **`auth-service`: decide the RPC surface — done 2026-08-17, no new RPCs
      built.** Investigated all three named gaps individually against
      `specs/backend-go/services/auth-service.md` (see that service's
      README "Known gaps" for the full per-item reasoning, not repeated
      here): **first-run setup** — not needed as an RPC, already closed by
      the `Bootstrap` startup routine (env-var-driven, shipped earlier this
      session) solving the same underlying problem a different way.
      **Access-policy CRUD** and **refresh-token flow** — both genuinely
      missing, both correctly deferred (not silently dropped): the former
      needs Epic E's OPA instance to exist first (CRUD for data nothing
      consumes is speculative), the latter needs Epic D's real JWT signing
      first (rotation plumbing on top of a token type that doesn't
      cryptographically exist yet would mean faking or later reworking it).
      This is Epic C's "these were never needed" outcome for one item and
      "correctly sequenced after its real prerequisite" for the other two —
      a real decision, not a non-decision.
- [x] **`workflow-service`: `ListTemplates`/`ResolveTemplate`/`CancelExecution`
      — done 2026-08-17, on explicit user request to build it now.**
      Template inheritance (`parent_template_id`, `ResolveTemplate`'s
      `WITH RECURSIVE` chain walk, depth<=5) is real, including a
      deliberate, documented resolution policy (closest ancestor with any
      steps wins) since workflow-service.md §6 specifies the query shape
      but not a merge policy. `ListTemplates` is keyset-paginated.
      `CancelExecution` was independent of inheritance and shipped
      alongside it. See §11 below for the full writeup, including two
      pre-existing, never-actually-run integration tests this pass found
      and fixed along the way.

### Epic D — JWT/JWKS chain (auth-service ↔ api-gateway) ✅ done, found already-landed 2026-08-18

Two ends of the same gap, should land together:

- [x] `auth-service`: implement `IssueServiceToken` for real — Vault
      Transit-backed signing (per `credential-broker-service`'s Transit
      adapter as the reference pattern), publish a JWKS endpoint.
- [x] `api-gateway`: replace `AuthValidator`'s unverified-claims parsing
      (`internal/usecase/validate_identity.go`) with a real JWKS client
      that verifies the signature against `auth-service`'s published keys.
      **Do not deploy `api-gateway` publicly before this lands** — every
      README involved flags this loudly, repeating it here because it's
      the one gap with direct security exposure to the public internet.

Both sides were already implemented (`common/jwtauth`, `internal/adapter/vault.TokenSigner`,
`internal/adapter/authclient.JWKSClient`) by a concurrent session by the time
§16 below started — confirmed real (not aspirational) via a clean
`go build`/`go test` across `common`, `auth-service`, `api-gateway`, not
just re-reading this checklist. See §16 for what was verified/closed on
top of it.

### Epic E — OPA policy bundle ✅ done, found already-landed 2026-08-18

Every service that needs authorization beyond "is this the right tenant"
now calls a real, embedded OPA evaluator (`common/policy.Evaluator`)
against a real `policy/orca-authz/*.rego` bundle, in place of the
placeholder inline checks this section originally described.

- [x] Stand up an OPA instance (sidecar or embedded via the Go SDK, per
      [`architecture/07-security-architecture.md`](../../specs/backend-go/architecture/07-security-architecture.md)).
      Landed as embedded (in-process, no sidecar) — matches the design
      doc's per-service option, not the `api-gateway` coarse-grained
      sidecar option, which remains unbuilt (see below).
- [x] Write the `orca-authz` Rego bundle's first policies: admin-action
      checks (replacing `auth-service`'s placeholder — `admin.rego`,
      replacing `requireAdminActor`'s old inline check), task-grant final
      decision (`task_grant.rego`, consuming `task-service`'s BFS output as
      OPA input), annotation author-only edit/delete (`annotation.rego`,
      real and enforced — `annotation-service`'s README no longer flags
      this as unenforced, though the admin-override branch is unreachable
      until `actor_role` is propagated into that service's context, a
      separate known gap).
- [ ] Add `opa test` to CI once the bundle exists. The bundle exists and
      `make opa-test` runs it locally; wiring it into an actual CI pipeline
      is still blocked on §5's "CI pipeline... none of this is wired yet"
      item, not specific to this epic.

**Not done, deliberately out of this epic's scope:** `api-gateway`'s own
coarse-grained "can this JWT call this endpoint" OPA check (design doc §9)
— still unbuilt, only `auth-service`/`task-service`/`annotation-service`
call OPA today. Bundle hot-reload (design doc wants it; the current
evaluator needs a service restart on a policy edit).

### Epic F — Horizontal-scaling blockers ✅ done 2026-08-17

Two services currently only work correctly at 1 replica:

- [x] `tenant-service`'s `GetResolvedProfile` cache is in-process LRU+TTL —
      fine at 1 replica, silently stale/inconsistent at >1. Decide: shared
      cache (Redis) vs. accept eventual consistency vs. no cache (measure
      first). **Decided: keep in-process LRU+TTL (Redis rejected), close
      the correctness gap with a best-effort NATS invalidation broadcast**
      — see §12 below.
- [x] `notification-service`'s `Broadcaster` is in-process channel fan-out —
      a `StreamNotifications` subscriber on replica A never sees an event
      broadcast on replica B. Fix via either sticky routing at
      `api-gateway` or republishing every broadcast to a shared NATS
      subject every replica also consumes (the service's own README
      recommends evaluating both). **Decided: neither literally — fixed at
      the NATS consumer-group layer instead (per-replica ephemeral
      consumers), achieving the same effect as "every replica consumes
      every broadcast" without a new subject or a sticky-routing
      requirement on api-gateway** — see §12 below.

### Epic G — Transactional outbox ✅ built 2026-08-17, on explicit user direction

Every event-publishing service in this scaffold (`usage-service`,
`issue-tracking-service`'s `LinkIssue`) publishes directly after a DB write
commits, not via the outbox pattern
[`architecture/05-data-architecture.md`](../../specs/backend-go/architecture/05-data-architecture.md)
specifies. Each service's README flags this as an accepted scaffold
simplification. Build a shared `common/outbox` helper (poll-based relay,
per that doc) once a second real consumer exists beyond
`notification-service` and a missed-publish actually has a cost worth the
added complexity — don't build it speculatively.

- [x] **Investigated first, built second — the precondition genuinely
      isn't met, and was built anyway on explicit user instruction, not
      silently ignored either way.** Initial pass (same day) checked the
      stated precondition directly: `usage.session.recorded` and
      `orca.issuetracking.link.created` both have zero real consumers
      anywhere in this codebase, so building `common/outbox` at that point
      would have been exactly the speculative build this item's own
      condition warns against — reported as "investigated, correctly not
      built." The user then explicitly asked for Epic G to be executed in
      full regardless. See §13 below for what was actually built, and for
      `issue-tracking-service`'s own real architectural fork (no
      database at all, by design) that came up along the way and was
      confirmed with the user before proceeding, rather than decided
      unilaterally.

---

## 3. Per-service tasks, by migration phase

### Phase 1 — Leaf services (lowest risk, per migration strategy doc)

**Status, 2026-08-17: all four rows below closed — see §14 for the full writeup.**

| Service | Remaining tasks |
|---|---|
| `usage-service` | ~~Wire `common/secrets.DatabaseCredentialsFromFile` into `main.go`~~ ✅. ~~Add OTLP exporter config~~ ✅ (shared `common/tracing`). Migrate hand-written SQL to `sqlc` — **deliberately still not done**, see §5: this is a cross-service pass, not per-service ad hoc. |
| `annotation-service` | ~~Add `request_id` column + idempotent write~~ ✅. ~~Wire OPA author-only check (Epic E)~~ ✅ (landed via a separate concurrent pass — see §14). |
| `notification-service` | Epic B ✅, Epic F ✅ (both already closed before this pass). ~~Add a `processed_events` dedup table~~ ✅. |
| `issue-tracking-service` | Epic B ✅ (already closed before this pass). ~~Jira `CreateIssue` hardcodes issue type `"Task"`~~ ✅ — real lookup added, scoped to avoid a `proto/` change (see §14). |

### Phase 2 — Mid-tier domain services ✅ done 2026-08-17 — see §15 below

| Service | Remaining tasks |
|---|---|
| `ai-provider-service` + `credential-broker-service` | ✅ Epic B (client wiring, already done). `credential-broker-service`: `RevokeCredential` now calls a real Vault KV v2 native destroy (`common/secrets.Client.KVDestroyMetadata`, new); `Ping` now calls a real `Sys().Health()` (`common/secrets.Client.Ping`, new); metadata+audit writes are now atomic via a new `TxRunner` port + one `pgx` transaction. `x-orca-service-id` → mTLS/SPIFFE stays correctly deferred (needs a service mesh that doesn't exist) — not attempted. |
| `automation-service` | ✅ Scheduler/ticker loop built (`internal/adapter/scheduler`, `SELECT ... FOR UPDATE SKIP LOCKED` claim, `next_run_at`/`dtstart`/`timezone`/`enabled` columns). ✅ `HandleExternalTrigger` implemented. ✅ `step_type` promoted to a first-class proto field (`orca.workflow.v1.StepType`, shared with workflow-service), no longer JSON-blob-parsed. |
| `workflow-service` | ✅ Real DAG wave-dispatch: `domain.DAGDefinition.BuildWaves` (Kahn's algorithm + general cycle detection), bounded worker pool (cap 10), `StepExecution` persistence, `Execute` now dispatches asynchronously on a background goroutine (documented architectural choice) instead of stopping at `status=running`. `ExecuteAdHocStep`'s persistence gap closed as a natural extension. Boot-time recovery scan still not implemented — honestly flagged, unchanged. |
| `infra-fleet-service` | ✅ `DevServer.ssh_target_id` links a dev server to its `SshTarget`; `sshconn.Connector` now wired into `devserveragent.Client` for real relay-ssh `Health` (dial+probe) and `Exec` (`shell.exec` only — other methods return a typed "no JSON-RPC agent" error, honestly, since the `relay.js` deploy step is still blocked on a build artifact this repo can't reach). |
| `project-service` | ✅ `UpdateProject`/`DeleteProject` (fail-closed active-execution guard, reusing the existing checkers) + full Repo/Worktree/ProjectGroup CRUD (21 new usecases) — proto extended, `Project` gained description/default_branch/visibility/created_by/timestamps. Source-projects, full membership CRUD, and OPA correctly stayed out of scope (not asked for). |

### Phase 3 — Gateway-facing services ✅ done 2026-08-17 — see §15 below (started ahead of Phase 4, at explicit user request for more parallelism)

| Service | Remaining tasks |
|---|---|
| `git-gateway-service` | ✅ Epic A relay path confirmed genuinely wired (re-verified, not just trusted). ✅ `GenerateCommitMessage` now relays the worktree's staged diff to the Dev Server Agent's real `ai.complete` RPC via `RelayExecutor.Complete` — method name/params verified against `specs/agent/api/agent-rpc-catalog-runtime.md`, not guessed. No-connection worktrees get a clear `FailedPrecondition`. |
| `scm-integration-service` | ✅ All 5 providers (GitHub/GitLab/Bitbucket/Azure DevOps/Gitea) now make real HTTP calls for ListIssues/CreatePullRequest/ListPullRequests/GetRateLimitStatus, with two providers' honest capability gaps (`ErrCapabilityUnsupported` for Azure DevOps issues, Gitea rate limits — those platforms don't have the concept). ✅ §9.1 decided: real OAuth 2.0 authorization-code web flow built (`StartOAuthFlow`/`CompleteOAuthFlow`/`GetAuthStatus`/`RevokeAuth`), not PTY/CLI login — credentials written via `credential-broker-service.WriteCredential`. ✅ `rate_limit_cache` real and consumed; `webhook_delivery_log` schema-only (no webhook-receiving RPC exists to write to it — honestly flagged, not invented). `RevokeAuth` can't actually revoke yet — needs a `RevokeCredentialByOwner`-shaped RPC on `credential-broker-service` that doesn't exist; flagged, not built (that service wasn't touched by this workstream). |

### Phase 4 — Identity/tenancy ✅ done 2026-08-18 — see §16 below (do last — everything above already depends on stubs pointing here)

| Service | Remaining tasks |
|---|---|
| `tenant-service` | ✅ Epic F (cache, closed 2026-08-17, see §12). ✅ `CreateTeamRequest`/`Team` now carry `settings_json`, threaded through `CreateTeamInput` → `domain.NewTeam` → the existing column — no `UpdateTeam` RPC yet, so settings can only be set at creation. |
| `auth-service` | ✅ Epic D (JWT/JWKS) — found already-landed. ✅ Epic E (OPA, replacing `requireAdminActor`) — found already-landed. SSO: still not built — confirmed this remains a product decision per the design doc, not attempted. Refresh-token/access-policy-CRUD/first-run-setup: first-run-setup was resolved separately (`bootstrap.go`, not a Phase 4 item); the other two remain an open decision, not built speculatively. |

### Phase 5 — Edge ✅ done 2026-08-18 — see §16 below

| Service | Remaining tasks |
|---|---|
| `api-gateway` | ✅ Epic D (real JWKS verification) — found already-landed. ✅ Wired REST routes for 12 of 14 remaining services (all but `scm-integration-service`/`workflow-service`, held back for real, documented backend-maturity reasons — see §16). |

---

## 4. Definition of done

A service is genuinely done — not just "scaffolded" — when it clears every
item in
[`specs/backend-go/standards/production-readiness-checklist.md`](../../specs/backend-go/standards/production-readiness-checklist.md).
That checklist's bar (own database ✅ already true for all 17, migrations
✅, Vault-sourced secrets ⬜ still env-var-based in every service's `main.go`,
mTLS ⬜, OPA coverage ⬜, load-tested ⬜, chaos-tested ⬜, runbook ⬜, …) is
the actual finish line — the tasks in §2/§3 above get a service to
"feature-complete," not to "production-ready." Don't conflate the two when
planning a release.

## 5. Infrastructure/process work not tied to any one service

- [ ] CI pipeline (`golangci-lint`, `govulncheck`, `buf lint`/`breaking`,
      per-module `go test`, Trivy image scan) — none of this is wired yet,
      only the Makefile targets that a real CI would call.
- [ ] Helm charts (one per service + the umbrella chart), per
      [`architecture/10-deployment-infrastructure.md`](../../specs/backend-go/architecture/10-deployment-infrastructure.md)
      — nothing beyond each service's standalone `Dockerfile` exists yet.
- [ ] A real `staging` environment with real Postgres instances, a real
      Vault cluster (HA, auto-unseal), and real NATS — everything in this
      repository has only been run against `docker-compose.yml`'s
      single-instance dev stack.
- [ ] `sqlc` migration for every service's hand-written `pgx` repository
      layer (flagged as a deferred gap in all 17 service READMEs
      identically) — worth doing as one focused pass across all services
      once the query set in each has stabilized, not per-service ad hoc.

## 6. Third update, same day — operator-chosen admin password + `/api/trace-stream`

- Bootstrap (§0) only runs once, on a fresh deployment with zero users —
  setting `BOOTSTRAP_ADMIN_PASSWORD` after the admin already exists
  (auto-generated on first boot) is a no-op by design. To rotate it: delete
  the existing row (`auth.sessions`/`auth.audit_log` FK rows first, then
  `auth.users`) and recreate `auth-service` — documented in
  `deploy/dev/.env.example`'s bootstrap section now, since this will come
  up again for anyone else redeploying fresh.
- Added `GET /api/trace-stream` (SSE) to `api-gateway` — was a flat 404
  before (route never existed), which `frontend/`'s `EventSource` silently
  swallows (auto-reconnects on any error) but which still spammed
  network-tab noise and left `TracePanel` permanently empty. Heartbeat-only
  for now — real backend trace-event forwarding (the old backend's global
  `registerTraceSink()` fan-out) has no backend-go equivalent yet; adding
  one is a real follow-up, not attempted here.

## 7. Fourth update — `wscompat`'s message loop was fully synchronous per connection (real architectural bug, not a missing channel)

**Symptom:** browser console showed `Uncaught Error: Request timed out: preflight.check` even though `preflight.check` itself, unregistered, would fail FAST via `notImplementedHandler` — a timeout specifically meant something else on the same connection never got a chance to respond at all.

**Root cause:** `wscompat.Handler.ServeHTTP`'s loop read one `InboundMessage`, fully processed it INLINE (including any downstream gRPC call, with no context deadline), and only then read the next message off the socket. A frontend bootstrap fires many `invoke` calls close together on one WS connection; if any one of them reached a real channel handler making a slow/unreachable downstream gRPC call, every message queued behind it on that same connection — including totally unrelated channels like `preflight.check` — sat unread until the client's own 30s `INVOKE_TIMEOUT_MS` gave up.

**Fix**, both in `internal/adapter/wscompat/handler.go`:
- The read loop now only reads — every `invoke`/`send` dispatches in its own goroutine (`writeMu sync.Mutex` serializes the actual WS writes back, since the underlying library doesn't allow concurrent writers, but dispatch/downstream-call work happens fully in parallel).
- Every dispatch gets a 25s `context.WithTimeout` at the transport layer — applies to every channel automatically (including ones wired in the future), not something each `channels.go` handler has to remember to add itself.
- `preflight.check` is now registered with a fast, LOCAL, honest response (`git: installed=true` since git-gateway-service's local executor requires the real binary; `gh`/`glab`: `installed=false, authenticated=false`, since scm-integration-service is deliberately a direct-API client, not a CLI wrapper — see that service's own doc for why "gh CLI installed" isn't even a concept that maps onto this architecture).

Verified with a raw WebSocket test (handshake + a manually-framed `invoke` message, since no WS test tooling was available on the server) through `172.20.2.39:6769` directly: `preflight.check` now responds in **0.0s**, not 30s.

**On "backend không thấy dev-server nào đăng ký vào" (no dev-server registered) — this is accurate, expected state, not a new bug:**
`infra-fleet-service`'s database is fresh and empty (0 rows), and — separately — `devServer.*`/`fleet.*` channels aren't registered in `wscompat` at all yet, so the frontend's dev-server-management UI has nowhere to send a create/list call even if one existed to list. Closing this for real means: (1) wiring `devServer.*`/`fleet.*` channels into `wscompat` (mechanical, same pattern as the channels already registered), AND (2) Epic A (the Dev Server Agent relay client) — a dev server is only useful once `infra-fleet-service` can actually reach it, which it can't yet (`internal/adapter/devserveragent/` is still a stub). (1) alone would let someone register a dev server's *metadata* but nothing would actually work against it until (2) lands. Not attempted in this pass — this is real, substantial scope, tracked in §2 Epic A, not something to shortcut.

## 8. Fifth update, 2026-08-17 — Epic A, relay-websocket pass

**Scope of this pass:** implemented `services/infra-fleet-service/internal/adapter/devserveragent/` for real, for **relay-websocket mode only** — Orca dials out to the agent's own WebSocket server and drives the Stack B (`relay-protocol.ts`) 13-byte-framed JSON-RPC handshake + call cycle. `direct-websocket` and `relay-ssh` remain unimplemented (see §2 Epic A's checklist for exactly what each still requires — they're materially larger: an inbound WS server + token-issuance endpoint for the former, SSH-deploy of a remote binary for the latter). relay-websocket was chosen as the smallest slice that is genuinely end-to-end correct rather than partially faked.

**Key finding while reading the TS source (`specs/agent/api/connection-modes.md` §0's "Important convergence" note, confirmed by reading `ws-handshake.ts`/`ssh-channel-multiplexer.ts` directly):** relay-websocket's handshake and all subsequent RPC traffic use **Stack B**'s frame codec (`backend/src/main/ssh/relay-protocol.ts` — `encodeJsonRpcFrame`/`FrameDecoder`/`MessageType`), not Stack A's `agent-wire.ts`, even though both stacks share the same 13-byte header shape. The devserveragent package's original stub doc comment (written before this was confirmed) sketched porting "the" wire protocol generically; this pass ports Stack B specifically, since that's what actually runs on the wire for this mode.

**What was built** (all in `internal/adapter/devserveragent/`):
- `frame.go` — `EncodeFrame`/`DecodeFrame`, byte-for-byte matching `relay-protocol.ts`'s header layout and `MAX_MESSAGE_SIZE` (16 MiB) rejection. One simplification that is NOT a behavior change: since a WebSocket transport delivers whole messages (unlike SSH exec-channel stdio, which is why `relay-protocol.ts`'s `FrameDecoder` is an incremental chunk-list parser), `DecodeFrame` here is single-shot per WS message — the two are equivalent for this transport.
- `jsonrpc.go` — JSON-RPC 2.0 request/response/error types and encode/parse helpers, including the same "has `id`, no `method`" filter `runOrcaInitiatorHandshake` uses to distinguish a response from an unrelated request/notification arriving on the same connection.
- `config.go` — `Config`/`LoadConfigFromEnv()` (`AGENT_PORT` default 6799, `ORCA_AGENT_TOKEN`, `ORCA_VERSION`), all TS-side timing constants ported as named fields (dial 10s, handshake 20s, request 30s, keepalive 5s, idle 20s, backoff 2s→60s+jitter) rather than re-derived magic numbers.
- `session.go` — one persistent connection per dev server: dial with `Authorization: Bearer <token>` header, `runInitiatorHandshake` (Orca sends `agent.handshake` first, frame id=1/ack=0, exactly matching `runOrcaInitiatorHandshake`), a read loop routing responses to pending calls by JSON-RPC id (frame-level id/ack is liveness/ack bookkeeping only, matching `handleFrame`'s actual routing — confirmed by reading `ssh-channel-multiplexer.ts` directly, not assumed), and a keepalive ticker. Reconnect in this pass is **lazy** (next `Exec`/`Health` call re-dials after a drop), not `DevServerRelayBridge`'s background exponential-backoff loop — `backoffDelay()` is ported and unit-tested but not yet wired into an automatic retry loop; a background reconnect loop is a reasonable near-term follow-up, not done here to keep this pass's surface reviewable.
- `client.go` — `Client{cfg, sessions map[devServerID]*session}` implementing `usecase.DevServerAgentClient`. `Exec` is a generic method+params passthrough (no per-method name/shape translation layer — see the package doc comment on why that's deliberately out of scope for this pass). `Health` reports `(false, nil)` rather than an error on an unreachable dev server, since "not reachable" is the expected common answer this method exists to give.

**A real domain-model gap found, and the decision made about it:** `domain.DevServer` (this scaffold's proto-sized subset) has only `ID`/`TenantID`/`Host`/`Mode` — no port or per-device token field. Rather than extend the domain type + proto + migration for this (which `specs/agent/api/connection-modes.md` §"relay-websocket token (contrast)" shows isn't how the real system models it anyway — `ORCA_AGENT_TOKEN` there is documented as "a static, operator-set, long-lived shared secret ... never expires or rotates," i.e. deployment-wide, not per-dev-server), this pass models `Port`/`Token` as service-level `Config` (env vars), matching that reality. A future per-dev-server override would need a real schema change — tracked as a follow-up, not silently worked around.

**Testing — no real Dev Server Agent exists in this environment, unlike every previous fix in this session which was verified live against `172.20.2.39`.** In its place: `frame_test.go`/`jsonrpc_test.go` unit-test the codec in isolation (round-trip, truncated/oversized-frame rejection, response-vs-request filtering), and `client_test.go` spins up a `fakeAgent` — a minimal stand-in for `agent-connection-relay.ts`'s WebSocket server, built with the same `coder/websocket` library `api-gateway`'s `wscompat` already uses — served over a real `httptest.Server` TCP loopback connection. This exercises a genuine dial → Bearer-auth check → Stack B handshake → JSON-RPC call round trip end-to-end, including an auth-failure case (`Health` returns `false` on a wrong token) and the direct-websocket/relay-ssh "not implemented" error path. 13 new tests, all passing. This is real coverage of the transport logic, but it is not the same as live verification against the actual `agent/` binary — that remains the honest gap until a real relay-websocket-mode agent is available to test against, flagged here rather than silently presented as equivalent to this session's other live-verified fixes.

**Verified clean, workspace-wide, after this change:** `go build`/`go vet`/`go test` clean for all 19 `go.work` modules (each module built/vetted/tested individually — the workspace root itself is not a module, so a root-level `go build ./...` correctly errors; this is pre-existing, not new). `go mod tidy` run for `infra-fleet-service` after adding the `github.com/coder/websocket` dependency.

**Not done in this pass** (see §2 Epic A's updated checklist): `direct-websocket`/`relay-ssh` modes; wiring `git-gateway-service`'s `RelayExecutor` and `workflow-service`'s step-executor stubs to this new client; wiring `devServer.*`/`fleet.*` channels into `wscompat` so this is reachable from the frontend at all; the `infra.connections`/`infra.port_forwards`/`infra.provider_registry_entries` schema extension; a background auto-reconnect loop.

## 9. Sixth update, 2026-08-17 — Epic B, all 4 consumers wired for real

**Scope:** replaced every stub `CredentialResolver`/`CredentialBrokerClient`/`VaultSigner` in `ai-provider-service`, `scm-integration-service`, `issue-tracking-service`, and `notification-service` with a real `credential-broker-service` gRPC client. Two of the four turned out to need real additions to `credential-broker-service` itself, not just a client-side swap — found by trying to make each call for real, the same "discovered while implementing, not guessed in advance" pattern Epic C's proto-gap items follow.

**Why 3 new RPCs, not 1:** `credentialbroker.proto` had exactly the RPC shape `WriteCredential`/`RotateCredential` needed (`ai-provider-service`'s create/rotate paths), but not what the other three call patterns needed:

- **`GetCredentialMetadata(credential_id) -> CredentialMetadata`** — `ai-provider-service.ResolveCredential`'s port contract must never see plaintext (its `grpcclient` package's pre-existing SECURITY-CRITICAL doc comment says so explicitly, and it's right: `usecase.CredentialRef` has no field for one). The existing `ResolveCredential` RPC returns `bytes value` — plaintext, by its own doc comment. There was no metadata-only RPC to call instead, so `ai-provider-service`'s `ResolveCredential` would have had to either call the plaintext RPC and discard the value (fragile, and audits a resolve that never needed to happen) or stay a stub forever. Added this RPC instead — a pure `CredentialMetadataRepository.Get` passthrough, no Vault call, no audit row (nothing secret is exposed, so nothing to audit).
- **`ResolveCredentialByOwner(tenant_id, category, owner_id) -> value`** — `scm-integration-service` and `issue-tracking-service`'s `CredentialResolver.Resolve(ctx, tenantID, provider)` signatures (pre-existing, driven by their own usecase call sites) never receive an opaque `credential_id` at all — neither service has a "connect this account" write flow yet, so there was never an id to be handed. `ResolveCredential`'s by-id lookup literally cannot serve these callers. Added a same-fail-closed-and-audited sibling keyed by `(tenant_id, category, owner_id)` instead, backed by a new `CredentialMetadataRepository.GetByOwner` query (most-recent non-revoked row for that key — real SQL, `ORDER BY created_at DESC LIMIT 1`, since this scaffold has no uniqueness constraint prevent­ing more than one row per owner). `owner_id` convention: the provider name itself (`"github"`, `"jira"`, etc.) — `CREDENTIAL_CATEGORY_SCM_OAUTH`/`_ISSUE_TRACKER_OAUTH` are each one category shared by every provider under that umbrella, so `owner_id` is what actually distinguishes a tenant's GitHub connection from their GitLab one.
- **`SignVapidPayload(tenant_id, payload) -> signature`** — notification-service's VAPID Transit signing is a "sign with a service-managed key" operation, not a "write/resolve/rotate a stored credential." None of the other 4 RPCs fit its shape (no `credential_id`, no envelope with a lifecycle). Added a narrow, purpose-named RPC rather than a generic "Transit-sign anything" surface — deliberately, since a generic cross-service Transit passthrough is a much bigger security-surface decision than this task warranted. Internally it's the exact same `store.TransitEncrypt(ctx, "vapid-signing-"+tenantID, payload)` call notification-service used to make directly against Vault — only the process boundary moved.

**A design-doc inconsistency this surfaced, not invented:** `architecture/06-secrets-vault-architecture.md`'s "What goes in Vault" table's VAPID row said *"`notification-service` asks Vault to sign push payloads"* (direct access), while its own "`credential-broker-service`'s role" section a few paragraphs down said no other service touches Vault directly for tenant secret material, with no VAPID exception carved out. The scaffold's original `vaultsigner/signer.go` followed the table's literal wording. Both places in that doc are now updated to say the same thing: VAPID signing goes through `credential-broker-service.SignVapidPayload`, closing the inconsistency by fixing the code AND the doc, not just one of them.

**Broker-side changes** (`credential-broker-service`): `internal/adapter/postgres/repository.go` gained `GetByOwner`; three new `internal/usecase/*.go` files (`get_credential_metadata.go`, `resolve_credential_by_owner.go`, `sign_vapid_payload.go`); `resolve_credential.go` refactored to extract its fail-closed/audit-ordering tail into a shared `resolveMetadata` helper both `ResolveCredential` and `ResolveCredentialByOwner` call, so the two can never drift on the audit-ordering guarantee (`resolve_credential.go`'s doc comment) — confirmed by a test (`TestResolveCredentialByOwner_RevokedIsNotFound`) that documents a real, intentional behavior difference between the two RPCs: `GetByOwner`'s query filters out revoked rows at the SQL level, so `ResolveCredentialByOwner` can never actually reach `resolveMetadata`'s "revoked" branch — a revoked credential looks identical to "never existed" for an owner-keyed lookup, unlike a by-id lookup which can still find and report a revoked row. 9 new tests for the 3 new usecases (metadata-only calls never touch Vault or the audit log; owner-lookup audit-ordering; not-found paths write no audit row, matching the pre-existing FK-driven limitation `ResolveCredential`'s own doc comment already documents; Transit-error propagation).

**Consumer-side changes**, one per service:
- `ai-provider-service`: `usecase.WriteCredentialInput` gained `OwnerID`; `create_account.go` populates it (`UserID` → `ProjectID` → `"ai-provider-service"` fallback chain, since most accounts in this scaffold are server-scoped with neither). `grpcclient/credential_broker_client.go` rewritten: real `WriteCredential`/`RotateCredential` calls (category hardcoded to `CREDENTIAL_CATEGORY_AI_PROVIDER_KEY` — the one category this whole service ever uses), `ResolveCredential` calls `GetCredentialMetadata`, never the plaintext RPC (see the SECURITY-CRITICAL doc comment carried over from the stub). `RotateCredential`'s `NewEncryptedEnvelope` goes over the wire empty — this scaffold has no client-side-crypto/`PushCiphertext` integration to produce a real one yet (a pre-existing, already-documented scaffold simplification `WriteCredential`'s own doc comment accepts for the same reason), so only the ref/status-transition plumbing is real, not "new material differs from old."
- `scm-integration-service` / `issue-tracking-service`: `internal/adapter/credentialbroker/stub.go` and `internal/adapter/credential/stub_resolver.go` deleted, replaced by `client.go` in each, both calling `ResolveCredentialByOwner`. `issue-tracking-service`'s `Credential` has 3 fields (`BaseURL`/`Email`/`Token` — Jira needs all three, Linear only `Token`), so its adapter documents and implements a JSON-envelope convention (`{"baseUrl":...,"email":...,"token":...}`) for what `WriteCredential` would need to write for this category — not yet exercised end to end against a real written credential, since neither service has a "connect this account" write flow built yet (a pre-existing gap, not one this pass introduces or was asked to close).
- `notification-service`: `vaultsigner/signer.go` rewritten from a `common/secrets.Client` wrapper to a `credentialbrokerv1.CredentialBrokerServiceClient` wrapper calling `SignVapidPayload`; `main.go` no longer constructs a Vault client at all for this service (dropped the `vault` health check accordingly, matching the convention every other broker-consumer main.go already follows: no dedicated health check for the broker connection).

**Deploy config**: `deploy/dev/docker-compose.yml`'s `notification-service` block — `depends_on: vault` replaced with `depends_on: credential-broker-service` (the other 3 consumers already had this dependency listed; `notification-service` was the one still pointing at its old, now-removed, direct Vault dependency).

**Verified clean, workspace-wide:** `go build`/`go vet`/`gofmt -l`/`go test` clean for all 19 `go.work` modules after this change (each checked individually, same as every prior pass — the workspace root is still not a module). `go mod tidy` run for every touched module (`proto` needed it after `buf generate` regenerated the 3 new RPCs; the other 5 touched services needed it after picking up new imports).

**Not done in this pass** (real, pre-existing gaps this pass's own consumers now surface more concretely, not new scope Epic B asked for): no "connect this SCM/issue-tracker account" write flow exists anywhere yet, so `WriteCredential` for `CREDENTIAL_CATEGORY_SCM_OAUTH`/`_ISSUE_TRACKER_OAUTH` has never been exercised against a real caller — only `ai-provider-service`'s create/rotate paths call `WriteCredential`/`RotateCredential` for real; a uniqueness constraint on `(tenant_id, category, owner_id)` for non-revoked rows (so `GetByOwner`'s "most recent" tie-break is a defensive fallback, not a load-bearing assumption); `RequestingService` (the `x-orca-service-id` gRPC metadata header every one of these new clients could now set) is still not sent by any client anywhere in the tree — every access-audit-log row this pass generates will show an empty/`"unknown"` accessor until that's wired, tracked separately (not part of Epic B's stated scope, but worth flagging since Epic B is what finally makes this header's absence observable end to end).

## 10. Seventh update, 2026-08-17 — Epic C, all four items resolved (3 built, 1 correctly deferred)

**Scope:** ran the three independent proto-gap items in parallel (three background subagents, one per service — safe to parallelize since they touch disjoint files and only one of them touches `proto/`), then wired `project-service`'s two consumer stubs to the results myself once all three finished. The fourth item (`auth-service`'s RPC surface) was investigated and resolved directly (see §2 Epic C's updated checklist — one sub-item closed via an already-shipped alternate mechanism, two correctly deferred pending their real prerequisite epics, none built as new RPCs).

**`workflow-service` — `HasActiveExecutions`, fully accurate.** `WorkflowExecution` gained a real `ProjectID` (previously accepted by `ExecuteInput` but explicitly documented as "not persisted" — now persisted), migration `0002_execution_project_id` adds the column plus a partial index scoped to non-terminal statuses. `HasActiveExecutions(project_id)` is a real `EXISTS` query against `(tenant_id, project_id, status IN ('pending','running','paused'))` — this one has no asterisk: workflow-service already tracked execution status accurately (`Pause`/`Resume` are real, tested transitions), so persisting `project_id` was the only missing piece.

**`task-service` — `HasActiveExecutions`, real but with one honest, documented limitation.** `Task` gained `ProjectID` (migration `0002_task_project_execution_tracking`), and — more substantively — `ExecuteTask.Execute` now actually transitions a task to `StatusInProgress` before dispatching (a real state change that did not exist before this pass; previously `Execute` never touched the task's status at all). `HasActiveExecutions` queries `status = 'in_progress'` for real. The asterisk, flagged in code comments, the migration's own SQL comment, this service's README, and now here: task-service has no execution-completion callback and no RPC to transition a task back out of `in_progress` — so this is a real, one-way signal today. A project with any task ever dispatched will read as "active" until a completion/status-update path is built (a real, separate follow-up — not silently pretended away, and not this pass's scope to also build). This is the same category of honest limitation workflow-service's own executions have always carried ("persisted in StatusRunning and never progresses further on its own") — applying that established, honest pattern to task-service's new status transition, not inventing a new kind of gap.

**`orchestration-service` — proto extended, `CreateGate` no longer fails closed, and a real premise-correction along the way.** Added `orchestration_task_id` to `CreateDispatchContextRequest`/`DispatchContext`, and `question`/`options` to `CreateGateRequest`/`DecisionGate`. The subagent doing this work caught something my own investigation (and this service's own README, at the time) had wrong: `CreateGate`'s usecase/repository already accepted `question`/`options` internally, but **`CreateDispatchContext`'s usecase/repository/port did NOT already accept `orchestration_task_id`** despite the migration's nullable FK being "ready" for it — the existing integration test had been directly patching the database after the fact to work around exactly this gap. Extending only the proto+gRPC-adapter layer (my original instruction to the subagent) would NOT have actually fixed `CreateGate`'s `FailedPrecondition`, since a dispatch context created through the real RPC path would still end up with `orchestration_task_id = NULL`. The subagent correctly extended the full chain instead: `CreateDispatchContextInput.OrchestrationTaskID` → `DispatchContextRepository.CreateDispatchContext(..., orchestrationTaskID)` → the Postgres `INSERT`. A judgment call from my original instructions was exercised as directed: `CreateGateRequest.orchestration_task_id` is deliberately left unread in the gRPC handler — `CreateGate` derives the task id internally from `dispatch_context_id` via a locked (`FOR UPDATE`) read, a derive-not-trust boundary `create_gate.go` documents explicitly; the proto field exists for wire symmetry but widening the usecase to accept (and trust) a caller-supplied override was correctly judged out of scope, not a widening this task asked for. New tests cover both the now-succeeding case (dispatch context created with a task id) and the still-correctly-failing case (a genuinely task-less dispatch context, e.g. ad-hoc coordinator-only work) — the latter is the real invariant working as designed, not a residual bug.

**`project-service` — both consumer stubs replaced with real RPC calls** (`internal/adapter/grpcclient/workflow_execution_checker.go`/`task_execution_checker.go`), reusing the `grpc.ClientConn`s these adapters already dialed (config/wiring was already correct — see the STUB doc comments this replaces, which explicitly said "the ClientConn is dialed for real... but HasActiveExecutions doesn't use it yet"). `RebindDevServer`'s guard is a real no-op-closing fix for the workflow-service half; for the task-service half, it now correctly fails closed (rejects a rebind) for any project with a task ever dispatched, per that service's one-way-status limitation above — a stricter-than-ideal but honest and safe default (fail closed on uncertain data, matching this usecase's own documented fail-closed policy for checker errors), not a broken guard.

**`auth-service` — see §2 Epic C's updated checklist and that service's own README "Known gaps" for the full per-item reasoning.** No new RPCs built; each of the three named gaps got an explicit, reasoned resolution rather than a blanket "not implemented."

**Parallelization notes** (for future reference, since the user explicitly asked for this): three background subagents ran concurrently, one each for workflow-service, task-service, and orchestration-service — safe because they touch disjoint file trees and only orchestration-service's agent touched `proto/` (avoiding a `buf generate` race between concurrent agents, which would corrupt the shared generated-code tree if two agents regenerated it at overlapping times). workflow-service's and task-service's proto additions were made and regenerated by me, sequentially, *before* dispatching any agent, specifically so those two agents needed zero proto access. `auth-service`'s investigation was deliberately kept out of the parallel batch and done directly, since "decide the RPC surface" is a judgment call better made with full context than delegated blind.

**Verified clean, workspace-wide:** `go build`/`go vet`/`gofmt -l`/`go test` clean for all 19 `go.work` modules after every change in this pass, checked together in one final sweep after project-service's wiring landed (not just per-agent). One unrelated, pre-existing `gofmt` violation in `workflow-service/internal/config/config.go` was found and fixed during this final sweep — introduced by unrelated, concurrent edits to that service happening outside this Epic C pass (visible via this session's own file-change notifications; not something any of this pass's three agents or this pass's own edits touched), not a defect in this pass's own work, but real all the same and worth a clean workspace either way.

**Not done in this pass, honestly flagged:** task-service's completion/status-update path (the real fix for the one-way `in_progress` limitation above) is a real, separate follow-up, not attempted here; `orchestration-service`'s new integration tests are written and compile (`go vet -tags=integration`) but could not be executed in that pass's sandbox (no `migrate` binary available at the time) — flagged rather than silently assumed passing. `workflow-service`'s `ListTemplates`/`ResolveTemplate`/`CancelExecution` were done in a follow-up pass — see §11.

## 11. Eighth update, 2026-08-17 — workflow-service's `ListTemplates`/`ResolveTemplate`/`CancelExecution`, on explicit request

**Why this landed after all, despite §2's deferral condition:** asked directly ("thực hiện giúp tôi" — please implement it) after explaining why it had been left deferred. The precondition this item's own text names ("once template inheritance is actually being built") still hadn't been met by anything else in the codebase at the time this request landed — confirmed by checking `internal/domain/template.go`/`migrations/0001_init.up.sql` fresh, not assumed from memory. This section documents building it now, deliberately, not the precondition having quietly become true on its own.

**`CancelExecution`** — the independent third of these three, built as a straightforward peer of `PauseExecution`/`ResumeExecution`: `domain.WorkflowExecution.Cancel()` (pending/running/paused -> cancelled, `ErrCannotCancelTerminal` otherwise), usecase, gRPC handler, wired into `main.go`. Nothing about it depended on inheritance.

**`ListTemplates`/`ResolveTemplate`** — the two that did depend on inheritance, built together:
- **Schema**: new migration `0003_template_parent_chain` adds `workflow.templates.parent_template_id` (nullable FK to itself) + an index — the column workflow-service.md §5 always specified but this scaffold's initial narrowed schema (deliberately) omitted.
- **`domain.WorkflowTemplate`** gained `ParentTemplateID`; `NewWorkflowTemplate` gained a `parentTemplateID` param and a new invariant, `ErrTemplateSelfParent` — direct self-parenting only, per workflow-service.md §4's exact wording ("Constructor rejects a template naming itself as its own parent, directly"). Multi-hop cycle detection beyond `ResolveChain`'s depth cap is explicitly not implemented, and explicitly not needed: this service's RPC surface has no `UpdateTemplate` to rewire an existing template's parent after creation, so a cycle structurally cannot arise through normal use (a parent must already have an assigned id, and therefore already exist, before a child can reference it).
- **Resolution policy — a deliberate, documented decision, not an assumption**: workflow-service.md §6 specifies the recursive-query *shape* (`WITH RECURSIVE ... depth < 5`) but never specifies a merge/override policy for what "the effective template" means once you have a chain of them. Chose: walk from the requested template outward to its ancestors, return the first (most specific) one whose `dag_json` actually defines at least one step. This means a personal template that exists purely to opt into its team/company parent's steps (an intentionally empty `dag_json`) correctly inherits rather than resolving to "no steps" — the alternative (always return the leaf, ignore ancestors entirely) would make `parent_template_id` pointless, and the alternative (always merge/union steps across the whole chain) isn't specified anywhere and would silently change semantics if a team ever wanted its own steps to fully replace a company template's. `ResolveTemplateResponse` also returns the full walked chain (root-first) for callers that want to show the inheritance path, not just the answer.
- **`ListTemplates`**: keyset-paginated, copied convention-for-convention from `annotation-service`'s `ListAnnotations` (opaque `page_token` = last-seen id, `ORDER BY id`, next-token set iff the page came back full) — deliberately not inventing a new pagination shape for this one service.
- **`CreateTemplate`** gained a `parent_template_id` request field with an explicit existence check (`GetTemplate` before insert, `WORKFLOW_PARENT_TEMPLATE_NOT_FOUND` on miss) rather than relying on the FK constraint alone to fail — matching task-service's `CreateTask`/`ParentID` convention exactly, for the same reason (a clean `KindFailedPrecondition` instead of an opaque DB constraint-violation error).

**Testing — real, and a genuine live-verification win beyond this pass's own new code.** 17 new unit tests (a new `fakeTemplateRepository`, since none existed before — `CreateTemplate` itself had never been unit-tested at all until this pass) covering: leaf-with-own-steps wins, empty-leaf-inherits-from-parent, all-empty-returns-leaf-itself, not-found, missing-template-id, tenant/scope filtering, pagination cursor advancement, self-parent rejection, unknown-parent rejection. Then — since a `migrate` binary and Docker were both available in this environment for the first time in this session's history (`migrate` wasn't installed; installed it via `go install` for this pass), **5 real integration tests ran against a live testcontainers Postgres**, not just compiled: `TestRepository_ListTemplates_KeysetPagination`, `TestRepository_ResolveChain_RootFirstOrder`, `TestRepository_ResolveChain_NotFound` (all new, all passing) plus the two pre-existing `TestRepository_CreateAndGetTemplate`/`TestRepository_ExecutionPauseResumeRoundTrip`. Those two **failed on first run** — `invalid input syntax for type uuid: "tmpl-1"` / `"exec-1"` — a real, pre-existing bug: these tests seed literal non-UUID strings as primary keys against `UUID`-typed columns, which fails at the database, not at compile time or in the (Docker-free) unit-test suite `go test ./...` runs by default. This bug had therefore never actually been caught, because these specific integration tests had (as far as this session's history shows) never actually been run before now, in this sandbox or otherwise — the earlier orchestration-service pass (§10) hit the same "no `migrate` binary" wall and could only confirm its own new integration tests *compiled*, not that they passed. Fixed by swapping the literal ids for valid UUID-format strings; all 5 integration tests now pass for real. This is the same "don't assume a test passes just because it compiles" discipline this session has applied throughout, applied here to tests that predate this pass, not just this pass's own additions.

**Verified clean:** `gofmt -l .`, `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test ./... -count=1` (17 new + all pre-existing tests) all clean for `workflow-service` on its own; `go build`/`go vet`/`gofmt -l`/`go test` clean for all 19 `go.work` modules together in a final workspace-wide sweep, same discipline as every prior pass.

**Not done, honestly flagged:** `StreamExecutionEvents` (the one remaining design-doc RPC not in the generated proto) wasn't asked for and wasn't added; multi-hop template-cycle detection beyond the depth cap, per the reasoning above, isn't needed given the current RPC surface but would need revisiting if `UpdateTemplate` is ever added.

## 11. Eighth update, 2026-08-17 — Epic A, second pass: everything except direct-websocket/relay-ssh

**Scope decision, made explicitly before starting (not assumed):** Epic A's two remaining connection modes — `direct-websocket` (needs `infra-fleet-service` to run an inbound WS server + a `POST /api/agent-token` endpoint + a slot-registration lifecycle) and `relay-ssh` (needs SSH-deploying and launching a relay binary over a connection manager this service doesn't own at all) — stay deliberately deferred, same reasoning §8's relay-websocket pass already used: ship the smallest genuinely-correct slice, not a partial fake. Everything else on Epic A's checklist was closed in this pass. The `infra.connections` table went with the real routing model (its own entity, not schema-only) — a decision made explicitly, not defaulted to.

**Foundational change, done first since three other items depend on it: a new generic `Relay` RPC.** `git-gateway-service`'s own design doc's sequence diagram, and `task-service`'s pre-existing stub doc comment, both already assumed an RPC shaped like `Relay(connectionId, method, params) -> result` existed on `infrafleet.proto` — it didn't. Added `rpc Relay(RelayRequest) returns (RelayResponse)` (`connection_id`, `method`, `params_json` string → `result_json` string — mirrors `devserveragent.Client.Exec`'s own generic passthrough, deliberately not one typed RPC per caller), implemented as `usecase.Relay`: resolve the connection, `DevServerAgentClient.Exec`, propagate a real error rather than an empty result on either failure (same "always relay, never silently swallow" rule `ScanWorkspacePorts` already established for TS Gap 7). Also added `rpc ListDevServers` (the `DevServerRepository.List` method already existed, unused — nothing had ever exposed it over gRPC) and `rpc CreateConnection` (the write path the new `infra.connections` table needs — without it, the table would be schema nobody writes to).

**`infra.connections` is now a real routing model, not schema-only** (`migrations/0002_connections`). `ResolveConnection` now joins `connections` → `dev_servers` within tenant scope instead of the original scaffold's `connectionId == dev_server.id` equation, and returns `repo_path`/`worktree_id` alongside the resolved `DevServer` — the per-connection metadata `git-gateway-service`'s `RelayExecutor` needs. `infra.port_forwards`/`infra.provider_registry_entries` got schema only in the same migration, per the design doc's fuller sketch — no usecase or RPC reads/writes them yet, tracked as a follow-up once a real caller needs port-forward or provider-registry-audit behavior (same "don't build speculatively" posture as Epic G's outbox pattern).

**Background auto-reconnect, closing a gap §8 explicitly flagged as a near-term follow-up.** `devserveragent/session.go`'s `backoffDelay`/`reconnectAttempt` were ported and unit-tested in the first pass but never wired into an actual retry loop — `handleDisconnect` now spawns `backgroundReconnect`, which retries `connect()` at `backoffDelay`-paced intervals until it succeeds or the session closes. The original lazy-redial-on-next-call behavior in `getOrCreateSession` is kept as the fallback for a call that arrives mid-backoff, not replaced. Tested with a fake agent that deliberately drops the first connection right after handshake and verifies the session re-handshakes on its own — the test asserts nothing but a passive read of internal session state during the wait, specifically to prove background reconnect (not the lazy-redial fallback) is what's under test.

**Three parallel consumers wired to the new `Relay` RPC, once the proto/schema landed** (three background subagents, one per service — safe to parallelize since they touch disjoint service trees and none of them touch `proto/`, which I'd already regenerated before dispatching them):

- **`git-gateway-service`**: `ConnectionResolver` and `RelayExecutor` (`internal/adapter/grpcclient/`) are real — `ConnectionResolver` dials `ResolveConnection`, `RelayExecutor` dials `Relay` with methods `git.status`/`git.diff`/`git.commit`/`git.push`/`git.pull`. Two honest gaps surfaced and documented rather than papered over: (1) the `git.*` param/result JSON field names are matched to this service's own `domain.GitStatus`/etc. field names, not verified against `specs/agent/api/agent-rpc-catalog-git-fs.md`'s *different* documented contract for the SSH Relay Daemon's `git.*` handlers (different field names, different result shapes) — reconcile before production use; (2) `usecase.GitExecutor`'s method signatures only carry `repoPath`, not the `connectionId` that resolved it, so `RelayExecutor` reuses `repoPath` as the relay's `connectionId` too — correct only because those two values happen to coincide today, not guaranteed to stay that way. See `git-gateway-service/README.md`'s updated gaps section and `relay_executor.go`'s doc comments for the full reasoning on both.
- **`workflow-service`**: `AgentStub`/`ShellStub`/`NotificationStub` are gone — `internal/adapter/infrafleetclient/` has real executors calling `Relay` with `agent.exec`/`shell.exec`/`notification.send`. A real, previously-undocumented gap closed along the way: the step-config JSON had no field identifying which dev server a step should target at all — added `ConnectionID` to new `AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig` domain types. Because DAG wave-dispatch is still not implemented (the pre-existing, separately-tracked gap in §3's Phase 2 table — `Execute` still never progresses an execution past `running`), these executors are reachable today only via the already-real `ExecuteAdHocStep` path, not a template-driven run. `agent.exec`'s method name/shape is flagged as needing reconciliation before production use — reading the old TS `StepExecutors.ts` surfaced that its `agent.exec` is a generic process-exec RPC, not the prompt-driven step this service's `agent` step type actually represents (TS needed a separate `agent.execPrompt` for that).
- **`api-gateway`**: `wscompat` gets `devServer.list` (→ `ListDevServers`), `devServer.add` (→ `RegisterDevServer`), and `fleet.health.checkAll` (→ `GetFleetHealth`, filtered client-side by the requested `serverIds` since the RPC has no server-side filter param) — the three frontend channels with a real backing RPC now. `devServer.connect`/`disconnect`/`remove`/`testConnection` stay unregistered (no backing RPC exists at all) with a comment explaining why, so the gap reads as intentional rather than an oversight. A real, load-bearing bug avoided: every *existing* channel in this file (`annotation.*`/`task.*`/`git.*`/`automation.*`) calls its downstream client with the bare inbound context, never forwarding tenant identity — copying that pattern would have made every new call fail with `INFRA_NO_TENANT`. The three new handlers explicitly call `gatewaygrpc.AttachIdentity` first; the existing channels' gap is flagged in a comment, not silently fixed (out of this pass's scope) and not silently copied forward either. Frontend field-shape gaps (`DevServer` has no `name`/`status`/`platform`/etc. server-side; `ServerHealthMetrics`' `relayVersion`/`nodeVersion`/`uptimeSeconds` have no proto equivalent) are filled with honest placeholders/nulls, documented in `channels.go`'s new section comment, not fabricated data.

All three needed the same missing piece independently: **no backend-to-backend (as opposed to api-gateway→backend) gRPC call in this codebase forwarded tenant identity on its outbound context.** Each of the three added its own small `withTenantMetadata`/equivalent helper (`tenant.RequireTenantID(ctx)` → `metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID)`) rather than a shared `common/` helper — small enough, and different enough in call-site shape (api-gateway's already has `AttachIdentity` for a different direction), that factoring it out felt premature; worth revisiting if a fourth caller needs the identical thing.

**Verified clean, workspace-wide:** `go build`/`go vet`/`go test` clean for every touched module (`common`, `proto`, `infra-fleet-service`, `git-gateway-service`, `workflow-service`, `api-gateway`), each individually and matching this doc's established per-module verification pattern. `buf lint` clean on the proto change. `gitnexus detect_changes({scope:"all"})` showed no HIGH/CRITICAL risk symbols anywhere in the (much larger, mostly pre-existing-and-unrelated) branch diff.

**Not done in this pass, honestly flagged:** `direct-websocket`/`relay-ssh` modes (see above — deliberately deferred, same reasoning as the first pass); `infra.port_forwards`/`infra.provider_registry_entries` have schema but no consumer; DAG wave-dispatch (workflow-service's separately-tracked gap) still means the new step executors are only reachable via ad hoc execution, not a real template run; **no live infra-fleet-service or Dev Server Agent deployment was available to verify any of this against** — every new code path in this pass is unit- or fake-client-tested, not live-verified, unlike this session's earlier `wscompat` fixes that were checked against `172.20.2.39` directly. Flagged plainly rather than implied equivalent.

## 12. Ninth update, 2026-08-17 — Epic F resolved, Epic G investigated and correctly deferred

**Scope:** closed both remaining Epic F horizontal-scaling blockers, and separately investigated Epic G's stated precondition rather than either building `common/outbox` speculatively or leaving the item untouched.

**The one shared root cause behind both Epic F fixes:** `common/eventbus.Consumer.Subscribe` gives every caller of the *same* `consumerName` a single shared JetStream cursor — a competing-consumers group. That's the right primitive for "process this event exactly once, cluster-wide" (e.g. a domain-of-record write), and it's what every existing consumer in this codebase already uses it for. It is the WRONG primitive for "every replica must independently react to every event" — which is exactly what both `tenant-service`'s cache invalidation and `notification-service`'s broadcast fan-out need, and neither had it: both existing designs (implicitly, in `notification-service`'s case) assumed only one replica would ever need to react. Rather than build two different one-off fixes, `common/eventbus` gained one new shared primitive, `Consumer.SubscribeEphemeral` — an unnamed JetStream consumer with no durable identity, so N processes calling it against the same subject each get their own full copy of every message, no shared cursor, no round-robin. `Consumer.Subscribe` itself is unchanged; this is a pure addition, zero risk to its existing callers.

**`notification-service` — Broadcaster fan-out, fixed without a new subject or sticky routing.** The `Broadcaster` type (`internal/adapter/broadcaster`) was never actually the bug — `Broadcast` correctly fans out to every locally-connected `StreamNotifications` subscriber, and still does, unchanged. The bug was one layer up: `internal/adapter/eventbus.Consumer.Run` gave every replica the same durable consumer name (`"notification-service-" + subject`) for each of the six subscribed subjects, so JetStream delivered each domain event to only ONE replica — that replica's `Broadcast` call fired, but subscribers connected to any other replica never saw it. Switching `Run` to `SubscribeEphemeral` means every replica now independently receives, translates, and broadcasts every event to its own locally-connected subscribers — cluster-wide fan-out, achieved entirely at the NATS layer. This is deliberately NOT what the execution plan's own wording suggested ("republishing every broadcast to a shared NATS subject") — that framing implies an extra publish hop and a new internal subject; achieving the identical end state (every replica sees every event) via per-replica consumer identity on the EXISTING subjects is simpler, avoids a double-fan-out/duplicate-delivery risk entirely, and needed zero changes to `notification-service.md`'s subject table. Documented trade-off: an ephemeral consumer has no durable cursor, so a replica that was down when an event was published never catches up after restarting — this matches, rather than adds to, this service's own already-stated "no offline WS replay queue" design (§2), so it's not a new gap.

**`tenant-service` — profile-cache invalidation, cross-replica for the first time.** `internal/adapter/cache.LRUTTLCache`'s own doc comment already argued against a shared Redis cache (small object, per-user, invalidation set exactly known at write time) — that reasoning holds and Redis was rejected again here. What was missing: `SetUserDepartment`/`AddTeamMember` only ever called `ProfileCache.Invalidate` on the replica that served the write; every OTHER replica's stale entry was correct only by luck of its TTL window (60s) already having elapsed. Fix: both usecases now also call a new `CacheInvalidationPublisher` port (best-effort, swallowed error — a missed publish just means a peer replica falls back to exactly today's TTL-bounded staleness, never wrong data, so this doesn't need outbox-grade delivery guarantees) after their local invalidation succeeds. A new `internal/adapter/eventbus` package (both a `Publisher` and, via `SubscribeEphemeral`, a `Consumer`) round-trips `orca.tenant.profile.invalidated` back to every replica, each of which invalidates its own local cache entry on receipt. tenant-service previously had zero NATS dependency at all (its `config.go` said so explicitly) — this is a new dependency, deliberately wired to degrade gracefully: `main.go` logs a warning and proceeds with cache behavior unchanged (TTL-bounded only) if NATS is unreachable at startup, rather than failing to boot, since tenant-service sits on the critical path for every other service's tenant resolution (§3 Phase 4) and must not crash-loop over an optional dependency.

**Epic G — investigated, not built, and that's the correct outcome.** The item's own stated precondition is "once a second real consumer exists beyond notification-service." Checked both candidate events directly instead of assuming either way: `usage.session.recorded` has zero consumers anywhere in this codebase (its own port doc comment already says so — nothing was ever wired to react to it); `orca.issuetracking.link.created` is absent from `notification-service`'s subscribed-subjects table and unconsumed by anything else — the design doc's "so task-service/project-service can update their own records" describes stated intent, not a built consumer. With no real consumer of either event, a missed publish today costs nothing observable, so building `common/outbox` now would be exactly the speculative build this item's own condition warns against. Same category of resolution as Epic C's auth-service first-run-setup item: a real, reasoned decision not to build, not a skipped task.

**New tests:** `TestSetUserDepartment_BroadcastsInvalidationToOtherReplicas`, `TestSetUserDepartment_PublishErrorDoesNotFailTheWrite`, `TestAddTeamMember_BroadcastsInvalidationToOtherReplicas` (all in `tenant-service/internal/usecase`, against a new `fakeCacheInvalidationPublisher`). `common/eventbus.SubscribeEphemeral` and `notification-service`'s consumer switch have no direct unit tests — both are thin wiring over a real NATS JetStream API with no fake/mock at that layer in this codebase (matching `common/eventbus.Subscribe`'s own pre-existing lack of unit coverage); correctness here rests on the `go vet`-clean build plus the documented reasoning above, not a live NATS run.

**Verified clean, workspace-wide:** `go build`/`go vet`/`go test`/`gofmt -l` clean for all 19 `go.work` modules (each checked individually, matching this doc's established per-module verification pattern). `go mod tidy` run for `tenant-service` after it picked up a new direct dependency on `common/eventbus` (transitively, `nats-io/nats.go`). `gitnexus detect_changes({scope:"unstaged"})`'s aggregate risk (`medium`) is dominated by ~49% proto-regeneration churn and other pre-existing, unrelated work already in this branch — none of this pass's own changed symbols appear with any elevated risk tag or a concerning `affected_processes` entry.

## 13. Tenth update, 2026-08-17 — Epic A, third pass: `direct-websocket` real, `relay-ssh`'s SSH connection layer real (deploy step still blocked)

**Scope, decided explicitly with the user before starting:** the second pass's two remaining checklist items — `direct-websocket` and `relay-ssh` — both needed infra this service had none of at all. This pass closes `direct-websocket` for real. For `relay-ssh`, research surfaced a hard blocker the second pass hadn't fully characterized: the mode's core step is SFTP-deploying and launching `relay.js` on the remote host, and `relay.js` is a Node.js build artifact from `agent/`'s Electron packaging pipeline with **no path reachable from backend-go's build at all** (the TS reference resolves it from `$ORCA_RELAY_PATH`/Electron's bundled `resourcesPath` — paths that only exist inside the Electron app process). Rather than build unverifiable fiction against that missing artifact, or defer the whole mode again with no new ground covered, the user chose a middle path: build the SSH connection + credential-issuance layer underneath relay-ssh for real (genuinely useful and testable on its own), leave the deploy/launch/relay-frame-exchange step explicitly stubbed.

**`direct-websocket` — the "agent dials in" counterpart to relay-websocket's "Orca dials out," built on the same already-shipped codec.** Key finding that simplified this materially versus the second pass's own fear: direct-websocket's handshake (`agent.handshake` over `MessageTypeRegular` JSON-RPC frames) is *structurally identical* to relay-websocket's own handshake, just run in the opposite direction — no new frame/message type was needed, unlike relay-ssh's separate `Handshake=2` version-check protocol.

- `internal/adapter/devserveragent/` (already-shipped code, extended, not replaced): `handshakeInfo`→`HandshakeInfo` and the JSON-RPC types (`jsonrpcRequest`/`Response`/`Error`, `encodeJSONRPCFrame`, `parseJSONRPCResponse`) exported so a new package could reuse them instead of duplicating the codec. `session.go` gained `attachConnection` — the shared tail of `connect()` (install `conn`, mark handshaked, start `readLoop`/`keepAliveLoop`) now factored out so both the outbound-dial path and a new inbound-accept path use it. `session` gained an `inbound bool` marker so `backgroundReconnect` never attempts an outbound `connect()` for a dropped direct-websocket session — there is nothing to dial; the agent must dial in again on its own. `Client.getOrCreateSession` now branches by mode: relay-websocket keeps its original lazy-dial behavior (`getOrDialSession`); direct-websocket only ever waits for an already-attached session (`getInboundSession`) and errors clearly ("the agent must dial in first") rather than attempting to dial anywhere. New `Client.AttachInboundSession(devServerID, host string, conn *websocket.Conn, info HandshakeInfo)` is the one seam a new inbound server calls into — everything past that point (Exec/Health routing, read/keepalive loops) is unchanged, shared code.
- New `internal/adapter/agentwsserver/` — the Go port of `AgentWebSocketServer`/`ws-handshake.ts`'s receiver side + `agent-token-routes.ts`: `Registry` (SHA-256-hashed single-use token slots, 60s connect-timeout expiry, idempotent re-registration, disposer-based cancellation — verified against the exact behaviors `desktop/src/main/dev-server/__tests__/agent-ws-server.test.ts` locks down in TS), `Server` (inbound WS handler at `/agent`: waits 20s for the first frame, requires `agent.handshake`, validates+consumes the token, replies `{ok:true,orcaVersion,sessionId}` or a typed `AuthFailed` (-33101) error + WS close 1008, then calls `AttachInboundSession`), `TokenIssuer` (`POST/GET /api/agent-token`: fail-secure 401 if `ORCA_AGENT_API_SECRET` is unset — never a bypass, matching the TS reference's own BUG-AWS-004 fix — TTL policy `permanent→30d` / `else min(ttl,600)s`, token format `agt-<devServerId>-<unixMilli>`, plaintext debug listing on GET since the registry only ever stores hashes).
- `main.go`: introduced a real `http.ServeMux` (previously `Handler` was `healthSrv.Handler()` directly, with nowhere for a second route to go) mounting the existing health handler at `/` alongside the two new routes — no new port, direct-websocket shares infra-fleet-service's existing HTTP port.

**`relay-ssh` — SSH connection layer built for real, deliberately left unwired.** New `internal/adapter/sshconn/`: generates an ephemeral ed25519 keypair in memory (never persisted, never logged), requests a short-lived certificate for its public half from a new `common/secrets.Client.SSHSignPublicKey(ctx, role, publicKeyOpenSSH)` method (POSTs to Vault's `ssh/sign/<role>`, matching `TransitSign`'s existing error-handling style in that file), builds an `ssh.ClientConfig` from the signed cert via `golang.org/x/crypto/ssh`, dials, and can run a command to prove the connection is genuinely alive. Tested against a real, minimal `ssh.NewServerConn`-based fake SSH server (mirroring `devserveragent/client_test.go`'s `fakeAgent` philosophy — a working counterpart, not a mock) authenticated against a self-generated test CA standing in for Vault, including a "wrong CA rejected" case proving auth is actually enforced. Two gaps carried forward and documented, not silently matched: no host-key verification (`ssh.InsecureIgnoreHostKey()`, same posture as the TS reference itself — not a regression, not a fix either) and no per-target SSH port (`domain.SshTarget` has none; defaults to 22). **`sshconn.Connector` is not imported by `devserveragent`, any usecase, any RPC, or `main.go`** — a deliberate staged increment, the same discipline the first relay-websocket pass itself used (ship something real and tested, wire it up in a later pass) — `Exec`/`Health` for relay-ssh-mode dev servers are unchanged and still return `ErrConnectionModeNotImplemented`.

**Parallelization notes:** the `devserveragent` core changes (exports, `attachConnection`, mode-branching `getOrCreateSession`, `AttachInboundSession`) were made directly, sequentially, before dispatching anything — both `agentwsserver` and `sshconn` needed a stable contract to build against, and `agentwsserver` specifically needed `AttachInboundSession` to already exist. Two background subagents then ran concurrently: one built `agentwsserver` (touching only that new package plus the export-only rename in `devserveragent/jsonrpc.go`), the other built `sshconn` plus the `common/secrets` extension — disjoint file trees, safe to parallelize. `main.go`'s mux merge was done directly afterward, since it's the one place both new packages' wiring actually meets.

**Verified clean:** `go build`/`go vet`/`go test` clean, uncached, for `infra-fleet-service` and `common` (the two touched modules) — `agentwsserver` (27 tests, including `-race`), `devserveragent` (existing suite plus new inbound-session coverage), `sshconn` (3 tests against the fake SSH server), `common/secrets` (3 new `SSHSignPublicKey` tests). `gofmt -l .` clean in both modules. **`api-gateway` and `annotation-service` were mid-edit from unrelated, concurrent work on other epics (JWT/JWKS auth chain; OPA) during this pass** — both fail to build for reasons entirely outside this pass's changed files (confirmed via `git diff --stat` + mtimes on the specific broken files, none touched by this pass); every other of the 19 `go.work` modules, including both this pass actually touched, builds clean individually. Not fixed here — out of scope and not this pass's mess to clean up mid-edit.

**Not done in this pass, honestly flagged:** `relay-ssh`'s deploy/launch/JSON-RPC-frame-exchange step remains entirely unbuilt — blocked on a `relay.js` build artifact this repo's build has no path to, not a Go-code gap; wiring `sshconn.Connector` into `devserveragent.Client`/`GetFleetHealth`/any RPC is a real follow-up once that artifact question is resolved (would also need a `DevServer`↔`SshTarget` linkage this scaffold's domain model doesn't have yet — flagged, not built); no live agent, Vault SSH secrets engine, or real SSH host was available to verify any of this against — every new code path here is unit/fake-server-tested, not live-verified.

**Not done in this pass, honestly flagged:** no live NATS cluster was available to verify the cross-replica behavior end-to-end (e.g. spin up two `tenant-service` processes against one NATS instance and confirm replica B actually invalidates within event latency) — this is unit- and build-verified, not live-verified, same honest caveat this doc's Epic A passes have already flagged for their own NATS-dependent code. `common/eventbus`'s new `SubscribeEphemeral` has no dedicated unit test (see above). At the time this section was written, Epic G was still genuinely open — see §13 below for what changed the same day.

## 13. Tenth update, 2026-08-17 — Epic G built in full, on explicit user direction

**Why this section exists at all:** §12 above investigated Epic G and reported back "precondition not met, correctly not built" — a real, reasoned decision, not a placeholder. The user then explicitly asked for Epic G to be executed regardless. That's the user's call to make, not something to second-guess by building it half-heartedly or re-litigating the earlier recommendation — this section is the honest record of what got built once that direction was given, including a second real decision point (`issue-tracking-service`'s database question) that came up along the way and was surfaced to the user rather than decided alone.

**`common/outbox` — the shared relay, built once, matching the architecture doc's shape exactly.** One package, `Store` (a port every service's own `internal/adapter/postgres` implements against its own outbox table — no shared schema, matching the database-per-service rule) and `Relay` (polls `Store`, publishes via `*common/eventbus.Publisher`, marks published — `PollInterval`/`BatchSize` configurable, a full batch triggers an immediate re-poll instead of waiting out the tick). Deliberately owns only the poll loop, not the SQL or the enqueue-inside-a-transaction step — those stay in each service's own adapter, per this codebase's established hand-written-SQL-per-service convention. Safe for every replica of a horizontally-scaled service to run its own `Relay` against the same `Store`: `MarkPublished` only ever follows a successful publish, so a row published twice by two racing replicas is ordinary at-least-once, not a bug.

**`usage-service` — the textbook case, wired exactly as the architecture doc describes.** `RecordUsageSession` no longer calls an `EventPublisher` after the DB write returns; instead `Repository.SaveSession` now enqueues a `usage.outbox_events` row in the SAME transaction as the session insert and rollup upsert (migration `0002_outbox`). A nice side effect: the old "session saved but event publish failed" partial-success return is gone — now it's just "the transaction committed or it didn't." The old `internal/adapter/eventbus` publisher package is deleted; `common/outbox.Relay` (started in `main.go`, waited-on during graceful shutdown same as every other background goroutine in this doc's pattern) is what actually reaches NATS now.

**`issue-tracking-service` — the real fork, surfaced to the user rather than decided alone.** True transactional outbox needs a domain-state write and an outbox-row write in one Postgres transaction — this service has never had a database at all, on purpose (Jira/Linear are its systems of record, design doc §2/§5). Two options existed: leave `LinkIssue`'s existing direct-publish-or-fail-the-RPC behavior alone (already correct given no DB to be transactional with — nothing to durably lose on a publish failure that wasn't already lost) or give this service a database purely to host an outbox table (a real architecture reversal of its stated design, bigger than what Epic G's own wording asked for). Asked the user directly rather than picking one — **user chose to add the database.** Built as the smallest version of that choice: `issuetracking.outbox_events` is the ONLY table (migration `0001_outbox` — this service's first migration ever), `Enqueue` is a single `INSERT` (there is no separate domain-state write to wrap it with — the outbox row IS the entire write, documented explicitly in `internal/adapter/postgres`'s package comment so this doesn't read as a misunderstanding of the pattern later). `DATABASE_DSN` is now required to boot, a real behavior change from before. `LinkIssue` now fails closed on a database problem instead of a NATS problem — an actual improvement, since NATS being briefly down no longer means the caller has to retry the whole RPC, only the relay has to retry.

**Deploy config kept in sync, not left to drift:** `deploy/dev/docker/postgres/init-databases.sh` gained `issuetracking` in its database list; `deploy/dev/docker-compose.yml`'s `issue-tracking-service` block gained `DATABASE_DSN` + a `postgres: service_healthy` dependency (moving it out of the "N services with no database" comment group), and a `migrate-issuetracking` one-shot container was added alongside the other 13 (now 14) `migrate-*` blocks.

**New tests:** `usage-service`: `TestRecordUsageSession_SavesAndEnqueuesOutboxEvent` (renamed/rewritten from the old direct-publish test), a new `TestRepository_Outbox_EnqueueFetchMarkPublished` integration test (enqueue → fetch → mark → fetch-again-empty, the exact cycle `Relay` drives). `issue-tracking-service`: `TestLinkIssue_EnqueuesLinkCreatedEvent`/`TestLinkIssue_EnqueueFailurePropagates`/`TestLinkIssue_NilEnqueuerFailsClosed` (renamed/rewritten from the old publish-based tests), plus its own new `TestRepository_Outbox_EnqueueFetchMarkPublished` integration test.

**Verified clean, workspace-wide:** `go build`/`go vet`/`go test`/`gofmt -l` clean for all 19 `go.work` modules (checked individually, matching this doc's established per-module sweep). `go vet -tags=integration ./...` also clean for both `usage-service` and `issue-tracking-service` (the two new outbox integration tests compile correctly, even though they weren't run — no Docker in this sandbox, same caveat every prior integration-test-touching pass in this doc has flagged). `go mod tidy` run for both services after picking up `common/outbox`/`jackc/pgx` (new for `issue-tracking-service`, which had never touched Postgres before) as dependencies. One unrelated, pre-existing build failure was found during the final sweep in `annotation-service` (a test file calling `NewAnnotation` with a stale argument count) — confirmed via `git diff`/`git log` that this predates and is unrelated to this pass's own changes (concurrent, separate work already in this branch, matching this doc's own established "found but not mine, not silently claimed as fixed or broken by this pass" precedent) — flagged here, not touched.

**Not done in this pass, honestly flagged:** no live Postgres or NATS was available to verify either service's outbox round trip end-to-end (enqueue → relay picks it up → NATS delivers → a real consumer acts on it) — both new integration tests are written and `-tags=integration`-compile-verified, not run, same caveat this doc's other testcontainers-dependent passes already carry. Epic G's own stated precondition — a second real consumer for either `usage.session.recorded` or `orca.issuetracking.link.created` — is STILL not met after this pass; the outbox now exists and is correct, but nothing downstream consumes either event yet. That remains real, separate, unstarted scope.

## 14. Eleventh update, 2026-08-17 — §3 Phase 1 (all 4 leaf services) closed

**Context this pass started from — the doc was already stale, confirmed rather than assumed:** before touching anything, re-checked the working tree against this doc and found substantial undocumented work already landed by other concurrent sessions (this repo had ~17 concurrent sessions active during this pass, confirmed via the harness's own session list) — `common/jwtauth`/`common/policy` + a real `policy/orca-authz/*.rego` bundle (Epic D/E, both still `[ ]` in §2 at the time), and `annotation-service`'s OPA author-only check already fully wired against it. Also caught `usage-service/cmd/server/main.go` and this very file changing content between two reads a few tool-calls apart in this session — live, real-time concurrent edits, not a one-off. Given that, every file below was re-read immediately before writing to it, not trusted from an earlier read in this pass; where a live collision looked possible, the write was skipped and flagged rather than guessed.

**`usage-service`:**
- `common/secrets.DatabaseCredentialsFromFile` wired into `main.go`/`internal/config/config.go` — new `DATABASE_CREDENTIALS_FILE` env (default `/vault/secrets/database-credentials`), falling back to `DATABASE_DSN` itself when the file doesn't exist, matching the helper's own documented fallback.
- OTLP exporter — implemented in **shared `common/tracing/tracing.go`**, not per-service (the gap is identical in all 17 services' `Init` call, so fixing it once here closes it everywhere): `Init` now batches spans to a real `otlptracegrpc.New` exporter over insecure/plaintext gRPC whenever `otlpEndpoint` is non-empty; behavior is byte-for-byte unchanged (exporter-less, always-sample) when it's empty, so local dev/unit tests are unaffected. `impact({target:"Init", direction:"upstream"})` flagged this CRITICAL (17 direct callers) before editing, per this repo's impact-analysis rule — surfaced to the user before proceeding; the fix stayed additive-only specifically to contain that blast radius.
- `sqlc` migration — **deliberately not done**, per this doc's own §5: "worth doing as one focused pass across all services... not per-service ad hoc." Doing it just for `usage-service` here would have contradicted that guidance.

**`annotation-service`:** OPA author-only check was already real and wired by the time this pass started (a concurrent session's work, verified rather than assumed or redone). What was still open: `request_id` idempotency. Migration `0002_annotation_request_id` adds `request_id TEXT NOT NULL` + `UNIQUE (tenant_id, request_id)` (existing rows backfilled with `gen_random_uuid()::text` so the constraint can apply uniformly, not just to post-migration rows). `domain.NewAnnotation` gained a `requestID` param + `ErrEmptyRequestID`; `CreateAnnotation` now requires it and checks a new `Repository.FindByRequestID` before inserting — a retry with the same `(tenant_id, request_id)` returns the original annotation instead of a duplicate, including a re-check on a unique-constraint insert failure to absorb a genuine race. This mirrors `automation-service.RunNow`'s idempotency pattern exactly, chosen as the reference implementation since it's this codebase's only other check-then-insert-with-race-recheck example. 2 new domain tests, 4 new usecase tests (idempotent retry, distinct-request-ids, requires-request-id, plus updating every existing seed-create call site across `list_annotations_test.go`/`update_annotation_test.go`/`delete_annotation_test.go`), 1 new integration test (`FindByRequestID` round-trip + tenant scoping).

**`notification-service`:** built by a background subagent (dispatched in parallel with `issue-tracking-service`'s, on explicit user request to increase parallelism). Migration `0002_processed_events` adds `notification.processed_events(event_id UUID PRIMARY KEY, subject TEXT NOT NULL, processed_at TIMESTAMPTZ NOT NULL DEFAULT now())`, matching notification-service.md §5 exactly. `HandleIncomingEvent` now calls a new `ProcessedEventRepository.MarkProcessed` — implemented as `INSERT ... ON CONFLICT (event_id) DO NOTHING` with `RowsAffected() == 0` as the redelivery signal — atomically, before decode/translate/broadcast; a redelivery is a debug-logged no-op, not an error. The atomicity matters specifically because this service's `SubscribeEphemeral` consumer (Epic F, §12) gives every replica its own independent JetStream consumer, so a genuine concurrent-replica race on a redelivered message is a real scenario here, not just a same-process retry. Dedup key is the domain event's own `EventID` (from `common/eventbus.Event.ID`), not the JetStream sequence number. 2 new usecase tests (redelivery no-ops, distinct event ids both process), 2 new integration tests, both run for real against Docker (available in that subagent's sandbox) — passed. **Found, not caused by this pass:** 3 pre-existing integration tests (`SaveSubscription`/`ListByUser`/`GetPublicKey`) fail with `invalid input syntax for type uuid` — the same non-UUID-literal-fixture-vs-UUID-column bug §11 already found and fixed in `workflow-service`'s tests, here still unfixed in `notification-service`'s pre-existing tests; flagged, not fixed (out of this task's scope). **Not done:** the `processed_events` retention/pruning job notification-service.md §8 mentions (~7-day window) — no cleanup usecase was asked for or built; the table grows unbounded without one.

**`issue-tracking-service`:** also built by a background subagent, in parallel with the above. Scoped deliberately to avoid a `proto/` change — with concurrent sessions active repo-wide, regenerating shared `proto/gen/` code is a known collision hazard this doc's own §10 already called out for exactly this reason, and the Phase 1 item's literal ask ("add a real `ListIssueTypes` lookup") doesn't require a new public RPC to satisfy. Added an unexported `listIssueTypes`/`resolveIssueType` pair to the existing Jira adapter (`internal/adapter/jira/client.go`), calling Jira Cloud's `GET /rest/api/3/issue/createmeta/{projectKey}/issuetypes` (the current, non-deprecated per-project endpoint — chosen over the older `createmeta?expand=projects.issuetypes` sketch in the design doc, which Atlassian has since deprecated, and over `GET /rest/api/3/issuetype/project?projectId=...`, which needs a numeric project id this adapter's existing `ListIssues`/`CreateIssue` convention doesn't otherwise resolve). Resolution policy: exact case-insensitive match on `"Task"`, else the first non-subtask issue type, else a clear error — `CreateIssue` now uses this instead of the bare hardcoded string. No caller-supplied override exists (`CreateIssueRequest` has no issue-type field in the generated proto, and this pass didn't add one, per the constraint above) — flagged as the honest remaining gap, not silently worked around. 7 new tests against a `httptest.Server` fake Jira, including one that specifically asserts the POST body carries the *resolved* (real, possibly differently-cased) type name, not the literal string `"Task"`.

**Parallelization notes:** `usage-service`'s Vault-DSN wiring and `common/tracing`'s OTLP change were done directly and sequentially first (both small, and `tracing.go` needed to be stable before any service's `main.go` change could be verified against it). `annotation-service` was also done directly — its full context (schema, domain, repo, usecase, gRPC, 3 test files) was already loaded from investigating the request_id gap, so handing it to a subagent would have meant re-deriving that context from scratch for no benefit. `notification-service` and `issue-tracking-service` were dispatched as two background subagents once the user asked to increase parallelism — safe to run concurrently since they touch fully disjoint service trees and neither touches `proto/`, `common/`, or each other's files.

**Verified clean:** `gofmt -l`/`go build`/`go vet` for `usage-service`, `annotation-service`, `notification-service`, `issue-tracking-service`, and `common` — all clean, each checked independently by this pass (not just trusted from the subagents' own reports). `go vet -tags=integration ./...` also clean for the three services with integration tests. A workspace-wide sweep across all 19 `go.work` modules found 4 modules **not** touched by this pass already broken from other concurrent sessions' in-flight work (`automation-service`: a domain/usecase test calling `NewAutomation` with a stale argument count; `infra-fleet-service`: an unused `"bytes"` import in `devserveragent/client.go`; `project-service`: a fake repository missing a new `DeleteProject` method; `workflow-service`: `main.go` calling `usecase.NewExecute`/`NewExecuteAdHocStep` with a stale argument count) — none in this pass's scope, none touched, flagged here per this doc's own established "found but not mine" precedent (§13). `gitnexus detect_changes({scope:"unstaged"})` reports aggregate `risk_level: "medium"` across the full 244-file/1146-symbol repo-wide diff — dominated by the same pre-existing, unrelated concurrent work as every prior pass's equivalent check; no HIGH/CRITICAL-risk symbol appeared in this pass's own changed set.

**Not done in this pass, honestly flagged:** none of the four services' new Postgres-backed code paths (annotation's `FindByRequestID`, notification's `MarkProcessed`) were live-verified against a running instance by me directly — `notification-service`'s integration tests were the exception, run for real against Docker inside that subagent's own sandbox. `sqlc` migration remains open for all 17 services, per §5, same as before this pass. §13's note that `annotation-service` had "a test file calling `NewAnnotation` with a stale argument count" is now resolved by this pass's own `request_id` work (which touched exactly that call site) — not a separate fix, just this pass closing a gap another section had already flagged.

## 15. Twelfth update, 2026-08-17 — §3 Phase 2 (all 5 mid-tier services) closed, Phase 3 (both gateway-facing services) closed early at explicit user request

**Scope and sequencing:** ran §3 Phase 2 in full on direct user request ("thực thi toàn bộ Phase 2"), then Phase 3 as well, ahead of its normal place in the phase ordering, on the user's explicit follow-up request for more parallelism once Phase 2's 5 agents were already in flight — a deliberate scope expansion the user asked for, not assumed. Foundational, shared-surface changes (`common/secrets` new methods, all proto edits) were made directly and sequentially first, matching this doc's own established discipline (§10's parallelization notes) for avoiding a `buf generate` race between concurrent agents — only after those landed and the `proto`/`common` modules were confirmed building clean were 7 background agents dispatched, one per service, each with a disjoint file tree and explicit instructions not to touch `proto/` or any other service.

**Context this pass started from, confirmed rather than assumed:** per this doc's own §14 precedent, checked for concurrent-session drift before and after — `ListAgents` showed 17 concurrent peer sessions active on this repo throughout. §14's own final sweep (written by a concurrent session) had flagged `automation-service`/`infra-fleet-service`/`project-service`/`workflow-service` as broken from OTHER in-flight work at the time it ran; this pass's own final workspace-wide sweep (below), run after all 7 agents completed, found all four clean — each was independently rewired by this pass's own agents (e.g. workflow-service's agent rewrote `main.go`'s `NewExecute`/`NewExecuteAdHocStep` call sites as part of its own wave-dispatch work), so those breaks were resolved as a byproduct, not separately chased down. `execution-plan.md` itself was found modified by a concurrent session mid-edit (§14 had already claimed the section number this pass originally reached for) — resolved by re-checking the file's tail immediately before every write and renumbering to the next free section, per §14's own "re-read immediately before writing, skip and flag on a live collision" discipline.

**`common/secrets` (foundational, shared by credential-broker-service):** two new methods — `Ping(ctx) error` (real `Sys().Health()` call) and `KVDestroyMetadata(ctx, mount, path) error` (real KV v2 native destroy-all-versions, `DELETE <mount>/metadata/<path>`).

**Proto changes (foundational, made directly, one `buf generate` pass, verified clean before any agent started):** `automation.proto` gained `HandleExternalTrigger` + `step_type`/`enabled`/`dtstart`/`timezone` on `Automation`/`CreateAutomationRequest` (reusing `orca.workflow.v1.StepType` rather than a duplicate enum). `project.proto` gained `UpdateProject`/`DeleteProject` + full `Repo`/`Worktree`/`ProjectGroup` CRUD surface, and `Project` gained `description`/`default_branch`/`visibility`/`created_by`/`created_at`/`updated_at`. `infrafleet.proto` gained `DevServer.ssh_target_id`/`RegisterDevServerRequest.ssh_target_id`. `scm-integration-service`'s agent independently needed and made its own proto change (4 new RPCs for the OAuth flow) — safe, since it was the only agent touching that file.

**`credential-broker-service`:** `RevokeCredential` now calls the real `KVDestroyMetadata` instead of an empty-payload `KVWrite` overwrite; `Ping` now calls the real `Sys().Health()`. Metadata+audit writes for `WriteCredential`/`RotateCredential`/`RevokeCredential` are now atomic via a new `usecase.TxRunner` port (`RunInTx`, implemented over `pgx.BeginFunc`) — a failed audit append now rolls back the metadata mutation, proven by a new test. `x-orca-service-id` → mTLS/SPIFFE stays correctly deferred (needs a service mesh that doesn't exist yet) — not attempted, per instruction.

**`automation-service`:** real scheduler (`internal/adapter/scheduler`, ticker + `SELECT ... FOR UPDATE SKIP LOCKED` claim so replicas don't double-dispatch, `next_run_at`/`dtstart`/`timezone`/`enabled` columns via migration `0002`), `HandleExternalTrigger` implemented (`trigger=external`, caller's own `request_id` used verbatim), `step_type` promoted off the `step_config_json` JSON blob onto a real column, read directly instead of via `domain.ParseStepType` (removed). 38 unit tests + 4 real-Postgres integration tests (testcontainers, actually run) covering the `SKIP LOCKED` concurrent-claim behavior specifically.

**`workflow-service` — the highest-risk item, DAG wave-dispatch:** `domain.DAGDefinition.BuildWaves` (real Kahn's-algorithm topological sort + general multi-node cycle detection, closing the gap `Validate`'s direct-self-reference-only check left open). New `StepExecution` persistence (migration `0004`) and a bounded worker pool (cap 10, per workflow-service.md §8) dispatching each wave's steps concurrently, gated on the previous wave's steps all reaching a terminal status — proven deterministically in tests via a channel-based gate, not sleep-and-hope. `Execute` now dispatches asynchronously on a detached background goroutine after synchronous DAG validation, a documented architectural decision (steps can run up to 30 minutes per §8; blocking the RPC on the whole DAG wasn't viable). `ExecuteAdHocStep`'s persistence gap (no synthetic execution/step_execution row) closed as a natural extension, needing `workflow.executions.template_id` to go nullable (migration `0005`). Boot-time recovery scan remains honestly unimplemented — flagged, not attempted, matching its pre-existing README gap.

**`infra-fleet-service`:** `DevServer.ssh_target_id` links a dev server to its `SshTarget` (migration `0003`), with a new invariant (`ConnectionModeRelaySSH` requires a non-empty `SSHTargetID`). `sshconn.Connector` (built standalone in Epic A's third pass, deliberately unwired then) is now wired into `devserveragent.Client` via a new `WithRelaySSH` option: `Health` does a real dial+probe+close, `Exec` supports `method="shell.exec"` for real (params `{"script", "env"}` — verified against the actual caller's wire shape, not the task's initial guess of `{"command"}`) and returns a typed error for every other method, honestly, since no JSON-RPC agent exists on the other end without the still-unreachable `relay.js` artifact. `main.go` does not yet call `WithRelaySSH` — needs Vault construction in this service's `main.go`, a separate pre-existing gap, correctly not forced into scope here.

**`project-service` — the largest single scope:** `UpdateProject` (empty-field-means-no-change, `dev_server_id` deliberately excluded), `DeleteProject` (fail-closed, reusing the existing `WorkflowExecutionChecker`/`TaskExecutionChecker` guards — `ON DELETE CASCADE` on repos/worktrees/members, documented), plus full Repo (`AddRepo`/`ListRepos`/`ReorderRepos`/`RemoveRepo`), Worktree (`RecordWorktreeCreated`/`RecordWorktreeRemoved`/`ListWorktrees`/`SetWorktreeActivation`/`RenameWorktree`), and ProjectGroup (`CreateProjectGroup`/`UpdateProjectGroup`/`DeleteProjectGroup`/`ListProjectGroups`, self-referential, no parent-rewiring on update — mirroring workflow-service's own no-cycle-detection-needed precedent for templates) surfaces — 21 new usecases total. Source-projects, `GetProjectContext`, membership beyond `AddMember`, and OPA correctly stayed out of scope (not asked for in this task). Found and fixed 3 real, pre-existing-pattern bugs along the way: a duplicate-column migration, `COALESCE(uuid_col, '')` failing to parse in Postgres (needs an explicit `::text` cast — also fixed on the pre-existing `dev_server_id` column), and new `scan*` helpers double-converting `pgx.ErrNoRows` in a way that silently defeated callers' own not-found checks. 73 unit tests + 17 real-Postgres integration tests, all passing.

**`git-gateway-service` (Phase 3):** `ConnectionResolver`/`RelayExecutor` re-verified as genuinely already wired (not just trusted from the doc's own earlier claim). `GenerateCommitMessage` now relays the worktree's staged diff to the Dev Server Agent's real `ai.complete` RPC via a new `RelayExecutor.Complete` — the method name and param/result shape (`prompt` in; `{content, model?}` out) were verified against `specs/agent/api/agent-rpc-catalog-runtime.md`'s real handler, not guessed, correcting the stale doc-comment claim that this RPC was blocked on missing AI-inference wiring (it wasn't, once Epic A's `Relay` RPC existed). A worktree with no relay connection returns a clear `FailedPrecondition` rather than any silent local fallback.

**`scm-integration-service` (Phase 3):** all 5 providers (GitHub/GitLab/Bitbucket/Azure DevOps/Gitea) now make real HTTP calls for `ListIssues`/`CreatePullRequest`/`ListPullRequests`/`GetRateLimitStatus`, with two honest, documented capability gaps (Azure DevOps has no native issue concept; Gitea has no rate-limit concept) returning a typed `ErrCapabilityUnsupported` rather than faking a response. §9.1's OAuth-web-flow-vs-PTY-CLI-login question was decided (real OAuth 2.0 authorization-code web flow — `StartOAuthFlow`/`CompleteOAuthFlow`/`GetAuthStatus`/`RevokeAuth`, state as a stateless HMAC-signed token, no new DB table; PTY/CLI login was rejected specifically because it would reintroduce TS Gap 1's original mechanism) and built, with credentials written via the already-real `credential-broker-service.WriteCredential`. `rate_limit_cache` is real and read/write-through on every `GetRateLimitStatus` call; `webhook_delivery_log` is schema-only (no webhook-receiving RPC exists yet to write to it — flagged honestly, not invented speculatively, matching Epic G's own established precedent for this kind of gap). `RevokeAuth` cannot actually revoke a stored credential yet — needs a `RevokeCredentialByOwner`-shaped RPC on `credential-broker-service` that doesn't exist; flagged rather than built, since that service wasn't in this workstream's scope.

**Verified clean, workspace-wide:** `gofmt -l`, `go build ./...`, `go vet ./...` for all 19 `go.work` modules — clean (checked individually right after all 7 agents completed, not just trusted from each agent's own report). `go test ./...` for all 19 modules — clean, zero failures. Each agent additionally ran real `-tags=integration` Postgres tests against testcontainers where the pattern existed for its service (credential-broker-service, automation-service, workflow-service, infra-fleet-service, project-service, scm-integration-service all report real integration runs, not just compile checks — git-gateway-service's change didn't need one). `gitnexus detect_changes({scope:"all"})` was run; see its own summary below rather than re-quoted here given its size (370K+ characters).

**Not done in this pass, honestly flagged:** the mTLS/SPIFFE service-identity item (credential-broker-service) and the `relay-ssh` deploy/launch step (infra-fleet-service) remain exactly as blocked as before — both need infrastructure (a service mesh; a `relay.js` build artifact) this repo's build has no path to, not a Go-code gap this pass could have closed. Boot-time execution recovery (workflow-service), OPA authorization on any of project-service's new RPCs, and `RevokeAuth`'s actual revocation (scm-integration-service, blocked on a credential-broker-service RPC that doesn't exist) are real, separate, unstarted follow-ups. No live Vault, live SSH host, live Dev Server Agent, or live NATS cluster was available in this environment to verify any of the Vault-dependent or agent-relay-dependent code paths end-to-end — every new path is unit- or fake-server-tested, matching this doc's own established honesty discipline for every prior Epic A/B pass.

## 16. Thirteenth update, 2026-08-18 — §3 Phase 4 (identity/tenancy) and Phase 5 (edge) closed

**Scope:** ran §3 Phase 4 and Phase 5 in full on direct user request ("thực
thi toàn bộ Phase 4 — Identity/tenancy và Phase 5 — Edge").

**Context this pass started from, confirmed rather than assumed:** per
§14/§15's own precedent, checked the working tree against this doc before
touching anything rather than trusting the (stale) checklists. Found Epic D
(JWT/JWKS: `common/jwtauth`, `auth-service`'s Vault-Transit `TokenSigner` +
`GetJWKS`, `api-gateway`'s real `JWKSClient`) and Epic E (OPA:
`common/policy.Evaluator`, a real `policy/orca-authz/*.rego` bundle, wired
into `auth-service`/`task-service`/`annotation-service`) already landed by
concurrent sessions — confirmed real, not aspirational, via a clean
`go build`/`go vet`/`go test` across `common`, `auth-service`,
`api-gateway`, `tenant-service`, `task-service`, `annotation-service`
before writing anything. `ListAgents` showed 18 concurrent peer sessions
active on this repo throughout this pass, consistent with §14/§15. This
materially narrowed the actual remaining work to: tenant-service's team
settings gap, this doc's own stale bookkeeping, and Phase 5's REST-route
wiring — see below for what was genuinely built in this pass versus what
was found already done.

**`tenant-service` — the one genuine Phase 4 build:** `tenant.proto`'s
`CreateTeamRequest`/`Team` gained `settings_json` (matching `Company`'s/
`Department`'s existing convention); `CreateTeamInput` gained a `Settings`
field threaded into `domain.NewTeam`; the gRPC adapter gained an
`unmarshalSettings` inbound counterpart to its existing `marshalSettings`,
and `toProtoTeam` became fallible to marshal settings back out. The
4-layer merge algorithm (`ResolveProfile`) needed **no changes** — it
already consumed `Team.Settings`, just was never fed a non-empty value.
`gitnexus impact({target:"CreateTeam", direction:"upstream"})` reported LOW
risk (contained to `NewCreateTeam`/`main.go`) before editing, per this
repo's impact-analysis rule. New test (`TestCreateTeam_PersistsSettings`)
covers the round trip through the fake repository.

**Phase 5 — REST route wiring, the largest piece of this pass:** dispatched
12 parallel background agents, one per downstream service, each confirmed
to have a real (non-`Unimplemented`) RPC surface first — `ai-provider-service`
and `orchestration-service` needed an explicit maturity check before
inclusion (both clean: every RPC has a real usecase, `go build`/`go test`
green) since neither was called out as done in §14/§15. Each agent wrote
only its own `internal/adapter/httpgateway/<service>_routes.go` +
`_test.go`, following the pre-existing `usage_routes.go` pattern
(`identityFromContext`/`gatewaygrpc.AttachIdentity`/`writeGRPCError`, tenant/
user identity always from context, never the request body) — mirroring
§10/§15's own "avoid a shared-file race between concurrent agents"
discipline, since `router.go`/`main.go`/`registry.go` were reserved for a
sequential pass afterward. Wired for real: `notification-service`,
`annotation-service`, `task-service`, `automation-service`,
`infra-fleet-service`, `git-gateway-service`, `auth-service` (admin
console — `/v1/auth`, distinct from the already-real `/auth/*` login
routes), `tenant-service`, `project-service`, `issue-tracking-service`,
`ai-provider-service`, `orchestration-service` — 12 of 14 remaining
services on the first wave. This pass's own maturity check initially ran
ahead of re-reading §15's account of Phase 2/3 closure, so
`scm-integration-service`/`workflow-service` were left stubbed at first —
caught before this section was finalized (not backdated after the fact):
both are genuinely real per §15 (all 5 SCM providers; DAG wave-dispatch),
so two more agents wired `mountSCMRoutes`/`mountWorkflowRoutes` the same
way, closing all 14 of 14. `registry.go`'s `NewDefaultServiceRegistry` now
has zero `RouteStubbed` entries — every prefix is real. The 501-stub
fallback path (`mountStubRoutes`) itself is still real code, still tested
(`router_test.go` now builds a synthetic one-rule stubbed registry for that
test rather than relying on any specific real service staying unwired),
just currently unexercised by the default registry.

**Shared-file integration (done sequentially, by this pass directly, after
all 12 agents landed):** `registry.go`'s `NewDefaultServiceRegistry` flipped
12 prefixes from `RouteStubbed` to `RouteWired`; `router.go`'s `Deps`
gained one client field per service, each `mountXRoutes` call guarded on
its client being non-nil; `main.go` gained 5 new `gatewaygrpc.Dial` calls
(`tenant-service`/`project-service`/`issue-tracking-service`/
`ai-provider-service`/`orchestration-service` — the other 7 were already
dialed for `wscompat`) plus matching `/readyz` health-check registrations.
Two small cross-cutting fixes found and closed during this integration
pass: `grpcCodeToHTTPStatus` (shared by every `mountXRoutes` file) had no
`codes.Unimplemented` case, mapping it to a misleading 500 instead of 501
— added. `router_test.go`'s pre-Phase-5 stub-fallback test asserted 501 on
`/v1/tasks`/`/v1/projects`/`/v1/notifications`, paths that are now real
routes — updated to exercise `scm-integration-service`/`workflow-service`
(the prefixes still actually stubbed) instead, and its
unauthenticated-request test moved off `/v1/tasks` (now unmounted-without-
a-client, hence a 404 that isn't what that test means to check) onto the
always-stubbed `/v1/scm` path.

**`api-gateway`/auth-service doc cleanup:** `api-gateway`'s
`usecase.JWKSClient` port doc comment still said "NOT IMPLEMENTED in this
scaffold" despite `internal/adapter/authclient.JWKSClient` being a real
implementation — fixed. `tenant-service`'s README "Known gaps" entry for
team settings updated to reflect the fix above.

**Explicitly not built, recorded rather than silently skipped:** SSO
(`auth-service`) — confirmed still a product decision, not attempted, per
this doc's own standing instruction not to build an OIDC flow
speculatively. Refresh-token/`IssueToken`/`RevokeToken`/access-policy-CRUD
proto surface — remains an open "decide if actually needed" question, not
decided on the user's behalf. `IssueServiceToken`'s caller-identity gap (no
check that the requester is authorized to mint a token for the target
user) — a real README-flagged footnote, not a Phase 4 table item; left
alone, since the original author explicitly scoped it out pending a proto
change. `api-gateway`'s own coarse-grained per-endpoint OPA check (design
doc §9) and OPA bundle hot-reload — both real Epic E gaps, out of this
pass's table-defined scope.

**Verified clean, workspace-wide:** `go build`/`go vet`/`go test`
individually clean for all 16 `go.work` modules this pass touched or
depends on (`common`, `ai-provider-service`, `annotation-service`,
`api-gateway`, `auth-service`, `automation-service`,
`credential-broker-service`, `git-gateway-service`, `infra-fleet-service`,
`issue-tracking-service`, `notification-service`, `orchestration-service`,
`task-service`, `tenant-service`, `usage-service`, `workflow-service`,
`scm-integration-service`). `project-service` was caught mid-build-break
partway through this pass — `cmd/server/main.go` briefly didn't pass the
`OPAClient` parameter several `usecase.NewXxx` constructors had just
started requiring, confirmed via `git diff` to be a concurrent session's
in-flight OPA-wiring work (§15 had flagged this as a real, unstarted
follow-up; §17 below is that same session's completed writeup) — flagged
rather than touched at the time, and re-confirmed clean by the end of this
pass once that session finished. `mcp__gitnexus__detect_changes({scope:"unstaged"})` was attempted
but exceeded the tool's output budget at repo scope (18 concurrent
sessions' combined diff) — per this repo's own "stop on saturation" rule,
not paginated through; this pass's own change surface was instead
confirmed directly via targeted `git status`/`git diff --stat` against the
specific files it touched.

**Not done in this pass, honestly flagged:** No live Postgres/Vault/NATS was available to
verify any of this pass's routes end-to-end against a running downstream
service — every new route is unit-/fake-client-tested, matching this
doc's established honesty discipline for every prior pass. All of this
pass's work remains **uncommitted**, on `fix/pty-session-expired-on-pane-remount`
— an unrelated frontend PTY bugfix branch, same as the rest of the
already-uncommitted pile this pass found on arrival; left for the user to
decide when/how to commit, on which branch.

## 17. Fourteenth update, 2026-08-18 — three of §15's five "honestly open" items closed, on explicit user request ("continue fix honestly open")

**Scope and sequencing:** §15 closed after listing five items as correctly deferred rather than built. Re-examined each on request rather than re-attempting all five uniformly: two are genuine infrastructure blockers unchanged since §15 (credential-broker-service's `x-orca-service-id` → mTLS/SPIFFE needs a service mesh this repo has none of; `infra-fleet-service`'s `relay-ssh` deploy/launch step needs a `relay.js` build artifact this repo's build has no path to) — neither was attempted here, faking either would mean building unverifiable fiction against missing infrastructure, the same reasoning §13's `relay-ssh` writeup already used. The other three had no real blocker, just hadn't been built yet, and — separately discovered before starting — one of their own stated preconditions had changed underneath them: Epic E's OPA foundation (§2), "no service calls OPA yet" as of when that Epic was written, was found **already real** (`common/policy`, `auth-service.requireAdminActor`, `annotation-service`'s author-only check, a `policy/orca-authz/` Rego bundle with `task_grant.rego` already in it) — built by a concurrent session, not this pass, and not re-verified or claimed here beyond what the `project-service` workstream below directly observed and built on top of. Foundational work (the new `RevokeCredentialByOwner` RPC) was added directly and sequentially first, same discipline as every prior multi-service pass in this doc, before dispatching 3 parallel background agents on the remaining disjoint work.

**`credential-broker-service` + `scm-integration-service` — `RevokeCredentialByOwner`, closing `scm-integration-service`'s `RevokeAuth` gap for real.** New RPC, mirroring `ResolveCredentialByOwner`'s lookup key `(tenant_id, category, owner_id)` but revoking instead of resolving — same real Vault-side `RevokeSecret` call `RevokeCredential` makes, same atomic metadata+audit transaction via the `TxRunner` port §15 added. One real, documented asymmetry versus by-id revoke: `GetByOwner`'s query already filters revoked rows out at the SQL level (an intentional pre-existing property, see §9's writeup), so `RevokeCredentialByOwner` can never distinguish "already revoked" from "never existed" the way by-id `RevokeCredential` can — both surface as `CREDENTIAL_NOT_FOUND`, proven by a dedicated test rather than special-cased away. `scm-integration-service.RevokeAuth` — previously unable to revoke anything at all, per §15's own honest flag — now calls through for real.

**`workflow-service` — boot-time execution recovery scan, the last piece of §8's resumability requirement.** `execute.go`'s own doc comment named this gap explicitly ("a boot-time recovery scan re-attaching to root_trace_id, not implemented in this pass"). New `RecoverExecutions` usecase runs once at `cmd/server/main.go` startup, before RPCs are accepted: lists every `status=running` execution process-wide (the one genuinely non-tenant-scoped port method in this codebase, documented as such — a boot scan by definition runs before any request-scoped tenant is known), rebuilds each one's DAG from its template (not a snapshot — `WorkflowExecution` has no snapshot field, verified by reading it rather than assumed), finds the first wave not fully `completed`, and resumes dispatch from there via a generalized `waveDispatcher.dispatchWavesFrom`. A step that was `status=running` when the process crashed has unknown real completion status; the deliberate, documented choice is to re-dispatch it rather than guess — extended uniformly to already-`failed` rows too, as one rule instead of a second special case. `paused` executions are untouched, per §8. **Two real, disclosed limitations, not silently glossed over:** ad hoc executions (`ExecuteAdHocStep`) can't be recovered at all, since their one-off step config is never persisted anywhere a restart could re-read it; and this scan has no distributed lock or leader election, so a multi-replica deployment could still have two replicas race to redispatch the same execution on simultaneous restart — real idempotency there depends on `dispatch_token` consumption at the `infra-fleet-service` relay layer, which §8 already flags as unresolved for unrelated reasons.

**`project-service` — OPA authorization on the RPC surface, per `project-service.md` §9's matrix.** New `policy/orca-authz/project.rego` (`package orca.authz.project`, actions `owner_only`/`any_member`, mirroring `task_grant.rego`'s existing style — 23/23 `opa test` passing, 10 new + 13 pre-existing), a new `internal/adapter/opaclient` mirroring auth-service's/annotation-service's real, working adapters exactly, and a shared `requireProjectAccess` usecase-layer helper (fail-closed on every error, same contract as `requireAdminActor`). Wired per the spec's matrix: `UpdateProject`/`DeleteProject`/`AddMember`/`RebindDevServer` → project-owner-or-global-admin; `GetProject`/`ListRepos`/`ListWorktrees` → any membership; `CreateProject`/`ListProjects` unchanged (auth-only). Two real judgment calls, both documented rather than silently made: `AddRepo`/`RemoveRepo`/`ReorderRepos` (not named in the spec's matrix) got the owner-only rule by the closest-fitting reasoning (a repo belongs to one project); `AddMember` needed a bootstrap exception — a strict owner-only gate would deadlock a brand-new project's very first membership row, so the project's own recorded `created_by` self-adding bypasses the check, every other adder still goes through `requireProjectAccess`. **A real, honestly disclosed gap this workstream found, not caused:** no role claim propagates from `api-gateway` into any service's gRPC context today (confirmed against `common/tenant`/`common/grpcmw`/`common/jwtauth` — none carry one) — `callerGlobalRole` always returns `""`, reusing `annotation-service`'s own existing precedent for this identical upstream gap rather than inventing a new claim-propagation mechanism unasked. The global-admin override branch is proven correct at the Rego layer (`opa test`) but is inert in Go until that upstream gap closes — a real, separate, unstarted follow-up. Worktree-mutation RPCs and `ProjectGroup` CRUD remain unauthorized beyond tenant auth, by design, pending future work on which rule best fits them.

**Verified clean, workspace-wide:** `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...` for all 19 `go.work` modules, all clean, zero failures — re-run after all 3 agents completed, not just trusted from each agent's own report. `gitnexus detect_changes({scope:"all"})` run; see its own summary rather than re-quoted here given size (380K+ characters).

**Not done in this pass, honestly flagged:** the two genuine infrastructure blockers (mTLS/SPIFFE, `relay-ssh` deploy) remain exactly as blocked as §15/§13 already documented — not attempted, not fakeable without the missing infrastructure. The multi-replica race in workflow-service's recovery scan and the inert global-admin-override branch in project-service's OPA wiring are both real, separate, unstarted follow-ups, not silently presented as solved. No live Vault, OPA sidecar, multi-replica deployment, or role-claim-issuing `auth-service` call path was available in this environment to verify any of the above end-to-end — every new path is unit-/`opa test`-/real-Postgres-integration-tested, not live-verified, matching this doc's established honesty discipline throughout.

## 18. Fifteenth update, 2026-08-18 — Epic A, fourth pass: `relay-ssh` closes for real, via a new `stdio` mode in `agent/`

**The blocker §13 left open, resolved by questioning its own premise.** §13's `relay-ssh` pass built a real SSH connection layer (`internal/adapter/sshconn`) but stopped there: the mode's spec'd deploy target — SFTP a separately-built `relay.js` binary, launch it via `--detached --connect --sock-path`, talk Stack B's version-handshake protocol over the exec channel — has no buildable artifact anywhere in this repo (`relay.js`'s Unix-socket-daemon model belongs to `backend/`'s legacy TS implementation, not `agent/`). On explicit direction to make relay-ssh real anyway, the fix wasn't to find or fake that artifact — it was to recognize that `agent/` (this repo's actual, buildable Dev Server Agent, already used for `direct-websocket`/`relay-websocket`) could grow a THIRD connection mode speaking the same protocol those two already use, over stdio instead of a WebSocket. `relay.js`'s daemon-reattach complexity (Unix sockets, `nohup`, checksum-then-native-deps-install, multi-platform bundle resolution) was never actually required by relay-ssh's own trust model ("implicit — the SSH connection itself") — only by its original, more elaborate deploy design. A simpler, foreground, one-shot equivalent is a real, complete relay-ssh implementation, not a lesser one.

**Key research finding that shaped the whole design:** `agent/src/relay/agent-session.ts` — the shared TS code `direct-websocket`/`relay-websocket` both already run through unmodified — sends `agent.handshake` as a JSON-RPC *request*, unconditionally, the instant its transport looks "open," regardless of mode. So the Go side driving a newly-launched `stdio` agent must be a **receiver**, not an initiator — the same shape `adapter/agentwsserver` (direct-websocket's inbound server, §13) already implements, minus the token/slot check (SSH is already the trust boundary).

**`agent/` — the new `stdio` mode** (`agent-connection-stdio.ts`, new): a `StdioWebSocketAdapter` duck-types the exact subset of the `ws` package's `WebSocket` interface `agent-session.ts` calls (`readyState`/`send()`/`close()`/`on('message'|'close'|'error')`), backed by `process.stdin`/`process.stdout` — confirmed against `agent-wire.ts`'s real Stack A frame layout (not assumed identical to Stack B's `protocol.ts` without checking) that stdio's arbitrary chunk boundaries need their own incremental reassembly (`StdioFrameAccumulator`, a chunk-list accumulator mirroring `protocol.ts`'s `FrameDecoder` technique without borrowing its decode logic — the raw bytes it hands to `'message'` are parsed by `agent-session.ts`'s own unmodified `decodeFrame`). No reconnect loop, no `AgentTokenManager`, no keepalive ping — SSH owns liveness and the connection is one-shot by design. `agent-config.ts`/`agent-entry.ts` gained the `stdio` mode/an early `--stdio` argv branch, mirroring the existing `ORCA_PTY_DAEMON_SOCKET` branch's shape. Rebuilt `agent/out/agent.js` (717.6 KB, version `2.1.0+10947430cd9c`). 9 new tests (real connected-stream pairs, not mocks); full suite (3741 tests) shows the exact same 2 pre-existing failures as the unmodified baseline (verified by stashing the diff and re-running) — nothing regressed.

**`infra-fleet-service` — the deploy/launch/handshake pipeline and the `Transport` generalization it needed.** `devserveragent.Client`/`session` previously hard-typed `*websocket.Conn` throughout; relay-ssh's SSH-exec-channel stdio isn't a WebSocket, so `session.conn` became `session.transport Transport` (`ReadFrame`/`WriteFrame`/`Close`), with `wsTransport` wrapping the exact same WS behavior relay-websocket/direct-websocket already had (zero change — the whole existing test suite passed unmodified after the refactor, confirming it). New `internal/adapter/sshrelay` (deploy: SFTP-upload + the same portable `node -e` SHA-256 checksum trick the original TS reference uses, new `github.com/pkg/sftp` dependency; launch: `node agent.js --stdio` over a fresh SSH exec session, wired into a new `sshExecTransport` backed by a new `devserveragent.IncrementalDecoder`; provisioner: resolves `SSHTargetID` → `SshTarget`, ties deploy+launch+receiver-handshake together) implements a new `devserveragent.SshProvisioner` port, wired via `Client.WithRelaySSH(provisioner)`. `getOrCreateSession` gained a third, symmetric branch (`getOrProvisionSession` — reuse a live session, else provision a fresh one) alongside the existing dial/inbound-accept branches; `Exec`/`Health` needed **zero** relay-ssh-specific code once this landed — the mode-specific shell.exec-only shortcut an earlier concurrent pass had built (a real, working, but narrower stand-in — supported only `shell.exec`, not the general passthrough) was retired in favor of this generic path, which supports `shell.exec` as just another method alongside everything else, matching the other two modes exactly.

**A real concurrency bug caught by review, not by the race detector failing first:** `sshExecTransport.ReadFrame` originally reused one persistent buffer across the goroutine it spawns per read to make a non-cancelable `io.Reader.Read` respect `ctx` — if a caller's `ctx` ever cancelled while a read was in flight (the 20s handshake timeout is the one real path where this could happen) and `ReadFrame` were called again on the same transport, the abandoned goroutine's eventual, late `Read` could race a new one into the same backing array. Fixed by allocating a fresh buffer per goroutine invocation instead — `go test -race` was clean both before and after (the hazard was never exercised by this pass's own call pattern, which always discards a transport after a handshake failure rather than retrying `ReadFrame` on it), so this was caught by reading the code with the failure mode in mind, not by a red test — flagged here rather than silently fixed without comment, since "the tests passed" would have been true either way.

**Cleanup:** `usecase.SshTargetResolver` (added earlier this pass to support the now-retired shell.exec-only path) had zero remaining consumers once `sshrelay` declared its own structurally-identical interface (this codebase's standard "port defined where consumed" convention) — removed rather than left as dead code, with the two doc comments that referenced it in `postgres/repository.go` corrected to point at `sshrelay.SshTargetResolver` instead.

**Verified clean:** `go build`/`go vet`/`gofmt -l`/`go test -race` clean for `infra-fleet-service` and `common` (both touched modules), including the full pre-existing WS-mode test suite (proving the `Transport` refactor is behavior-preserving) and new coverage for `IncrementalDecoder`, `getOrProvisionSession`'s reuse-vs-reprovision logic (in-memory fake transport), and `sshrelay.Provisioner.Provision` end-to-end against a real fake SSH server extended to also serve SFTP and a scripted exec handshake (checksum-mismatch and unresolvable-target failure paths included, not just the happy path). `go mod tidy` run for the new `github.com/pkg/sftp` dependency. `gitnexus impact` run on `session`/`Client.getOrCreateSession`/`Client` before editing (all LOW risk, contained to this module — `internal/` packages aren't importable cross-service, so the blast radius was always going to be local).

**Not done in this pass, honestly flagged:** the same two `sshconn`-level gaps §13 already carried forward remain unchanged (no host-key verification; hardcoded port 22, `SshTarget` has no port field) — this pass didn't touch `sshconn.Connector` itself beyond adding the `NewSession`/`SFTPClient` accessors `sshrelay` needed. Process lifecycle is deliberately simpler than the originally-spec'd model: one exec channel per session, foreground, no detach/nohup/Unix-socket reattach — a dropped SSH connection ends the session outright; the next call re-provisions from scratch rather than reattaching to a still-running remote process. **No real SSH host, real Vault SSH secrets engine, or real deployed `agent/out/agent.js` running against a live target was available to verify any of this against** — the fake-SSH-server test harness (extended this pass to also serve SFTP and a scripted stdio handshake) is real transport/protocol coverage, not live verification, the same honest gap every connection-mode pass in this epic has carried and disclosed rather than implied away.
