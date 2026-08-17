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

- **JWT signing via Vault Transit is not wired.** `IssueServiceToken`
  exists and compiles (`internal/adapter/grpc/server.go`) but returns
  `codes.Unimplemented` with a clear message rather than a token that looks
  real but validates against nothing. Per `auth-service.md` §6-7, the real
  implementation needs a Vault Transit client (`internal/adapter/vault/`,
  not present in this scaffold) that requests a signature without the RSA
  private key ever leaving Vault, plus the JWKS-publication sequencing in
  §9 (publish the new public key *before* it's used to sign anything). Wire
  this before any consumer relies on mobile/CLI/service-to-service JWTs.
  `GetJWKS` itself isn't in this scaffold's proto surface either — add it
  alongside real signing.
- **SSO is not implemented — a real product gap, not a mechanical stub.**
  Per `auth-service.md` §9, the TS system's `GET /auth/sso/:provider`
  always returned `501`; this redesign is supposed to implement OIDC
  properly (authorization-code flow, provider metadata discovery, an
  `sso_identities` table) if SSO is actually needed, as an explicit product
  decision *before* auth-service's initial build — not something this
  scaffold should invent. `InitiateSSO`/`HandleSSOCallback` aren't in the
  generated proto and aren't implemented here.
- **OPA authorization checks are not wired — every admin-console usecase
  does a simple role check instead.** `internal/usecase/authorization.go`'s
  `requireAdminActor` looks up the acting user and checks
  `role == RoleAdmin` directly. Per `auth-service.md` §9, this is exactly
  the bug class (`requireAdmin`/`requireOwnerOrAdmin` being login-only
  checks) the OPA-based design is meant to close structurally: the correct
  implementation calls the embedded OPA SDK (`internal/adapter/opa/`, not
  present in this scaffold) with the caller's resolved role/claims as
  input, replacing this placeholder in `CreateUser`, `ListUsers`,
  `UpdateUserRole`, `RevokeSession`, and `QueryAuditLog`. Do not treat the
  placeholder as sufficient for production.
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
- **No refresh-token / `IssueToken` / `RefreshToken` / `RevokeToken` model.**
  `auth-service.md` §3/§5 describes a `refresh_tokens` table and
  reuse-detection flow for mobile/CLI JWTs; none of that RPC surface is in
  the generated proto or implemented here — only session-cookie login
  (`Login`/`Logout`/`ValidateSession`) and the admin-console RPCs are.
- **No access-policy CRUD, first-run setup, or session/refresh-token
  reaper job.** `auth-service.md` §3/§5/§8 describes `AccessPolicy` CRUD,
  `GetFirstRunStatus`/`CompleteFirstRunSetup`, and a background reaper for
  expired sessions — none of these are in the generated proto's RPC set,
  so none are implemented in this scaffold. `CreateUser`'s "first admin
  user" case has no special-cased single-use guard here.
- **`common/secrets` (Vault) is not wired into this service's `main.go`** —
  `DATABASE_DSN` is read directly from the environment for local dev, same
  caveat as usage-service's README.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in
  (see that package's doc comment).
- **No audit-log-integrity DB grant is applied by the migration.**
  `migrations/0001_init.up.sql`'s comment on `auth.audit_log` notes the
  INSERT/SELECT-only, no-UPDATE/DELETE database-role grant `auth-service.md`
  §9 calls for is an environment-provisioning step, not expressed in this
  migration — apply it per deployment.
