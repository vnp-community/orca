# `credential-broker-service`

See
[`specs/backend-go/services/credential-broker-service.md`](../../../specs/backend-go/services/credential-broker-service.md)
for the full design and
[`specs/backend-go/services/usage-service.md`](../../../specs/backend-go/services/usage-service.md) /
[`usage-service/README.md`](../usage-service/README.md) for the package
layout and conventions this service follows — this service departs from
that reference only where the design doc explicitly calls for it (§6:
`internal/adapter/vault/` is real here, not absent).

This is **the** answer to this system's "use Vault for sensitive data"
requirement. `credential-broker-service` is the mediation layer for all
tenant/user secret material in the system: every OAuth token, AI provider
API key, SSH credential, and service-to-service shared secret that any other
service needs to read, write, rotate, or revoke passes through this
service's gRPC API. **The secret bytes themselves live in HashiCorp Vault —
this service's own PostgreSQL database holds pointers, lifecycle state, and
an append-only audit trail, and nothing else.**

## What's implemented

- `internal/domain/` — `CredentialMetadata` (id, tenant_id, owner_id,
  category, status, vault_path — **no field capable of holding a secret
  value, ever**) and `AccessAuditEntry`, both with invariant-enforcing
  constructors, pure unit tests. `Category.Engine()` derives which Vault
  engine (`transit`/`kv2`) a category's material belongs to, never
  independently settable.
- `internal/usecase/` — `WriteCredential`, `ResolveCredential`,
  `RotateCredential`, `RevokeCredential`, each tested against in-memory fake
  `CredentialMetadataRepository`/`AuditRepository`/`SecretStore`, no real
  Postgres/Vault needed. Notably tested:
  - `ResolveCredential` writes its access-audit row **before** returning the
    resolved value, on every path that reaches Vault (success, revoked,
    Vault error) — see `resolve_credential_test.go`'s call-order assertion.
  - `RevokeCredential` genuinely calls `SecretStore.RevokeSecret` (not just
    the Postgres status flip) — see `revoke_credential_test.go`.
  - A failed audit write fails the whole operation, per the design doc's
    "audit log write must never be best-effort" rule — see
    `write_credential_test.go`/`resolve_credential_test.go`.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  both `CredentialMetadataRepository` and `AuditRepository` against the
  `credential` schema. **Not one column in this schema can hold a secret
  value** — see `migrations/0001_init.up.sql`'s prominent comment.
- `internal/adapter/vault/` — **this service's real, working Vault
  integration**, wrapping `common/secrets.Client` as a thin pass-through:
  `TransitEncrypt`/`TransitDecrypt`/`KVWrite`/`KVRead` all delegate directly
  to the matching `common/secrets` method, no reinterpretation. This is the
  one service in the whole backend-go scaffold where that package is wired
  for real, non-stub, tenant-secret use — every other service's Vault
  identity is scoped only to its own dynamic DB credential lease.
- `internal/adapter/grpc/` — implements the generated
  `credentialbrokerv1.CredentialBrokerServiceServer`
  (`WriteCredential`/`ResolveCredential`/`RotateCredential`/`RevokeCredential`
  — the actual generated proto's RPC surface, a subset of the design doc's
  fuller sketch; see "Deviations" below).
- `migrations/0001_init.{up,down}.sql` — real DDL: `credential.credential_metadata`
  (no secret columns, by construction and by a schema-level SQL comment
  saying so), `credential.access_audit_log` (append-only, FK to
  `credential_metadata`), RLS policies.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, a **real** `secrets.NewClient()` wired into
  `internal/adapter/vault`, gRPC server with the shared interceptor chain,
  health/readiness HTTP server (including a Vault reachability check — see
  "Vault health check" below), graceful shutdown on SIGTERM.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres vault   # see ../../docker-compose.yml
migrate -path services/credential-broker-service/migrations \
  -database "$DATABASE_DSN" up        # golang-migrate; see architecture/05

cd services/credential-broker-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/credential_broker?sslmode=disable \
VAULT_ADDR=http://localhost:8200 \
VAULT_TOKEN=root \
  go run ./cmd/server
```

A real local Vault dev server (`vault server -dev`) needs, at minimum, a KV
v2 mount at `credential-secrets` (`vault secrets enable -path=credential-secrets kv-v2`)
and one Transit key per category this scaffold will actually exercise
(`vault secrets enable transit` once, then
`vault write -f transit/keys/credential-broker-<category>` per category,
e.g. `credential-broker-scm_oauth`) — see "Known gaps" below for why these
names are currently hardcoded rather than driven by real policy.

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

There is no equivalent integration-tagged test against a real Vault dev
server today — `internal/adapter/vault` is exercised only indirectly, via
the `usecase` layer's fakes. See "Known gaps".

## Known gaps / follow-ups (tracked, not silently skipped)

This service has the **fewest stubs** of any service in this scaffold — its
Vault integration is real, not mocked — but it is still a scaffold, not a
production-ready deployment. What's left, explicitly:

- **Vault mount/key names are hardcoded placeholders.**
  `internal/usecase/vault_paths.go` hardcodes `kvMount = "credential-secrets"`
  and a `credential-broker-<category>` Transit key naming scheme. A real
  deployment needs an actual Vault policy design (per
  credential-broker-service.md §9's least-privilege table) — a KV v2 mount
  and per-category Transit keys provisioned by platform/security, with this
  service's Vault Kubernetes-auth role's policy scoped to exactly those
  names — before this can run against a production Vault cluster. Nothing
  in this scaffold depends on that policy already existing; it depends only
  on a Vault server that will let the token in `VAULT_TOKEN` create/read
  under those fixed names (true of any local dev-mode Vault, root-token
  scoped).
- **The "client-side encrypted envelope" handling in `WriteCredential` is
  simplified.** `WriteCredentialRequest.encrypted_envelope` is, per the
  design doc, already transport-encrypted by the browser before it reaches
  this service (ADR-008), and this service is supposed to decrypt that
  transport envelope in-memory before re-encrypting under Vault Transit.
  Real client-side crypto integration — negotiating a per-request transport
  key with the browser, implementing that specific envelope's decrypt
  format — is out of scope for this scaffold. **What this scaffold actually
  does instead**: `internal/usecase/write_credential.go` treats the
  envelope as **opaque bytes** end to end. It is never decrypted, never
  called "plaintext" anywhere in this codebase (variable names, comments,
  log fields), and is passed directly into `SecretStore.TransitEncrypt` as
  the value to encrypt. This is honest about what happens (Vault's own
  encryption protects the bytes at rest; no server-side transport-decrypt
  step exists) rather than faking a decrypt that isn't really there. **What
  a real integration needs to add**: (1) a documented, versioned envelope
  wire format (`payload_encoding` in the design doc's fuller
  `WriteCredentialRequest` sketch — `"transit-envelope-v1"` vs
  `"plaintext"` — collapsed to a single opaque-bytes path in the actual
  generated proto used here), (2) a genuine transport-decrypt step in this
  usecase keyed by whatever the browser and this service negotiate,
  producing a value this codebase would then, and only then, be honest
  calling "plaintext" for the remainder of that one request's stack frame,
  and (3) the same in-memory-only, never-logged, never-returned discipline
  this scaffold already follows for the ciphertext it does handle.
- **Vault Kubernetes auth (production) vs static `VAULT_TOKEN` (this
  scaffold's local-dev path).** `common/secrets.NewClient()`'s own doc
  comment is explicit about this: "Production services authenticate via the
  Kubernetes auth method through a Vault Agent sidecar instead of a static
  token." This service's `cmd/server/main.go` calls `secrets.NewClient()`
  exactly as documented — nothing service-specific needs to change here;
  the Kubernetes-auth wiring is infrastructure this scaffold doesn't stand
  up, not application code this service is missing.
- **`RevokeCredential`'s Vault-side revocation is an overwrite, not a native
  delete.** `common/secrets.Client` has no KV-delete-version or
  Transit-key-delete method today, and this service must not modify
  `common/` to add one. `internal/adapter/vault.SecretStore.RevokeSecret`
  therefore calls the real `KVWrite` with an empty payload — a genuine Vault
  call, not a stub — which leaves no readable `ciphertext` field for a
  subsequent `KVRead` to find. This is **not** equivalent to Vault's native
  "destroy version" API (the prior version's bytes may still be recoverable
  via KV v2's version history to a caller with broader-than-"current
  version" read access) and does not touch the Transit key at all (Transit
  keys are shared per category, so deleting one would revoke every
  credential in that category). The primary revocation enforcement is the
  fail-closed status check in `ResolveCredential` — Vault-side overwrite is
  defense in depth, not the only mechanism. See
  `internal/adapter/vault/secret_store.go`'s doc comment.
- **Vault health check is a heuristic, not a native ping.**
  `common/secrets.Client` has no `Ping`/`Sys().Health()`-style method.
  `internal/adapter/vault.SecretStore.Ping` reads a well-known,
  expected-to-be-absent KV v2 path and treats a "not found" error as
  "Vault answered, so it's reachable" — any other error is treated as
  unreachable/misconfigured. This is documented, not hidden; a production
  hardening pass should switch to a real `Sys().Health()` call once
  `common/secrets` exposes one.
- **`RequestingService` identity is a plain gRPC metadata header, not real
  mTLS/JWT identity.** The design doc requires `RequestingIdentity` to be
  "resolved JWT/mTLS subject, not client-asserted" (§4). `common/grpcmw`'s
  `TenantExtractionInterceptor` extracts tenant/user identity this way
  already, but not a calling-service identity, and this service must not
  modify `common/grpcmw` to add that. `internal/adapter/grpc/server.go`
  reads a plain `x-orca-service-id` gRPC metadata header instead — trusted
  only as far as this scaffold's local-dev/internal-mesh boundary goes, and
  explicitly **not** a substitute for real mTLS SPIFFE identity extraction
  before production use.
- **Category/engine branching isn't fully wired into the usecase layer.**
  `domain.Category.Engine()` enforces the category→engine invariant the
  design doc requires (§4), but `WriteCredential`/`ResolveCredential`
  currently route every category through the same Transit-encrypt-then-
  KV-store round trip for simplicity, rather than differentiating (e.g.) the
  SSH secrets engine's certificate-signing flow or `ai-provider-service.md`
  §9's execution-plane-only Transit decrypt path (where
  `ResolveCredential` should return metadata + `vault_path` only for
  `AI_PROVIDER_KEY`, never plaintext, per the design doc's table in §3 —
  the actual generated `ResolveCredentialResponse` always returns `value`
  uniformly, so this scaffold does too). `Category.Engine()` is ready for
  that branching to consume once it's built.
- **No integration test against a real Vault dev server.**
  `internal/adapter/postgres` has a `-tags=integration` test against a real
  Postgres via `testcontainers-go` (same pattern as `usage-service`);
  `internal/adapter/vault` has no equivalent — it's exercised only
  indirectly through `internal/usecase`'s fakes. Add one once a
  Vault-dev-server-via-testcontainers helper exists in `common/testutil`.
- **Metadata write + audit append are not wrapped in one explicit SQL
  transaction.** The design doc (§8) asks for the metadata mutation and its
  audit row to land in "the same Postgres transaction." This scaffold's
  `internal/adapter/postgres.Repository` implements both
  `CredentialMetadataRepository` and `AuditRepository` against the same
  pool but issues them as separate statements; `internal/usecase`'s
  `appendAudit` still fails the whole operation if the audit write fails
  (preserving the *never-best-effort* guarantee), but the two writes are
  not atomic against each other today. Wrapping both in one `pgx.Tx` is the
  recommended next step.
- **No `sqlc` codegen wired**, same as `usage-service` — hand-written SQL
  via `pgx` is a valid destination per the tech stack doc, not the
  codegen-checked default.
- **`common/tracing` has no OTLP exporter configured**, same as
  `usage-service` — spans are created but not shipped anywhere until a
  collector endpoint is wired in.
- **RLS role/grant setup isn't part of this migration.** The `tenant_isolation`
  policies exist, and `access_audit_log`'s "no UPDATE/DELETE beyond
  INSERT/SELECT for this service's own DB role" requirement (§8, §9) is
  documented in a SQL comment, but the actual restricted Postgres role and
  its grants are infrastructure this migration doesn't provision — see
  `architecture/05-data-architecture.md`'s general RLS/role-provisioning
  guidance, same gap `usage-service` leaves open.
- **`PushCiphertext` and `ListAccessAudit` (design doc §3) are not
  implemented.** The generated proto
  (`proto/orca/credentialbroker/v1/credentialbroker.proto`) is a smaller,
  already-finalized surface than the design doc's fuller sketch — it has
  `WriteCredential`/`ResolveCredential`/`RotateCredential`/`RevokeCredential`
  only. This scaffold implements exactly that generated surface; extending
  the proto and this service's usecase/adapter layers together is future
  work, same pattern `usage-service`'s README describes for its own partial
  RPC surface.

## Deviations from the design doc (and why)

- **Domain model is simpler than §4's sketch.** The design doc's
  `CredentialMetadata` includes `Scope`/`ScopeRefID`, `VaultEngine`/
  `VaultMount` as separate stored fields, and a `RotationState` struct
  (`CurrentVersion`/`PreviousVersion`/`RotationGraceUntil`). The actual
  generated `CredentialMetadata` proto message has only `id`, `tenant_id`,
  `owner_id`, `category`, `status`, `vault_path` — this scaffold's domain
  struct mirrors that generated surface plus timestamps, rather than the
  richer, not-yet-generated sketch. `vault_path` alone is enough to address
  a KV v2 location; a Transit key name is derived deterministically from
  category instead of stored separately (see `vault_paths.go`).
- **`AccessAuditEntry` has no `Outcome`/`RequestID` field.** The design
  doc's §4 sketch includes both; the concrete schema this task specified
  (`id, credential_id, accessor_service, occurred_at, action`) doesn't. This
  scaffold follows the simpler, concrete schema — `action` alone
  distinguishes what happened, and a denied/failed resolve attempt is still
  recorded as one `action = 'resolve'` row (see `resolve_credential.go`).
  Adding `outcome` back is a compatible follow-up migration, not a
  redesign.
- **A not-found `ResolveCredential` writes no audit row.** §8 says "every
  access must be audited," but `access_audit_log.credential_id` has a
  `NOT NULL` foreign key to `credential_metadata(id)` — there is no valid
  row to reference for a credential that never existed. This is a
  schema-driven limitation, documented at each of the three places it
  matters: the migration's FK, `resolve_credential.go`'s doc comment, and
  this service's tests.
