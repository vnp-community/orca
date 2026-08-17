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
  `RotateCredential`, `RevokeCredential`, plus three RPCs added for Epic B
  (`docs/execution-plan.md` §9, 2026-08-17) so every consumer that needed
  a real client had a real RPC shaped for its actual call pattern:
  `GetCredentialMetadata` (metadata-only, no Vault call, no audit row —
  for `ai-provider-service`, which must never see plaintext),
  `ResolveCredentialByOwner` (fail-closed/audited like `ResolveCredential`,
  but keyed by `(tenant_id, category, owner_id)` instead of an opaque id —
  for `scm-integration-service`/`issue-tracking-service`, which are never
  handed a `credential_id`), and `SignVapidPayload` (a narrow Transit-sign
  passthrough — for `notification-service`, which used to call Vault
  directly for this). Each tested against in-memory fake
  `CredentialMetadataRepository`/`AuditRepository`/`SecretStore`, no real
  Postgres/Vault needed. Notably tested:
  - `ResolveCredential`/`ResolveCredentialByOwner` write their access-audit
    row **before** returning the resolved value, on every path that
    reaches Vault (success, revoked, Vault error) — see
    `resolve_credential_test.go`'s call-order assertion. Both share this
    ordering guarantee via one `resolveMetadata` helper
    (`resolve_credential.go`) so they can't drift.
  - `ResolveCredentialByOwner`'s owner-keyed lookup filters out revoked
    rows at the SQL level (`GetByOwner`), so a revoked credential looks
    identical to "never existed" for this RPC — a real, intentional
    behavior difference from the by-id `ResolveCredential`, which can
    still find and report a revoked row. See
    `TestResolveCredentialByOwner_RevokedIsNotFound`.
  - `GetCredentialMetadata` never calls `SecretStore` or `AuditRepository`
    — asserted directly in `get_credential_metadata_test.go`, since "this
    RPC exposes no secret material" is the whole point of it existing.
  - `RevokeCredential` genuinely calls `SecretStore.RevokeSecret` (not just
    the Postgres status flip) — see `revoke_credential_test.go`.
  - `RevokeCredentialByOwner` (added alongside a follow-up pass closing
    `scm-integration-service`'s `RevokeAuth` gap, 2026-08-18) mirrors
    `RevokeCredential`'s Vault-revoke-then-audited-status-transition logic,
    keyed by `(tenant_id, category, owner_id)` via the same `GetByOwner`
    `ResolveCredentialByOwner` uses. Because `GetByOwner` filters revoked
    rows out at the SQL level, this RPC has **no** "already revoked" success
    branch the way `RevokeCredential` does by id — revoking an
    already-revoked or never-existent owner-keyed credential both surface
    as plain `CREDENTIAL_NOT_FOUND`. See
    `revoke_credential_by_owner_test.go` and that file's doc comment.
  - A failed audit write fails the whole operation, per the design doc's
    "audit log write must never be best-effort" rule — see
    `write_credential_test.go`/`resolve_credential_test.go`. For
    `WriteCredential`/`RotateCredential`/`RevokeCredential` specifically,
    this is now a real atomicity guarantee, not just error propagation: the
    metadata mutation and audit append run inside one `TxRunner.RunInTx`
    call, so a failed audit append rolls the metadata mutation back too —
    see `TestWriteCredential_AuditWriteFailure_FailsTheWholeOperation`'s
    rollback assertion and "Known gaps" below.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  both `CredentialMetadataRepository` and `AuditRepository` against the
  `credential` schema. **Not one column in this schema can hold a secret
  value** — see `migrations/0001_init.up.sql`'s prominent comment.
- `internal/adapter/vault/` — **this service's real, working Vault
  integration**, wrapping `common/secrets.Client` as a thin pass-through:
  `TransitEncrypt`/`TransitDecrypt`/`KVWrite`/`KVRead`/`RevokeSecret`/`Ping`
  all delegate directly to the matching `common/secrets` method, no
  reinterpretation. This is the one service in the whole backend-go
  scaffold where that package is wired for real, non-stub, tenant-secret
  use — every other service's Vault identity is scoped only to its own
  dynamic DB credential lease. `RevokeSecret` now calls
  `common/secrets.Client.KVDestroyMetadata` (Vault's native
  `DELETE <mount>/metadata/<path>`, a genuine permanent delete) and `Ping`
  now calls `common/secrets.Client.Ping` (a real `Sys().Health()` call) —
  see "Known gaps" below for what changed and `secret_store_test.go` for
  the httptest-backed regression coverage proving both.
- `internal/adapter/grpc/` — implements the generated
  `credentialbrokerv1.CredentialBrokerServiceServer`: the original
  `WriteCredential`/`ResolveCredential`/`RotateCredential`/`RevokeCredential`
  four (a subset of the design doc's fuller sketch; see "Deviations" below)
  plus `GetCredentialMetadata`/`ResolveCredentialByOwner`/`SignVapidPayload`/
  `RevokeCredentialByOwner`, added by Epic B and not in the original design
  doc sketch at all — see the usecase bullet above for why each exists.
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
go test ./...                 # unit tests (domain/, usecase/, adapter/vault/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

`internal/adapter/vault/secret_store_test.go` now exercises `Ping` and
`RevokeSecret` against an `httptest`-backed fake Vault server (same pattern
as `common/secrets`' own tests) — proving `Ping` calls `GET /v1/sys/health`
and `RevokeSecret` calls `DELETE <mount>/metadata/<path>`, not the KV paths
either used before. There is still no integration-tagged test against a
*real* Vault dev server — see "Known gaps".
`internal/adapter/postgres/repository_test.go`'s
`TestRepository_RunInTx_CommitsBothOnSuccess`/
`TestRepository_RunInTx_RollsBackBothOnFailure` (integration-tagged, real
Postgres via testcontainers) prove `RunInTx`'s metadata mutation and audit
append commit or roll back together.

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
- ~~**`RevokeCredential`'s Vault-side revocation is an overwrite, not a
  native delete.**~~ **Done.** `common/secrets.Client` now exposes
  `KVDestroyMetadata` (Vault's native `DELETE <mount>/metadata/<path>`), and
  `internal/adapter/vault.SecretStore.RevokeSecret` calls it directly —
  genuinely permanent, scrubbing every KV v2 version, not just the current
  one. Still does not touch the Transit key (Transit keys are shared per
  category, so deleting one would revoke every credential in that category)
  — the fail-closed status check in `ResolveCredential` remains defense in
  depth alongside this Vault-side delete, not a substitute for it. See
  `internal/adapter/vault/secret_store.go`'s doc comment and
  `secret_store_test.go`'s `TestRevokeSecret_CallsDestroyMetadata`.
- ~~**Vault health check is a heuristic, not a native ping.**~~ **Done.**
  `common/secrets.Client` now exposes `Ping` (a real `Sys().Health()` call),
  and `internal/adapter/vault.SecretStore.Ping` calls it directly, replacing
  the prior well-known-absent-KV-path heuristic. See
  `secret_store_test.go`'s `TestPing_CallsRealHealthEndpoint`/
  `TestPing_UnreachableVaultErrors`.
- **`RequestingService` identity is a plain gRPC metadata header, not real
  mTLS/JWT identity.** The design doc requires `RequestingIdentity` to be
  "resolved JWT/mTLS subject, not client-asserted" (§4). `common/grpcmw`'s
  `TenantExtractionInterceptor` extracts tenant/user identity this way
  already, but not a calling-service identity, and this service must not
  modify `common/grpcmw` to add that. `internal/adapter/grpc/server.go`
  reads a plain `x-orca-service-id` gRPC metadata header instead — trusted
  only as far as this scaffold's local-dev/internal-mesh boundary goes, and
  explicitly **not** a substitute for real mTLS SPIFFE identity extraction
  before production use. **Still true after Epic B wired all 4 consumers
  (2026-08-17): none of their new gRPC clients set this header either** —
  every access-audit-log row Epic B's new call paths generate will show an
  empty/`"unknown"` `accessor_service` until a client actually sets it.
- **`common/secrets.TransitEncrypt` stands in for a dedicated Transit
  "sign" operation in `SignVapidPayload` too.** `common/secrets` exposes
  `TransitEncrypt`/`TransitDecrypt` today, not Vault's asymmetric-key
  `sign` endpoint — `usecase.SignVapidPayload` uses `TransitEncrypt` as the
  available equivalent, the exact same choice `notification-service`'s
  pre-Epic-B `vaultsigner.Signer` made when it called Vault directly (see
  that service's README). Swap it for a real `transit/sign/<key>` call
  once `common/secrets` grows one.
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
  `internal/adapter/vault` has no equivalent — `secret_store_test.go` now
  covers `Ping`/`RevokeSecret` against an `httptest`-backed fake Vault
  server (same pattern as `common/secrets`' own tests), which is real
  regression coverage for this adapter's request shapes, but still not a
  real Vault binary. Add a real-dev-server test once a
  Vault-dev-server-via-testcontainers helper exists in `common/testutil`.
- ~~**Metadata write + audit append are not wrapped in one explicit SQL
  transaction.**~~ **Done for the three usecases that mutate metadata**
  (`WriteCredential`, `RotateCredential`, `RevokeCredential` —
  `ResolveCredential`/`ResolveCredentialByOwner`/`GetCredentialMetadata`/
  `SignVapidPayload` never mutate metadata, so this doesn't apply to them).
  `internal/usecase/ports.go` adds a `TxRunner` port —
  `RunInTx(ctx, fn)` opens one Postgres transaction and hands `fn` a
  `CredentialMetadataRepository`/`AuditRepository` pair scoped to it,
  reusing the existing port shapes rather than introducing
  transaction-specific interfaces. `internal/adapter/postgres.Repository`
  implements it via `pgx.BeginFunc`: its query methods now run against a
  `db dbtx` field (satisfied by either `*pgxpool.Pool` or a `pgx.Tx`) so no
  SQL is duplicated between the pooled and transactional paths. Each of the
  three usecases now wraps its metadata mutation + `appendAudit` call in one
  `RunInTx` call — a failed audit append rolls back the metadata mutation
  too, turning the pre-existing "audit failure fails the whole operation"
  *convention* into a real atomicity *guarantee*. See
  `internal/usecase/write_credential_test.go`'s
  `TestWriteCredential_AuditWriteFailure_FailsTheWholeOperation` (in-memory
  `fakeTxRunner` rollback) and
  `internal/adapter/postgres/repository_test.go`'s
  `TestRepository_RunInTx_RollsBackBothOnFailure` (real Postgres, requires
  `-tags=integration`).
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
