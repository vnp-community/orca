# ai-provider-service

The system of record for **which AI provider accounts exist** (Anthropic,
OpenAI, Google, Azure, AWS Bedrock, Ollama, vLLM) and for the daily
quota/spend rollup used to enforce them before an agent spawn — see
[`specs/backend-go/services/ai-provider-service.md`](../../../specs/backend-go/services/ai-provider-service.md)
for the full design. Built following the exact package layout and
conventions established by
[`usage-service`](../usage-service/README.md), the Phase 0 reference
implementation.

This service's entire reason to exist is a security property, not a
feature: **its database has zero secret columns, by construction, not
convention.** Every table, every domain type, and every wire message in
this codebase carries a `credential_ref` — an opaque pointer
`credential-broker-service` resolves — and nothing else. A full dump of
this service's database, or a bug that logs every field of every struct it
defines, still never yields a usable credential.

## What's implemented

- `internal/domain/` — `ProviderAccount` entity with an invariant-enforcing
  constructor (rejects empty `tenant_id`, an unrecognized `ProviderType`,
  an unrecognized `AccountStatus`/`AccountScope`, and any scope/ref
  mismatch), plus `QuotaState` and `ErrNoProviderAvailable{Reason}`. Pure
  unit tests, including one (`TestProviderAccount_HasNoSecretField`) whose
  entire job is to flag in review if a future field addition ever puts
  secret material on this struct.
- `internal/usecase/` — `CreateAccount`, `ResolveProvider`, `RotateKey`,
  `GetUsageToday`, each tested against in-memory fakes for
  `ProviderAccountRepository`/`UsageRepository`/`CredentialBrokerClient` —
  no real Postgres or credential-broker-service needed.
  - **`ResolveProvider` implements the user→project→server cascade** —
    narrowest scope wins first. This is the one piece of logic the design
    doc calls out as previously documented backwards in TS; see
    `resolve_provider_test.go`'s
    `TestResolveProvider_UserScopeWinsOverProjectScope`, which asserts the
    cascade order directly (an account at both scopes exists; the user-scope
    one must win), plus fallback/skip/cross-tenant/no-match cases.
  - `ResolveProvider` makes **no cross-service call** — it reads only this
    service's own `accounts` table, per the design doc's `Resolve` hot-path
    latency budget (§8).
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  both `ProviderAccountRepository` and `UsageRepository`, hand-written SQL
  (same `sqlc`-is-the-eventual-target choice usage-service made).
- `internal/adapter/grpc/` — implements the generated
  `aiproviderv1.AiProviderServiceServer`, pure wire↔usecase translation.
- `internal/adapter/grpcclient/` — **stub** `CredentialBrokerClient` (see
  "The stub" below).
- `migrations/0001_init.{up,down}.sql` — `ai_provider.accounts` (scope,
  `user_id`/`project_id` nullable per the `scope_ref_matches_scope` CHECK,
  `rotation_grace_until`, `credential_ref` — no other credential-shaped
  column exists) and `ai_provider.usage` (daily rollup: `cost_usd`,
  `request_count`), both with `tenant_id` row-level security policies
  matching usage-service's pattern.
- `cmd/server/main.go` — composition root: config load, Postgres pool,
  gRPC server with the shared interceptor chain, health/readiness HTTP
  server, graceful shutdown on SIGTERM.

## The stub: `credential-broker-service` is not wired

This is the most important gap in this scaffold, called out explicitly per
the task brief, because **this is where TS Gap 2's fix ultimately lives.**
TS Gap 2 (`backend-agent-target-architecture.md` §"Gap 2") was that the old
system's spawn path forwarded a plaintext `resolvedApiKey` from backend to
the execution agent — a real violation of "backend never sees plaintext."
The Go design closes this by having `ai-provider-service` and
`credential-broker-service` push ciphertext to the execution plane ahead of
time, so `Resolve` only ever hands back a `credential_ref` (see the design
doc §9's sequence diagram). **That fix depends on `credential-broker-service`
existing**, and it doesn't yet in this workspace — the two services are
explicitly Phase 2, "built and cut over together... neither is
independently useful" (§10).

`internal/adapter/grpcclient/credential_broker_client.go` is therefore a
stub: it doesn't dial anything. `WriteCredential`/`RotateCredential` return
locally-synthesized, opaque `CredentialRef{ID, Status}` values (a
`uuid`-suffixed string and a status like `"pending_push"`) so the rest of
this service's create/rotate paths — and their unit tests — can be
exercised end-to-end today.

**No fake plaintext leaked into the stub, even for realism's sake.** This
matters concretely, not just as a principle: `credential-broker-service`'s
real proto **already exists** at
[`proto/orca/credentialbroker/v1/credentialbroker.proto`](../../proto/orca/credentialbroker/v1/credentialbroker.proto),
and its `ResolveCredentialResponse` carries `bytes value = 1` —
plaintext, by that proto's own doc comment ("caller must not persist or
log"). It would be easy for a "more realistic" stub to wire
`usecase.CredentialBrokerClient.ResolveCredential` straight through to that
real RPC once a `credentialbrokerv1` client exists — that would be wrong.
The stub's `ResolveCredential` deliberately does **not** call that RPC and
never will; it returns synthesized status metadata only
(`CredentialRef{ID, Status: "active"}`), matching what
`usecase.CredentialRef` can even express — there is no plaintext field on
that type to populate by mistake. The package doc comment on
`credential_broker_client.go` spells this out as a
"SECURITY-CRITICAL — read before wiring the real client" note so it isn't
lost when the real dial is finally wired in.

## Known gaps / follow-ups (tracked, not silently skipped)

- **`credential-broker-service` client is a stub** — see above. Replace
  `grpcclient.New`'s body with a real dial once that service ships;
  `internal/config` already threads `CREDENTIAL_BROKER_ADDR` through for
  that purpose.
- **`CreateAccountRequest`/`ResolveProviderRequest` proto messages are
  minimal** — the generated proto only carries `tenant_id`+`type` for
  create (no `scope`, `user_id`/`project_id`, or encrypted-blob field yet)
  and `tenant_id`+`user_id`+`project_id` for resolve (no `dev_server_id` or
  `model_hint`, present in the design doc's fuller sketch). `CreateAccount`
  defaults to server scope with no ref when the request doesn't specify
  one; `ResolveProvider`'s server-scope fallback is tenant-wide rather than
  per-dev-server. Extend `proto/orca/aiprovider/v1/aiprovider.proto` and
  this service's usecase/adapter layers together once the fuller surface
  (`WriteCredential`, `TestConnection`, `GetAccount`, `ListAccounts`,
  `UpdateAccount`, `DeleteAccount`) is needed.
- **No `PushCiphertext` port** — the design doc's §9 ciphertext-push flow
  (account creation/rotation isn't complete until ciphertext reaches the
  target dev server) isn't modeled here; `RotateKey` leaves
  `RotationGraceUntil` unset rather than guessing at a value. Add this once
  `infra-fleet-service`'s dev-server resolution API exists to call.
- **No transactional outbox** — the design doc's §5 `ai_provider.outbox`
  table (lifecycle events for `infra-fleet-service`/`credential-broker-service`)
  isn't implemented; this scaffold has no event-publishing adapter at all
  yet (unlike usage-service's `internal/adapter/eventbus/`, included there
  because that service already had a defined event consumer).
- **`common/secrets` (Vault) is not wired into this service's `main.go`** —
  same known gap as usage-service: `DATABASE_DSN` is read directly from the
  environment for local dev.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in.
- **No health-check reconciliation job** — the design doc's §8 "every 15
  minutes, call `TestConnection` per active account" cron job isn't
  implemented; `status`/`last_health_check_at` only change via `RotateKey`
  today. `last_health_check_at` itself isn't modeled on `ProviderAccount`
  yet, since nothing writes it.

## Deviations from the design doc (and why)

- **`ProviderAccount` carries fewer fields than §4's sketch** — no `Label`,
  `ModelHint`, `BaseURL`, `QuotaLimitDay`, `LastHealthCheckAt`, or
  `CreatedBy`. The task scope for this scaffold was the entity's
  security-relevant and cascade-relevant fields (id, tenant, provider type,
  status, credential ref, scope + ref, rotation grace); the remaining
  fields are pure display/config metadata addable later without touching
  the invariants this package enforces.
- **`ai_provider.usage` has a `tenant_id` column** the task's column list
  didn't mention, added so its RLS policy could match `accounts`' pattern
  ("`tenant_id` RLS like usage-service") — an RLS policy needs a
  `tenant_id` column to filter on.
- **`ProviderAccountRepository.UpdateStatus` takes a small input struct**
  (`status` + optional `credential_ref` + optional `rotation_grace_until`)
  rather than a bare status value, so `RotateKey` can persist a status
  transition and its new credential ref atomically in one call instead of
  two (avoiding a window where `status="rotating"` but `credential_ref`
  still points at the ref mid-rotation). Still a single repository method,
  per the 4-method (`Create`/`Get`/`List`/`UpdateStatus`) shape.
- **`ResolveProvider`'s public method is named `Resolve`, not `Execute`** —
  the other three usecases use `Execute` (matching usage-service's
  convention), but `ResolveProvider` also implements the
  `ProviderResolutionPort` interface, whose method needed a name distinct
  from the generic per-usecase convention to read sensibly at that
  interface's call sites.
- **`CredentialBrokerClient.ResolveCredential` never resolves to
  plaintext** — see "The stub" above. This is a deliberate, permanent
  constraint, not a temporary scaffold simplification.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/ai-provider-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/ai-provider-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/ai_provider?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```
