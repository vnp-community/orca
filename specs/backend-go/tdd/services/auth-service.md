# `auth-service`

Category: **Identity** · ADR-021 schema: `auth` · Migration phase: **4**

## 1. Overview & responsibility

`auth-service` is the system of record for "who is this, and are they who
they claim to be." It owns identity (users), the mechanisms that prove and
carry identity across a request (sessions, JWTs, JWKS), the append-only
record of security-relevant actions (audit log), and the admin console that
manages all of the above. It does **not** decide "is this action allowed" —
per [`07-security-architecture.md`](../architecture/07-security-architecture.md)
that decision belongs to OPA, evaluated against data this service supplies.
`auth-service` answers "who" and publishes the facts OPA needs to answer
"may they"; it structurally cannot itself become the kind of ad hoc
`if role === 'admin'` check that caused the TS system's
`requireAdmin`/`requireOwnerOrAdmin` login-only-check bug (see §9).

It owns:

- **Users** — identity, credential hash, global role, account status.
- **Sessions** — browser cookie sessions and mobile/CLI JWT issuance,
  refresh, and revocation.
- **JWKS** — the public-key set every other service validates JWTs against.
- **Audit log** — system-wide append-only record of security-relevant
  events (own + ingested from other services' outbox streams, see §9).
- **Access policies** — the *data* OPA's Rego rules evaluate against (role
  definitions, resource-scope grants, rate-limit tiers), CRUD'd through the
  admin console. It does **not** own the Rego policy logic itself (§2).
- **Admin console backend** — user CRUD, session force-revoke, access-policy
  CRUD, audit-log query, first-run setup. Folded in per
  [`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md)
  because admin operations are RBAC operations on data this service already
  owns — a separate `admin-service` would just be a second front door onto
  the same tables with an extra network hop.

## 2. Bounded context — what this service does NOT own

- **Companies, departments, teams, user profiles/settings** —
  `tenant-service`. `auth-service`'s `users` table carries only identity and
  the `tenant_id` a user was provisioned under (a logical FK, validated by
  calling `tenant-service`); profile fields, department assignment, and team
  membership live entirely in `tenant-service`.
- **Task grants, project membership, workflow-execution permission** —
  `task-service`/`project-service`/`workflow-service` own the domain data
  those decisions are made about. They call OPA themselves (embedded,
  in-process) with domain context `auth-service` doesn't have, rather than
  asking `auth-service` "can this user do X" as a remote procedure call.
- **The Rego policy bundle (`orca-authz`) itself** — the compiled decision
  logic is a version-controlled artifact reviewed like code, not a row in
  this service's database. `auth-service` is the primary *data* source the
  bundle's `input`/`data` documents draw from (user role, access-policy
  rows), but it does not author or ship policy logic. Where the bundle
  lives operationally (a dedicated policy repo + OCI bundle registry vs.
  folded into this service's deploy) is a platform decision outside this
  doc's scope — either way, the boundary "data here, logic there" holds.
- **Secret material** — password hashes are the one exception this service
  stores directly (see §4); everything else (Vault tokens, OAuth client
  secrets for a future SSO integration, agent tokens) is `credential-broker-service`
  metadata backed by Vault, never this database.
- **Dev-server/agent tokens** — `infra-fleet-service` (different trust
  domain: a host's identity, not a user's).

## 3. API surface (gRPC)

Internal gRPC only, per [`04-tech-stack.md`](../architecture/04-tech-stack.md);
`api-gateway` exposes the cookie-facing subset over HTTP via `grpc-gateway`.
Grouped by the TS capability they replace:

**Session lifecycle** (replaces `AuthManager`, `auth-router.ts`)
- `Login(email, password)` → verifies bcrypt hash, creates a session row,
  returns the opaque session token `api-gateway` sets as the `orca_session`
  cookie, plus the user's identity/role for the initial page load.
- `Logout(sessionToken)` → revokes the caller's own session.
- `ValidateSession(sessionToken)` → used by `api-gateway` on cookie-authenticated
  requests; returns user ID, role, tenant ID, and expiry, or "invalid/expired."
  Cacheable at the gateway for a few seconds (§8) — this is the one RPC on
  literally every browser request's path.
- `IssueToken(sessionToken | refreshToken)` → mints a short-lived RS256 JWT
  for mobile/CLI/service-to-service use, plus a refresh token.
- `RefreshToken(refreshToken)` → rotates to a new access JWT; rotates the
  refresh token itself (refresh-token reuse detection, §9).
- `RevokeToken(refreshToken | jti)` → invalidates a refresh token or a
  specific JWT (denylist entry, checked by services that need immediate
  revocation semantics stronger than "wait for TTL expiry").
- `GetJWKS()` → public, unauthenticated. Returns the current + previous
  signing key's public half (RS256) so every service can validate JWTs
  without calling back to `auth-service` per request.

**SSO** — see §9. `InitiateSSO(provider)` / `HandleSSOCallback(...)` are
named here as the eventual replacement for the TS stub
(`GET /auth/sso/:provider`, always `501`) but are **not** part of this
service's initial build — flagged as a product decision, not designed
further in this doc.

**Admin — users** (replaces `admin-user-handlers.ts`)
- `CreateUser`, `GetUser`, `ListUsers` (paginated, filterable by status/role),
  `UpdateUser` (email, role, profile fields that live here vs. delegate to
  `tenant-service` for the rest), `DeactivateUser`, `ReactivateUser` — the Go
  service adds `ReactivateUser` deliberately; the TS admin console had a
  `status` field that supported it but no handler ever implemented the
  transition back from deactivated (business-capabilities.md, "Admin
  console" gap note).

**Admin — sessions** (replaces `admin-session-handlers.ts`)
- `ListSessionsForUser`, `ForceRevokeSession` (single session),
  `ForceRevokeAllSessionsForUser` (kill-all, e.g. after a password reset or
  suspected compromise).

**Admin — access policies** (replaces `admin-policy-handlers.ts`)
- `CreateAccessPolicy`, `GetAccessPolicy`, `ListAccessPolicies`,
  `UpdateAccessPolicy`, `DeleteAccessPolicy` — CRUD on the data rows OPA's
  bundle reads as `data.orca.policies.*`; editing a row here changes what
  OPA decides on the next evaluation, it does not change *how* OPA decides.

**Admin — audit** (replaces `admin-audit-handlers.ts`)
- `QueryAuditLog(filters: actor, action, resource, time range, page)` —
  read-only, paginated; the log itself is never mutated through this API
  (§9).

**First-run setup** (replaces `first-run-setup.ts`)
- `GetFirstRunStatus()` → whether an initial admin account has been
  created yet.
- `CompleteFirstRunSetup(email, password)` → creates the first admin user;
  a single-use operation, structurally disabled (not just UI-hidden) once
  any user row exists — see §9.

## 4. Domain model

- **`User`** — `id`, `tenantID`, `email` (unique per tenant), `passwordHash`,
  `role` (`admin` | `member` — coarse global role; fine-grained authorization
  is OPA's job, not an enum grown over time here), `status`
  (`active` | `deactivated`), `createdAt`, `lastLoginAt`. Invariant: a
  `User` cannot be constructed with a plaintext password reaching the
  domain layer — the constructor takes a `PasswordHash` value object,
  produced by the `PasswordHasher` port (§6) in the usecase layer, never in
  `domain/`.
- **`Session`** — `id` (opaque token, high-entropy, not a JWT), `userID`,
  `createdAt`, `expiresAt`, `lastSeenAt`, `ip`, `userAgent`. Invariant:
  `expiresAt` is always set at creation (no "session that never expires" —
  the TS system's idle-timeout logic is preserved as an absolute TTL plus
  sliding `lastSeenAt`-based extension, not an unbounded session).
- **`RefreshToken`** — `jti`, `userID`, `familyID` (rotation family, for
  reuse-detection, §9), `expiresAt`, `revokedAt`. Not the JWT itself (JWTs
  are stateless and never stored) — this is the record that lets
  `RevokeToken`/reuse-detection work at all.
- **`AccessPolicy`** — `id`, `name`, `kind` (role-definition | rate-tier |
  resource-scope), `document` (the JSON/YAML fed to OPA as a `data`
  document), `version`, `updatedBy`, `updatedAt`. Invariant: every update
  is a new version, not an in-place mutation — OPA bundle sync and audit
  both need a stable history of "what did the policy input look like at
  time T."
- **`AuditEntry`** — `id`, `actorUserID` (nullable — some entries are
  system-initiated), `action`, `resourceType`, `resourceID`, `payload`
  (structured, redacted of secret material), `occurredAt`. Invariant:
  immutable once written — no `usecase/` method exists to update or delete
  an `AuditEntry`; the only outbound path from this table is `QueryAuditLog`
  and the retention job (§9).

## 5. Data model (Postgres, database `auth`)

Own physical database/cluster per
[`05-data-architecture.md`](../architecture/05-data-architecture.md) — this
is one of the three services (`auth`, `tenant`, `credential`) called out
there as recommended for a dedicated instance given blast radius.

| Table | Key columns | Indexes | Replaces (TS) |
|-------|-------------|---------|-----------------|
| `users` | `id UUID PK`, `tenant_id UUID`, `email TEXT`, `password_hash TEXT`, `role TEXT`, `status TEXT`, `created_at`, `last_login_at` | unique `(tenant_id, email)`; `idx_users_status` | `orca_users` |
| `sessions` | `id TEXT PK` (opaque token hash — see §9), `user_id UUID FK→users`, `created_at`, `expires_at`, `last_seen_at`, `ip inet`, `user_agent TEXT` | `idx_sessions_user_id`; `idx_sessions_expires_at` (for the reaper job) | in-memory/TS session state, no dedicated table previously |
| `refresh_tokens` | `jti UUID PK`, `user_id UUID FK`, `family_id UUID`, `expires_at`, `revoked_at NULL` | `idx_refresh_tokens_user_id`; `idx_refresh_tokens_family_id` | new — TS had no mobile/CLI JWT refresh model |
| `access_policies` | `id UUID PK`, `name TEXT`, `kind TEXT`, `document JSONB`, `version INT`, `updated_by UUID`, `updated_at` | unique `(name, version)` | new table — TS had no unified policy table (`resolveUserPermissions()`/`TaskGrantService` were code, not data) |
| `audit_log` | `id BIGSERIAL PK`, `actor_user_id UUID NULL`, `action TEXT`, `resource_type TEXT`, `resource_id TEXT`, `payload JSONB`, `occurred_at` | `idx_audit_log_occurred_at` (BRIN, append-only + time-range queries); `idx_audit_log_actor` | `orca_audit_log` |
| `outbox` | standard transactional-outbox shape (`id`, `event_type`, `payload`, `published_at NULL`) | `idx_outbox_unpublished` | new — required by the outbox pattern (§7) |

`sessions.id` stores a hash of the token, not the token itself (same
principle already applied correctly to Dev Server agent tokens in the TS
system per `07-security-architecture.md` — a stolen DB snapshot shouldn't
yield usable session tokens). `users` has no `tenant_id NOT NULL` violation
risk at signup time because `tenant_id` is assigned during provisioning
(invite flow, owned by `tenant-service`), not chosen by the registering
user.

## 6. Package layout notes

Standard layout from
[`03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md)
applies; auth-specific additions:

- `internal/usecase/ports.go` defines `PasswordHasher` (bcrypt lives behind
  this port in `adapter/crypto/`, never imported by `domain/`),
  `TokenSigner` (wraps Vault Transit — see §7, no private key material ever
  materializes in this service's process memory), `PolicyDataPublisher`
  (pushes `access_policies` changes to wherever the OPA bundle registry
  lives), and `SessionRepository`/`UserRepository`/`AuditRepository`.
- `internal/adapter/opa/` — embedded OPA SDK client, used for this
  service's *own* admin-endpoint authorization checks (§9). This is the
  same in-process pattern every other service uses for fine-grained checks;
  `auth-service` is not special-cased to skip it just because it also owns
  the policy data.
- `internal/adapter/vault/` — two distinct Vault interactions, kept in
  separate files even though both go through the same client: (1) dynamic
  Postgres credentials for this service's own DB pool (same as every
  service), (2) Transit engine calls to sign JWTs — the RSA private key
  never leaves Vault, `auth-service` sends a signing request and gets a
  signature back.
- `internal/adapter/jwks/` — serves the public JWKS document, handles key
  rotation (old key stays published until every issued JWT under it has
  expired, per §9).

## 7. Dependencies

**Called by:** every other service, directly for admin-console gRPC calls
against this service, and indirectly for identity resolution — `api-gateway`
calls `ValidateSession`/validates JWTs against `GetJWKS` on essentially every
request before routing anywhere else (per the dependency graph in
[`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md),
`auth-service` is called by "nearly everything"). Services other than
`api-gateway` validate JWTs locally against the cached JWKS (stateless — no
synchronous call to `auth-service` per request), only calling this service
directly for admin operations or session-management RPCs.

**Calls:**
- **Vault** — dynamic Postgres credentials for its own database pool;
  Transit engine for JWT signing (§6). No other outbound service
  dependency for the core login/session/JWT path — this is intentional,
  since a login path that itself depends on `tenant-service` or others
  being up would make `auth-service` less available than the services that
  depend on it, which is backwards.
- **NATS JetStream** (via its outbox) — publishes `user.created`,
  `user.deactivated`, `session.revoked`, and audit events other services
  may want to react to (e.g. `notification-service` for security alerts).
  Does not consume events from other services for the core auth path —
  `tenant-service` lookups (e.g. validating a `tenant_id` at user
  provisioning time) go through a synchronous gRPC call from
  `tenant-service` into `auth-service` (user creation is initiated by
  `tenant-service`'s invite flow calling `CreateUser` here), not the
  reverse.

## 8. Non-functional requirements

This service is on the critical path of essentially every authenticated
request in the system — its SLOs are tighter than most:

- **`ValidateSession` / JWT-local-validate path**: p99 < 20ms. `api-gateway`
  caches `ValidateSession` results for a few seconds per session token to
  keep this off the hot path for every single request; JWT validation
  needs no round trip at all once the JWKS is cached locally by the
  validating service.
- **`Login`**: p99 < 300ms including the bcrypt verify (cost 12 is
  deliberately expensive — this is a security/latency tradeoff, not a bug;
  see §9).
- **`GetJWKS`**: effectively a static-content read, cacheable aggressively
  (minutes) by every consumer; a `auth-service` outage should not
  immediately break JWT validation elsewhere as long as the cached JWKS is
  still within its rotation-overlap window (§9).
- **Availability**: this service's availability target is the *highest* of
  the 17 — an `auth-service` outage takes down login and, once caches
  expire, effectively every other service's ability to authorize new
  requests. Horizontally scaled, stateless except for the DB (standard
  `pgxpool` + read-replica routing for `ValidateSession`/`GetUser` reads).
- **Session/refresh-token reaper**: a background job expiring rows past
  `expires_at`, run frequently enough that `sessions`/`refresh_tokens`
  table growth stays bounded — not a correctness requirement (expired rows
  already fail validation) but an operational one.

## 9. Security notes

This service is largely *about* security, so most of the interesting
design decisions live here rather than being a separate concern layered on
top.

- **Closing the `requireAdmin` bug class structurally.** The TS system's
  `requireAdmin`/`requireOwnerOrAdmin` were, for a period, login-only
  checks — any authenticated user passed them, not just admins
  (patched, per `business-capabilities.md`'s admin-console note, but only
  because someone happened to catch it in review). This redesign makes
  that bug class structurally unreachable rather than relying on the next
  reviewer catching the next instance of it: **every** admin-console
  `usecase/` method (`CreateUser`, `ForceRevokeSession`,
  `UpdateAccessPolicy`, …) calls the embedded OPA SDK with the caller's
  resolved role/claims as input before doing anything else, the same
  mechanism every other service uses for its own fine-grained checks. There
  is no handler-local `if user.role == "admin"` anywhere in this service's
  code for that reason — a missing check shows up as an OPA policy gap
  (testable with `opa test`, reviewable in one place, the `orca-authz`
  bundle), not a silent gap in one handler among many.
- **Cookie session hardening.** `HttpOnly`, `SameSite=Strict`, `Secure`
  **always on**, not conditional on an environment variable — the TS
  system's `secure` flag was only true when `NODE_ENV==='production'`,
  meaning any non-production deployment (which includes real self-hosted
  installs that never set that variable) shipped session cookies over
  plaintext HTTP. Treated here as a bug, not a feature to preserve: `Secure`
  is unconditional, and local development uses a real TLS terminator (or
  is explicitly documented as accepting degraded security), not a silent
  env-var-gated downgrade.
- **Session fixation/hijacking.** `Login` always issues a *new* session ID
  (never reuses a pre-auth anonymous session ID, if one existed); session
  tokens are high-entropy random values, stored hashed (§5), so a DB read
  doesn't yield a usable token; `ForceRevokeAllSessionsForUser` is called
  automatically on password change, not just exposed as an admin action.
- **Refresh-token reuse detection.** Refresh tokens rotate on every use and
  are grouped by `family_id`; if a already-used (rotated-away) refresh
  token is presented again, the entire family is revoked — the standard
  signal that a refresh token was stolen and both the attacker and the
  legitimate client are now racing to use it.
- **JWT/JWKS rotation.** Signing key rotation publishes the new public key
  to `GetJWKS` *before* it's used to sign anything (so no consumer
  validates against a key it hasn't fetched yet), and keeps the previous
  key published until every JWT signed under it has expired — a
  consumer's locally cached JWKS is never invalidated mid-flight by a
  rotation.
- **Bcrypt, 12 rounds minimum.** Carried forward from the TS system as-is
  (it was already correct there). Cost factor is a config value, not a
  compile-time constant, so it can be raised as hardware gets faster
  without a code change; a future migration to Argon2id is an option to
  revisit but not required at launch.
- **Brute-force mitigation.** `Login` is rate-limited per email and per
  source IP (policy data lives in `access_policies`, evaluated the same
  way as everything else — this is itself an OPA-governed decision, not a
  hardcoded counter), closing a gap the TS system's design docs don't
  describe having addressed at all.
- **First-run setup is structurally single-use.** `CompleteFirstRunSetup`
  checks `SELECT EXISTS(SELECT 1 FROM users)` inside the same transaction
  that inserts the first admin — not a feature flag or a UI-hidden route,
  so it cannot be re-invoked once any user exists even if an old
  first-run URL is replayed.
- **Audit log integrity.** Append-only at the database-permission level
  (the service's own Postgres role has `INSERT`/`SELECT` but not
  `UPDATE`/`DELETE` on `audit_log` — enforced independently of application
  code bugs), shipped to the observability pipeline's log store with a
  longer retention window than operational logs per
  [`07-security-architecture.md`](../architecture/07-security-architecture.md)
  (a compliance requirement, not a debugging convenience). Retention period
  itself is a policy/compliance decision to set per deployment, not fixed
  in this doc.
- **SSO is a real gap, not a stub to carry forward.** The TS system's
  `GET /auth/sso/:provider` always returns `501` — it was never
  implemented. Porting that as another `501` in Go would just move the gap
  forward without deciding anything. This redesign should implement OIDC
  properly (authorization-code flow, provider metadata discovery, a
  `sso_identities` table mapping external subject → internal `User`) if
  SSO is actually needed — **that's a product decision to make explicitly
  before this service's initial build**, not a mechanical translation of
  the existing stub.

## 10. Migration notes

- **Phase 4** — deliberately near-last. Per
  [`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md),
  `auth-service` (with `tenant-service`) is the highest-blast-radius
  service in the catalog: every other service resolves identity through it,
  so it's rolled out only once the services it doesn't depend on are
  already stable in Go, minimizing the number of moving parts during the
  highest-risk cutover.
- **What TS gap this closes**: the `requireAdmin`/`requireOwnerOrAdmin`
  login-only-check history (§9) and the `Secure`-cookie-only-in-production
  bug (§9) — both become structurally prevented rather than "patched, must
  not regress."
- **Data backfill** — unlike stateless services, this one has real
  production data to migrate, not just a schema to stand up:
  - `users` / `orca_users`: direct backfill. `password_hash` values carry
    over unchanged — bcrypt hashes are self-describing (cost factor
    embedded in the hash string) and portable across implementations, no
    rehash required.
  - `sessions`: **not backfilled**. Existing TS sessions are short-lived by
    design; the cutover forces re-authentication rather than attempting to
    translate an in-memory/TS session representation into the new
    hashed-token model. This is safer than a lossy translation and the
    UX cost is one extra login.
  - `access_policies`: **new data, not a backfill** — the TS system never
    had a unified policy table (`resolveUserPermissions()` and
    `TaskGrantService.resolvePermission()` were code, not rows). Initial
    `access_policies` rows and the matching `orca-authz` Rego bundle must
    be authored and tested (`opa test`) to reproduce the TS system's actual
    current behavior before cutover, then diffed against real traffic
    (shadow-mode OPA evaluation logging "would have denied" without
    enforcing) to catch behavior drift before it's load-bearing.
  - `audit_log` / `orca_audit_log`: direct historical backfill (append-only
    on both sides, no conflict resolution needed) — preserves audit
    continuity across the cutover for compliance purposes.
  - Expand/contract per
    [`05-data-architecture.md`](../architecture/05-data-architecture.md):
    dual-write or a one-time cutover window (given `auth-service` sits on
    every request's path, a brief maintenance window for the final
    `users`/`access_policies` cutover is likely preferable to a long
    dual-write period against two different identity systems at once —
    decide the exact cutover mechanics in the rollout runbook, not this
    doc).
