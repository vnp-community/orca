# Data Architecture — PostgreSQL

## Database-per-service, physically

Each of the 13 data-owning services (see
[`02-microservices-decomposition.md`](./02-microservices-decomposition.md))
gets its **own PostgreSQL database** — not a schema in a shared instance.
This is a deliberate departure from ADR-021's "schema-per-service in one
shared instance," which that ADR itself frames as a *pragmatic intermediate
step for migrating an existing system incrementally* (its own Phase 3 defers
real separation). A ground-up Go build has no such migration constraint, so
it goes straight to the end state:

- Each service's database can be its **own Postgres instance/cluster**
  (recommended for `auth`, `tenant`, `credential` — the highest-blast-radius
  services) or a logical database within a shared managed Postgres cluster
  (acceptable for lower-traffic services, e.g. `annotation`, `usage`) —
  decided per service based on scale/compliance needs, documented in that
  service's own doc.
- No cross-database queries, no cross-database foreign keys — full stop,
  not "discouraged." A service that needs data another service owns calls
  that service's API. This is enforced at the infrastructure level (separate
  Vault-issued credentials per database, network policy) as well as by
  convention, matching ADR-021's principle 3 ("cô lập theo Postgres ROLE")
  taken to its logical conclusion.
- **`credential-broker-service`'s** database holds metadata only — no
  secret material, ever. See
  [`06-secrets-vault-architecture.md`](./06-secrets-vault-architecture.md).

## Multi-tenancy

Same core model as ADR-021 §3, carried forward:

- Every tenant-scoped table has `tenant_id UUID NOT NULL` (no cross-database
  FK to `tenant-service`'s `companies` table — logical reference, validated
  by calling `tenant-service`).
- **Primary enforcement: application-layer tenant scoping.** Every
  repository method in every service's `adapter/postgres/` layer takes the
  tenant ID from the validated request context (propagated via gRPC
  metadata from `api-gateway`, which extracted it from the JWT/session) and
  includes it in every query — never optional, never inferred from request
  body content. A lint rule / code-review checklist item
  (`standards/production-readiness-checklist.md`) requires every generated
  `sqlc` query touching a tenant-scoped table to take `tenant_id` as a bound
  parameter.
- **Secondary defense: Postgres Row-Level Security (RLS)**, enabled on every
  tenant-scoped table, policy driven by `current_setting('app.tenant_id')`
  set via `SET LOCAL` at the start of every transaction. Same rationale as
  ADR-021: this is a defense-in-depth backstop against an application bug,
  not the primary mechanism — a service must never rely on RLS alone to be
  correct, both because a bug could still leave `SET LOCAL` uncalled and
  because it doesn't help isolate at the schema/service-boundary level (that
  job belongs to database-per-service in the first place).
- System-wide, non-tenant-scoped tables (e.g. `credential-broker-service`'s
  Vault-path registry, `schema_migrations`) explicitly have no `tenant_id`
  column — don't add one just for consistency.

## Migration conventions

- `golang-migrate`, sequential numeric prefixes per service
  (`0001_init.up.sql` / `0001_init.down.sql`, …), mirroring the TS system's
  own migration-numbering habit for continuity of mental model across teams
  working on both codebases during the transition.
- Every migration must have a working `down` migration — not optional, not
  "we'll write it if we need to roll back." Enforced by CI (a check that
  runs `up` then `down` then `up` again against a fresh `testcontainers-go`
  Postgres for every migration file changed in a PR).
- No destructive migration (`DROP COLUMN`, `DROP TABLE`, `NOT NULL` without
  a backfill) merges without: (1) a backfill script if data exists, (2) a
  minimum one-release soak period where both old and new shape are
  supported (expand/contract pattern), matching how the TS system's own
  migrations (`0019` retrofitting `tenant_id` as nullable before a later
  migration made it `NOT NULL`) already handle this.

## Cross-service data consistency

No distributed transactions across service databases (2PC is explicitly
rejected — it doesn't compose well with database-per-service and creates
availability coupling between otherwise-independent services). Two patterns
instead, chosen per interaction:

### Transactional outbox + async events (default)

For "service A does something, other services need to eventually know":

1. Service A writes its domain state change and an outbox row (event
   payload) in the **same Postgres transaction**.
2. A relay process (either a goroutine in-process with a polling loop, or
   Debezium-style CDC — start with polling, it's simpler and sufficient at
   this scale) reads unpublished outbox rows and publishes them to NATS
   JetStream.
3. Consumers process at-least-once; consumer-side handlers must be
   idempotent (dedupe on event ID).

Example: `task-service` completes a task → outbox row `task.completed` →
`notification-service` consumes it and fans out a push notification. If the
publish step fails or is delayed, the task state itself was already
committed correctly — no dual-write inconsistency.

### Synchronous saga (only where a caller needs to know the outcome before responding)

For "service A needs service B to also succeed before A's operation can be
considered complete, and the caller is waiting":

- A's usecase layer calls B's gRPC API synchronously as one step in an
  explicit saga (a sequence of compensable steps, not a database
  transaction). If step 2 fails, step 1's compensating action runs.
- Used sparingly — e.g. `project-service.CreateProject` calling
  `infra-fleet-service` to validate a `devServerId` exists before
  committing the binding. Most cross-service interactions should prefer the
  outbox pattern; reach for a saga only when the caller genuinely cannot
  return success before a dependent step is confirmed.

## Read models / query needs across service boundaries

Where the frontend needs data assembled from multiple services in one
view (e.g., a task list showing task-service data alongside the assignee's
tenant-service profile), `api-gateway` performs the aggregation
(parallel gRPC calls, merge in the edge layer) rather than any service
reaching into another's database. If aggregation becomes a performance
problem for a specific high-traffic view, the fix is a purpose-built
read-model service subscribing to the relevant services' events (CQRS-style)
— not a shared database. Not built by default; call this out explicitly if
a specific view's latency requires it.

## Mapping from the TS system's existing schema

Every table in the TS system's 17 migrations
(`specs/backend/models/02-sql-schema-catalog.md`) maps to exactly one Go
service's database — the table is [`services/00-service-catalog.md`](../services/00-service-catalog.md)'s
"replaces" column. Column-level DDL for each service's database is
specified in that service's own doc, not repeated here — this document
covers the cross-cutting policy, not the field-by-field schema.
