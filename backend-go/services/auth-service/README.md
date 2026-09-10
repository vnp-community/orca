# auth-service

The Phase 4 identity service — see
[`specs/backend-go/services/auth-service.md`](../../../specs/backend-go/services/auth-service.md)
for the full design. Follows the exact package layout and conventions
demonstrated by
[`usage-service`](../usage-service/README.md), the reference
implementation — see that README for the layout rationale.

## What's implemented

- `internal/domain/` — `User`/`Session`/`AuditEntry` entities with
  invariant-enforcing constructors, pure unit tests. `Session` stores only
  the SHA-256 hash of a session token, never the raw value
  (`domain.HashSessionToken`); the raw token exists in this service's
  process memory only long enough to return it once from `Login`.
- `internal/usecase/` — `Login`, `Logout`, `ValidateSession`, `CreateUser`,
  `ListUsers`, `UpdateUserRole`, `RevokeSession` (admin force-revoke, distinct
  from `Logout`), `QueryAuditLog`. `Login`/`ValidateSession` are tested
  against in-memory fakes (`UserRepository`/`SessionRepository`/`AuditRepository`/
  `PasswordHasher`/`Clock`), no real Postgres needed; `CreateUser`'s
  admin-role check is covered too.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  all three repository ports, hand-written SQL (see `architecture/04-tech-stack.md`
  — `sqlc` codegen is the eventual target).
- `internal/adapter/bcrypt/` — `PasswordHasher` via `golang.org/x/crypto/bcrypt`,
  cost floored at 12 regardless of configuration (`auth-service.md` §9).
- `internal/adapter/grpc/` — implements the generated
  `authv1.AuthServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — `auth.users`, `auth.sessions`,
  `auth.audit_log`, RLS policies, `(tenant_id, email)` uniqueness.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM. No NATS/event
  publishing is wired — this scaffold doesn't need it for the RPCs
  implemented (see "Known gaps" for what a fuller build would add).

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/auth-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/auth-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/auth?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/, adapter/bcrypt) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **JWT signing via Vault Transit is now real, for `IssueServiceToken` +
  `GetJWKS` only (Epic D).** `internal/adapter/vault/token_signer.go` wraps
  `common/jwtauth.TransitSigner` over a `*secrets.Client`, backed by a
  single global (not per-tenant — auth-service issues JWTs across every
  tenant from one service identity) Vault Transit RSA key, `"jwt-signing"`
  / `"rsa-2048"`. `cmd/server/main.go` calls `TokenSigner.Ensure(ctx)` at
  startup — the server refuses to start if the key can't be
  created/verified — and registers a `vault` health check
  (`TokenSigner.Ping`, mirroring credential-broker-service's Vault
  reachability pattern) so a pod that loses Vault access is pulled out of
  rotation. `IssueServiceToken` (`internal/usecase/issue_service_token.go`)
  looks the target user up via the existing `UserRepository.GetUserByID`,
  sets `sub` = that user's ID, `tenant_id` = that user's tenant, `aud` =
  the request's `audience`, `iss` = `jwtauth.Issuer`, `exp` = now +
  `ServiceTokenTTL` (config, default 15m, `SERVICE_TOKEN_TTL` env), `jti` =
  `generateRandomToken`, then signs through Vault Transit — the RSA private
  key never enters this process's memory. `GetJWKS`
  (`internal/usecase/get_jwks.go`) publishes the signing key's
  current+previous version (rotation overlap) as an RFC 7517 JWK Set;
  it is deliberately unauthenticated, per the RPC's proto doc comment.
  **Known gap, not fixed here:** `IssueServiceTokenRequest` carries no
  caller-identity field, so there is no check that the *requester* of a
  token is itself authorized to mint one for the given `user_id` — this
  usecase only verifies the target user exists. Closing that needs a proto
  change (a caller-identity field on the request) outside this task's
  scope. **Also still not done:** the fuller
  `IssueToken`/`RefreshToken`/`RevokeToken` RPC surface and
  refresh-token-family design from `auth-service.md` §6-7 — only the
  pre-existing `IssueServiceToken` RPC was made real; see the
  "Refresh-token flow" entry below, still deferred to a later pass now that
  Epic D's signing primitive exists for it to build on.
- **SSO is implemented (CR-LOGIN-001): GitHub, Google, and generic/
  self-hosted OIDC (Keycloak-compatible).** `StartSsoLogin`/
  `CompleteSsoLogin` RPCs, PKCE + HMAC-signed state
  (`internal/adapter/oauthstate`), provider exchangers
  (`internal/adapter/oauth`), and `auth.sso_identities`
  (migration `0003`) are all real — see
  `internal/usecase/login_or_provision_sso_user.go`'s doc comment for the
  account-collision policy (returning identity / auto-link a verified email
  to an existing local account / provision a brand-new user, role always
  `RoleUser` — SSO never auto-admins). api-gateway's
  `GET /auth/sso/{provider}` and `GET /auth/callback` are the only callers;
  both are unauthenticated by necessity, same as `Login`.
  **An unverified email is rejected outright, whether or not it collides
  with an existing account** (`AUTH_SSO_EMAIL_UNVERIFIED_COLLISION` /
  `AUTH_SSO_EMAIL_NOT_VERIFIED`) — no admin-resolution UI exists or is
  planned for either case, by design: silently provisioning (or linking) an
  unverified email is itself an account-takeover vector, not merely a UX
  gap to smooth over. Concretely, before this check existed on *new*-account
  provisioning too (not just the collision path), an attacker could
  pre-register an SSO identity against a victim's real email at a lax/
  unverified IdP — squatting that email in `auth.users` before the victim
  ever signs up — so that the victim's later, genuinely verified SSO login
  for the same email would auto-link into the attacker's pre-existing,
  attacker-controlled account. Requiring verification at the moment an
  email is FIRST claimed (new account or link, either one) closes that hole
  structurally; an admin "just override it" UI would reopen the exact same
  hole with extra steps. A user whose IdP genuinely can't verify their
  email has no self-serve path today — that's an IdP-configuration problem
  for them to fix, not something this service should paper over.
  **Multi-tenant SSO (domain-based, shared IdP credentials):** a brand-new
  SSO user's tenant is resolved via `TenantResolver.ResolveTenantForEmail`,
  which calls tenant-service's `ResolveCompanyByEmailDomain` RPC against
  the verified email's own domain (e.g. `alice@vnpay.vn` -> whichever
  company registered `vnpay.vn` via `AddCompanyEmailDomain`). Falls back to
  "the sole existing company" (this service's original single-tenant-only
  behavior) when the domain isn't registered to anyone — so a deployment
  that has never called `AddCompanyEmailDomain` keeps working exactly as
  before. Fails closed (`AUTH_SSO_UNKNOWN_ORGANIZATION`) when neither
  resolves. **Known gaps, not fixed here:**
  - **One shared IdP credential set for every tenant**, by explicit design
    choice (not a limitation to lift casually) — every company uses the
    SAME `SSO_GOOGLE_CLIENT_ID`/etc. configured on this deployment; there is
    no per-tenant IdP (a different Okta/Keycloak per customer) config store
    or admin UI for one. Adding that is a materially bigger feature
    (per-tenant client secret storage, redirect_uri disambiguation across
    tenants) than domain-based tenant *routing*, which is what's built here.
  - **No admin-console UI to manage email domains** — `AddCompanyEmailDomain`/
    `RemoveCompanyEmailDomain`/`ListCompanyEmailDomains` exist as real,
    tested tenant-service RPCs and REST routes
    (`api-gateway`'s `POST/GET /v1/tenants/companies/{id}/email-domains`,
    `DELETE /v1/tenants/email-domains/{domain}`), but nothing in `frontend/`
    calls them yet — registering a domain today means calling the REST
    route directly (e.g. via `curl` with an admin session cookie).
  - **`domain.User.SsoProvider` is "last used", not history.** It's
    overwritten on every SSO login (`UserRepository.SetSsoProvider`) —
    useful for `GET /auth/me`'s cosmetic `provider` field, not an audit
    trail of every IdP a user has ever linked (that's what
    `auth.sso_identities` itself is for, per-provider).
- **OPA authorization checks are now real (Epic E).**
  `internal/usecase/authorization.go`'s `requireAdminActor` resolves the
  acting user, then calls the embedded OPA policy decision
  (`data.orca.authz.admin.allow`, `backend-go/policy/orca-authz/admin.rego`)
  through `internal/adapter/opaclient` (a thin wrapper over the shared
  `common/policy.Evaluator`, mirroring task-service/annotation-service's own
  `internal/adapter/opaclient`), instead of the earlier inline
  `role == RoleAdmin` check — closing the bug class (`requireAdmin`/
  `requireOwnerOrAdmin` being login-only checks) `auth-service.md` §9
  describes. `CreateUser`, `ListUsers`, `UpdateUserRole`, `RevokeSession`,
  and `QueryAuditLog` all thread an `OPAClient` port through their
  constructors. A policy-evaluation error fails closed (denies), matching
  every other Epic E consumer's contract. `cmd/server/main.go` constructs
  one `policy.NewEvaluator(cfg.OPABundlePath)`, shared across every call —
  `OPABundlePath` config (`OPA_BUNDLE_PATH` env, default
  `../../policy/orca-authz`) matches task-service/annotation-service's own
  convention. **Known gap, not fixed here:** the bundle has no hot-reload —
  a policy edit needs a service restart to take effect (see
  `common/policy.Evaluator`'s doc comment).
- **`CreateUser`'s password handling is a placeholder.** The generated
  `CreateUserRequest` (see `proto/orca/auth/v1/auth.proto`) has no password
  field — there is no invite/reset-link flow in this scaffold to hand a
  chosen password to the new user. `CreateUser` generates a random password,
  hashes it, and never returns or stores it anywhere retrievable — the
  resulting account is unusable until a real credential-issuance flow
  (invite email, forced first-login reset, or SSO) is wired. See
  `internal/usecase/create_user.go`'s doc comment.
- **`Login` resolves users by email only, with no tenant discriminator.**
  The generated `LoginRequest` carries `email`/`password` only (no
  `tenant_id`), so `UserRepository.GetUserByEmail` doesn't take one either.
  In a deployment where the same email exists under multiple tenants (the
  schema's `(tenant_id, email)` uniqueness allows this), `Login` is
  ambiguous. Flagged here rather than inventing an out-of-scope
  tenant-selection step; revisit if/when the proto's `LoginRequest` grows a
  tenant hint.
- **Refresh-token / `IssueToken` / `RefreshToken` / `RevokeToken`, access-policy
  CRUD, and first-run setup — investigated for Epic C
  (`docs/execution-plan.md` §10, 2026-08-17), each resolved differently, not
  left as one undifferentiated "not implemented" bucket:**
  - **First-run setup: not needed as an RPC — a different mechanism already
    closes the gap it exists for.** `auth-service.md` §3's
    `GetFirstRunStatus`/`CompleteFirstRunSetup` solve "how does a fresh
    deployment with zero users get its first admin account" via an
    interactive first-boot wizard. This scaffold already solves the same
    underlying problem via `internal/usecase/bootstrap.go`'s `Bootstrap`
    (env-var-driven: `BOOTSTRAP_ADMIN_EMAIL`/`BOOTSTRAP_ADMIN_PASSWORD`,
    runs once at `cmd/server/main.go` startup, never via RPC — see that
    file's doc comment) — found live and shipped 2026-08-17, before this
    Epic C pass. Different UX (operator-configured env vars vs. an
    interactive setup screen), same goal, already closed. Building the
    RPC pair too would be two mechanisms doing the same job; not done.
  - **Access-policy CRUD: genuinely missing, but correctly deferred, not
    forgotten.** `AccessPolicy` rows are OPA's *input data* (`auth-service.md`
    §3/§5) — CRUD for documents nothing consumes is speculative scaffolding.
    Epic E (OPA policy bundle, `docs/execution-plan.md` §2) is explicit that
    "no service calls OPA yet." Build this once Epic E stands up an actual
    OPA instance for it to feed — tracked there, not duplicated here.
  - **Refresh-token flow: genuinely missing, but correctly deferred, not
    forgotten.** Mobile/CLI `RefreshToken`/`RevokeToken` need real JWT
    issuance behind them to mean anything. Epic D has now landed real
    Vault-Transit-backed signing + JWKS publication for `IssueServiceToken`/
    `GetJWKS` (see "JWT signing via Vault Transit" above) — but the
    dedicated `IssueToken`/`RefreshToken`/`RevokeToken` RPCs and
    refresh-token-family rotation state are still not in the generated
    proto surface or implemented here. That's a distinct piece of work
    (new proto messages/RPCs, a `refresh_tokens` table, rotation-family
    bookkeeping) layered on top of the signing primitive Epic D built, not
    included in this pass.
  - **No session/refresh-token reaper job** — the `sessions` table itself
    is real (see `ValidateSession`), but nothing expires old rows yet;
    `refresh_tokens` doesn't exist until the point above lands. A cheap,
    separable follow-up once there's a real table to reap.
  - `CreateUser`'s "first admin user" case has no special-cased single-use
    guard — not needed either, since `Bootstrap` (not `CreateUser`) is what
    actually creates that first user; `CreateUser` itself always requires
    an existing admin caller (`requireAdminActor`), which is already the
    right invariant for every call after the first.
- **`common/secrets` (Vault) is wired into this service's `main.go` for
  Transit-based JWT signing only (Epic D, see above) — not for database
  credentials.** `DATABASE_DSN` is still read directly from the environment
  for local dev, same caveat as usage-service's README; a Vault-Agent-
  rendered dynamic DB credential (the bootstrap exception every other
  service also defers, per `common/secrets`' doc comment) isn't wired here
  either.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in
  (see that package's doc comment).
- **No audit-log-integrity DB grant is applied by the migration.**
  `migrations/0001_init.up.sql`'s comment on `auth.audit_log` notes the
  INSERT/SELECT-only, no-UPDATE/DELETE database-role grant `auth-service.md`
  §9 calls for is an environment-provisioning step, not expressed in this
  migration — apply it per deployment.
