# `usage-service`

**Category:** Supporting
**ADR-021 schema:** `usage`
**Replaces (TS):** `ClaudeUsageStore`, `CodexUsageStore`, `OpenCodeUsageStore`
(`orca-claude-usage.json` / `orca-codex-usage.json` — JSON files, not SQL
tables, even in today's TS server mode; see [§10](#10-migration-notes))
**Migration phase:** **0/1 — this is the Phase 0 pilot service.**

## 0. Reference implementation, not just another service

Per
[`migration/ts-to-go-migration-strategy.md`](../migration/ts-to-go-migration-strategy.md)
§"Phase 0", `usage-service` is the **first Go service built end to end** —
before any of the other 16 exist. It ships real AI-CLI usage tracking, and
simultaneously validates the Go + Clean-Architecture + Postgres-per-service
+ Vault pattern this doc set prescribes, before that pattern is trusted for
anything with dependents. §11 doubles as a worked template for whoever
builds service #2 (`annotation-service`, Phase 1's next entry).

Why this is the right pilot: it's a **leaf node with no dependents** —
only `api-gateway` calls it, for reads (dependency graph in
[`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md))
— so a pilot defect has zero blast radius elsewhere. It has **no legacy TS
gap to fix** — unlike `ai-provider-service` (Gap 2) or
`scm-integration-service` (Gap 1), TS's scan/attribute/aggregate logic is
already correct, so the pilot can focus on Go/Postgres/Vault mechanics
alone. And its data is **append-mostly**: sessions are written once, daily
rollups are monotonic counters, no in-place-mutating or
referentially-tangled rows (contrast `ai-provider-service`'s rotation state
machine or `task-service`'s DAG edges) — a wrong backfill is cheap to
detect (row counts / per-day token sums diff directly against source JSON)
and cheap to re-run.

## 1. Overview & responsibility

System of record for **AI-CLI usage and cost tracking**: per-session token
counts and, where available, cost, for Claude Code, Codex CLI, and OpenCode
CLI invocations, plus a daily rollup for dashboarding. Answers "what did
this user/project spend running AI-CLI tools" — observability, not
enforcement (§2).

## 2. Bounded context

Owns **`UsageSession`** (one record per CLI session/rollout: token totals,
model, cwd/git-branch/worktree/repo attribution, turn count, cost where
computable) and **`DailyUsageRollup`** (per tenant/user/day/provider/
model/project — the shape the dashboard and `automation-service`'s
run-attribution query; per
[`07-usage-tracking-stores.md`](../../backend/models/07-usage-tracking-stores.md),
every `AutomationRun.usage` today references back into the Claude/Codex
store by session id — `usage-service` is that same target going forward,
§7).

**Distinction from `ai-provider-service`** (its own doc draws this line in
§2 — restated here so neither doc reads correctly alone):

| | `usage-service` | `ai-provider-service` |
|---|---|---|
| Tracks usage of | A **CLI session** (Claude Code, Codex, OpenCode) | A **provider account** (an API key) |
| Granularity | Per-session record + daily rollup | Daily rollup per account |
| Purpose | Cost **observability** | Quota **enforcement** |
| Consulted at | Dashboards, async, off the hot path | Spawn time, sync, on the hot path |

One agent spawn can feed both — `ai-provider-service`'s quota rollup
increments at spawn time, `usage-service`'s session record is written once
the CLI session ends — different callers, different timing, different
purpose. Merging them would couple fast/simple quota enforcement to
slower/richer usage reporting for no benefit.

## 3. API surface (gRPC sketch)

```proto
service UsageService {
  // Idempotent upsert keyed on (provider, provider_session_id) — see §7 for
  // who calls this and why event-driven is recommended over direct sync.
  rpc RecordUsageSession(RecordUsageSessionRequest) returns (RecordUsageSessionResponse);
  rpc GetUsageToday(GetUsageTodayRequest) returns (DailyUsageRollup);
  rpc ListDailyRollups(ListDailyRollupsRequest) returns (ListDailyRollupsResponse);
  rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);
  rpc GetSession(GetSessionRequest) returns (UsageSession);
}
```

`RecordUsageSessionRequest` carries `tenant_id`/`user_id`, `provider`
(`CLAUDE`/`CODEX`/`OPENCODE`), `provider_session_id` (the CLI's own
session/rollout id — the idempotency key with `provider`), `first_at`/
`last_at`, optional `model`/`primary_worktree_id`/`primary_repo_id`,
`turn_count`, a `TokenUsage` (`input`/`output`/`cache_read`/`cache_write`/
`reasoning` token counters — §4), optional `cost_usd` (from the reporter
only, never synthesized here), and `location_breakdown` — field-for-field
the same shape as the `UsageSession` domain type (§4).

`RecordUsageSession` is a full-state upsert (current totals, not a delta)
— mirrors how the TS scanners already recompute a session's totals from
its transcript on every incremental pass, so this usecase needs no
delta-merge algorithm the TS system never had either.

## 4. Domain model

- **`Provider`** — enum `Claude` / `Codex` / `OpenCode`.
- **`TokenUsage`** — value object, five `int64 >= 0` counters
  (constructor-enforced); `Total()` is a pure method, not a redundantly
  stored column.
- **`UsageSession`** — `ID`, `TenantID`, `UserID`, `Provider`,
  `ProviderSessionID` (natural key with `Provider`), `FirstAt`/`LastAt
  time.Time` (`LastAt >= FirstAt` enforced in the constructor), `Model
  *string`, `LastCWD`/`LastGitBranch *string`, `PrimaryWorktreeID`/
  `PrimaryRepoID *string`, `TurnCount int`, `Tokens TokenUsage`,
  `CostUSD *float64`, `LocationBreakdown []LocationUsage`.
- **`LocationUsage`** — per-project/repo/worktree slice of one session's
  usage, unchanged in shape from `ClaudeUsageLocationBreakdown`/
  `CodexUsageLocationBreakdown`.
- **`DailyUsageRollup`** — `TenantID`, `UserID`, `Provider`, `Day` (date),
  `Model *string`, `ProjectKey`, `ProjectLabel`, `RepoID`/`WorktreeID
  *string`, `TurnCount int`, `Tokens TokenUsage`, `CostUSD *float64`.
- **Domain errors**: `ErrSessionNotFound`, `ErrInvalidProvider`,
  `ErrRollupNotFound`.

`CostUSD` is nullable everywhere and not computed by this service — it
carries whatever the reporting side attached (today: `nil` for
Claude/Codex, an estimate for OpenCode, matching
`OpenCodeUsageParsedEvent.estimatedCostUsd`; Claude/Codex's TS types have
no cost field at all). Faithful port, not scope expansion — real per-model
pricing for all three providers is future work.

## 5. Data model (Postgres — `usage` schema)

**One unified schema with a `provider` discriminator column**, not three
near-duplicate table sets (the TS-adjacent migration `0022` draft used
per-provider tables). The providers' TS types differ only in which token
buckets they populate — a sparse-columns problem, not a schema-shape one —
and three tables would triple every index, migration, and read-path query
for no benefit.

```sql
CREATE TABLE usage.sessions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID NOT NULL,
    user_id              UUID NOT NULL,
    provider             TEXT NOT NULL CHECK (provider IN ('claude','codex','opencode')),
    provider_session_id  TEXT NOT NULL,           -- the CLI's own session/rollout id
    first_at             TIMESTAMPTZ NOT NULL,
    last_at              TIMESTAMPTZ NOT NULL,
    model                TEXT,
    last_cwd             TEXT,
    last_git_branch      TEXT,
    primary_worktree_id  UUID,
    primary_repo_id      UUID,
    turn_count           INTEGER NOT NULL DEFAULT 0,
    input_tokens         BIGINT NOT NULL DEFAULT 0,
    output_tokens        BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens    BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens   BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens     BIGINT NOT NULL DEFAULT 0,
    cost_usd             NUMERIC(12,4),           -- NULL where not computed by the reporter (§4)
    location_breakdown   JSONB NOT NULL DEFAULT '[]',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_sessions_provider_session UNIQUE (provider, provider_session_id)
);
CREATE INDEX idx_sessions_tenant_user ON usage.sessions(tenant_id, user_id, last_at DESC);

CREATE TABLE usage.daily_rollups (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    user_id             UUID NOT NULL,
    provider            TEXT NOT NULL CHECK (provider IN ('claude','codex','opencode')),
    day                 DATE NOT NULL,
    model               TEXT,
    project_key         TEXT NOT NULL,
    project_label       TEXT NOT NULL,
    repo_id             UUID,
    worktree_id         UUID,
    turn_count          INTEGER NOT NULL DEFAULT 0,
    input_tokens        BIGINT NOT NULL DEFAULT 0,
    output_tokens       BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens   BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens  BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens    BIGINT NOT NULL DEFAULT 0,
    cost_usd            NUMERIC(12,4),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- NULL never equals itself in a UNIQUE index (Postgres/SQLite both) —
    -- repo_id/worktree_id must be coalesced to a sentinel by the upsert
    -- usecase, or NULL-bearing rows won't collide as intended. Same gotcha
    -- the TS-adjacent migration 0022 draft documents for this table shape.
    CONSTRAINT uq_daily_rollups_key UNIQUE
        (tenant_id, user_id, provider, day, model, project_key, repo_id, worktree_id)
);
CREATE INDEX idx_daily_rollups_lookup ON usage.daily_rollups(tenant_id, user_id, day DESC);
```

No outbox table: nothing downstream consumes an event *from*
`usage-service` in Phase 0 — it only consumes one (§7) and serves reads.

No `processed_files`/`scan_state` bookkeeping (present in the TS-adjacent
migration `0022` draft, carried over from the JSON stores' incremental-scan
cache). That exists in TS because the backend process itself scans
transcript files on its own local filesystem
(`~/.claude/projects`, `claude-usage/scanner.ts`) and must remember which
files it already parsed. §10 explains why that scanning responsibility
doesn't belong in this stateless, replicated Go service — `usage-service`
receives only fully-formed session records, so there's nothing to dedupe
at the file level.

## 6. Package layout notes

Standard layout per
[`03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md).
One usage-service-specific point:

```
internal/usecase/
├── record_usage_session.go     # on-write upsert of the session row + increment
│                                #   of the matching daily_rollups row, same DB
│                                #   transaction (§8)
├── reconcile_daily_rollups.go  # periodic batch job: recompute daily_rollups
│                                #   from sessions, correct drift (§8) — a
│                                #   safety net, not the primary write path
├── get_usage_today.go / list_daily_rollups.go / list_sessions.go
├── ports.go
```

Both usecases share the `SessionRepository`/`DailyRollupRepository` ports
but stay separate types (per "one exported type per use case") because
they run on different triggers — one from `adapter/grpc/` per inbound RPC,
the other on a timer from `cmd/server/main.go`.

## 7. Dependencies

The write path's origin is an open question at Phase 0: the service that
would naturally hold the Dev Server Agent relay connection for AI-CLI
execution isn't built yet (`infra-fleet-service` is Phase 2). Two shapes
are documented here as a contract for whoever ends up owning that relay:

- **Option A — synchronous gRPC**: the relay-owning service calls
  `RecordUsageSession` directly when the agent reports a session complete.
- **Option B — async event (recommended)**: the relay-owning service
  publishes `orca.usage.session.recorded` to NATS JetStream (transactional
  outbox, per [`04-tech-stack.md`](../architecture/04-tech-stack.md)), and
  `usage-service`'s `adapter/eventbus/` consumer calls the same usecase.

Recommendation: **B**. Usage recording is after-the-fact bookkeeping for a
session the user already finished — it must never add latency or a failure
mode to the interactive path that produced it, and `usage-service` being
briefly unavailable should never affect whether a CLI session completes.

| Direction | Service | Why |
|---|---|---|
| Consumes (async, recommended) | usage-reported event, published by whichever service owns the Dev Server Agent relay for AI-CLI execution (decomposition doc points at `infra-fleet-service`; confirm when that doc is written) | Session-completion data (§3) |
| Called by | `api-gateway` | Usage dashboards: `GetUsageToday`/`ListDailyRollups`/`ListSessions` |
| Called by | `automation-service` | Attributes token/cost to a run by provider + session id — mirrors TS's `AutomationRun.usage` (see [`07-usage-tracking-stores.md`](../../backend/models/07-usage-tracking-stores.md)) |

`usage-service` calls no other service — one of the few in the catalog with
zero synchronous outbound dependencies, part of why it's the right pilot
(§0): the only new integration surface is `usage-service` ↔ its own
Postgres, ↔ Vault, and ↔ NATS as a consumer, nothing chained through
services that don't exist yet.

## 8. Non-functional requirements

- **On-write rollup is the primary aggregation path, not a scheduler.**
  `RecordUsageSession` increments the matching `daily_rollups` row (upsert
  on §5's unique key) in the **same Postgres transaction** as the session
  write — simpler than a periodic batch job, justified because write
  volume (session-completion events, not per-token-streamed events) is low
  enough an inline counter update costs nothing meaningful.
- **Batch reconciliation is a safety net, not the source of truth.**
  `ReconcileDailyRollups` (§6) runs periodically (recommend hourly),
  recomputing `daily_rollups` from `sessions` and correcting drift —
  covers double-delivered events, an on-write bug, or an out-of-band edit.
  Same shape `ai-provider-service`'s health-check job uses for different
  drift.
- **Relaxed availability SLO, appropriate for a pilot.** Per
  [`09-observability-reliability.md`](../architecture/09-observability-reliability.md)'s
  system-wide floor, `usage-service` is explicitly named at **99.5%**
  availability (not the 99.9% floor for sync-critical-path services like
  `ai-provider-service`/`task-service`), because it's "only reachable via
  async events or admin-path operations." A record arriving seconds, or if
  the service is briefly down minutes, late has no user-visible effect —
  nothing blocks on it, nothing enforces a limit against it. A genuinely
  relaxed target, and a comfortable bar for a first-ever Go service.
- **Idempotency**: `RecordUsageSession` is a full-state upsert keyed on
  `(provider, provider_session_id)` — safe to redeliver, satisfies the
  production-readiness checklist's idempotency item, and is what makes
  Option B's at-least-once NATS delivery correct with no extra dedup
  bookkeeping.

## 9. Security notes

Standard posture, called out only because "usage data" can sound
low-stakes and isn't:

- **Tenant isolation**: `tenant_id NOT NULL` on every table, application-
  layer filtering in every repository method, RLS as secondary defense —
  the catalog-wide baseline.
- **Access control via OPA**: usage data reveals activity patterns (when a
  user works, which projects consume the most AI-CLI time, relative spend
  across a team) sensitive in aggregate. Reads are authorized per
  tenant/user through the same OPA policy path every service's reads go
  through — no bespoke authorization logic for this service.
- **No secret material.** No credential/token/key columns in this schema.
  Its only Vault interaction is the standard per-service dynamic DB
  credential bootstrap (§11) — one of the simplest Vault policies in the
  catalog.

## 10. Migration notes

The **Phase 0 pilot migration** — structured to be copied, not just its
outcome:

- **Source**: `orca-claude-usage.json`/`orca-codex-usage.json`
  (`ClaudeUsagePersistedState`/`CodexUsagePersistedState`, see
  [`07-usage-tracking-stores.md`](../../backend/models/07-usage-tracking-stores.md)),
  plus the OpenCode equivalent — JSON files on the backend host's local
  filesystem, not SQL tables, even in today's TS server mode. No existing
  Postgres store to migrate away from; the backfill source is these files.
- **Backfill mechanics**: a one-time script reads each file's
  `sessions`/`dailyAggregates` arrays into `usage.sessions`/
  `usage.daily_rollups` per §5's field mapping (1:1 where a field exists in
  both shapes; `reasoning_tokens`/`cost_usd` populate only from
  Codex/OpenCode source data). Verified against `staging` by diffing
  per-day token sums and session counts against source JSON before
  touching `production` — cheap because the data is append-mostly and
  every row is independently checkable (§0).
- **What does not migrate**: the JSON stores' `processedFiles`/`scanState`
  bookkeeping (§5) has no destination table — scan-cache plumbing for a
  filesystem-scanning architecture this service doesn't have. Dropped;
  nothing downstream reads it.
- **Ingestion model changes, not just storage.** The one place "faithful
  port" (§0) needs a caveat: TS's stores are populated by the backend
  process scanning `~/.claude/projects` etc. on its own host — the same
  category of host-filesystem dependency
  [`02-microservices-decomposition.md`](../architecture/02-microservices-decomposition.md)
  gives as the reason `ai-vault.*` is **not** carried forward at all.
  Unlike `ai-vault.*`, this data doesn't have to be dropped: the scan can
  instead happen where the CLI actually ran (the Dev Server Agent, which
  already has host filesystem access) and be reported to `usage-service` as
  a session-completion event (§7) instead of self-scanned by a stateless,
  replicated Go process. New work for whoever builds the agent-side
  reporting logic — not a lift-and-shift of `claude-usage/scanner.ts`.
- **Dual-write / cutover**: per the standard criteria in
  [`ts-to-go-migration-strategy.md`](../migration/ts-to-go-migration-strategy.md)
  §"Cutover criteria," run the TS scanners writing their JSON files and the
  new event-based path in parallel for the recommended 1–2 week soak
  window, comparing daily rollup sums, before retiring the TS scanners for
  server-mode deployments.

## 11. Phase 0 pilot checklist — building this service, operationally

Full gate criteria live in
[`standards/production-readiness-checklist.md`](../standards/production-readiness-checklist.md);
this is the *order of operations* to get there, reusable almost verbatim
for service #2:

1. **Scaffold** — `usage-service/` directory, own `go.mod`, standard
   package layout (§6), CI wired for `golangci-lint`/`govulncheck`/tests.
2. **Database** — stand up this service's own PostgreSQL database
   (physical, per-service), write the first `golang-migrate` migration
   for §5's schema.
3. **Vault registration** — Kubernetes auth role for `usage-service`'s
   pods, Database secrets engine config for this service's Postgres,
   Vault policy scoped to exactly that DB-credential path (§9 — no
   KV/Transit access needed).
4. **Domain + usecase** — `internal/domain/` types (§4) with mock-free
   unit tests; `internal/usecase/` (§6) against hand-written port fakes.
5. **Adapters** — `adapter/postgres/` (`sqlc` against §5's schema,
   `testcontainers-go` integration tests), `adapter/grpc/` (§3,
   contract-tested against the `.proto`), `adapter/eventbus/` (NATS
   consumer, §7).
6. **`cmd/server/main.go`** — wire config → adapters → usecases → inbound
   handlers; `/healthz`/`/readyz`; `orca-go-common` observability
   middleware.
7. **CI green** — lint, unit, integration, `buf breaking`, image build.
8. **Deploy to `dev`** — Helm chart, Vault Agent sidecar, confirm the
   dynamic DB credential flow end to end (kill a lease, confirm the pool
   re-fetches without a restart).
9. **Validate against the full production-readiness checklist** before
   treating any of this as a template — a pilot that skips its own
   checklist proves nothing about the checklist's usability for service #2.
10. **Write down what was awkward** — anything harder than it should have
    been in steps 1–9 gets fixed at the shared-tooling level before
    service #2 starts, not rediscovered independently by its author.

Only after this list is fully checked does Phase 1 proceed to
`annotation-service`.
