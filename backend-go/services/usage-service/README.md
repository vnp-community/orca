# usage-service

The Phase 0 pilot / reference implementation for the Go rewrite — see
[`specs/backend-go/services/usage-service.md`](../../../specs/backend-go/services/usage-service.md)
for the full design and
[`specs/backend-go/migration/ts-to-go-migration-strategy.md`](../../../specs/backend-go/migration/ts-to-go-migration-strategy.md)
for why this service is built first. **Every other service in this
repository follows the exact package layout and conventions demonstrated
here** — read this README before scaffolding another service.

## What's implemented

- `internal/domain/` — `UsageSession`/`DailyUsageRollup` entities with
  invariant-enforcing constructors, pure unit tests.
- `internal/usecase/` — `RecordUsageSession`, `GetDailyUsage`, `ListSessions`,
  each tested against an in-memory fake `Repository`, no real Postgres/NATS
  needed.
- `internal/adapter/postgres/` — real `pgx`-backed repository, hand-written
  SQL (see `architecture/04-tech-stack.md` — `sqlc` codegen is the eventual
  target; this scaffold writes the equivalent queries directly to avoid an
  extra build-time toolchain dependency). `SaveSession` writes the session,
  the daily rollup, AND a `usage.outbox_events` row in one transaction —
  the real transactional-outbox pattern (Epic G, `docs/execution-plan.md`),
  not a direct publish call. Also implements `common/outbox.Store`
  (`FetchUnpublished`/`MarkPublished`) for the relay below.
- `internal/adapter/grpc/` — implements the generated
  `usagev1.UsageServiceServer`, pure wire<->usecase translation.
- `common/outbox.Relay` (started in `cmd/server/main.go`, no
  service-specific adapter package needed anymore) — polls
  `usage.outbox_events` and publishes `orca.usage.session.recorded` via
  `common/eventbus` (NATS JetStream). This replaced the previous
  `internal/adapter/eventbus` package, which called the publisher directly
  from the usecase right after the DB write — see "Known gaps" below,
  now closed.
- `migrations/0001_init.{up,down}.sql` — real DDL: `usage.sessions`,
  `usage.daily_rollups`, RLS policies, the `(tenant_id, request_id)`
  idempotency constraint. `migrations/0002_outbox.{up,down}.sql` — real DDL:
  `usage.outbox_events`.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, NATS connection (degrades gracefully if unavailable — the
  outbox relay simply doesn't start; enqueued rows still get written
  durably and queue up until a future restart), gRPC server with the
  shared interceptor chain, health/readiness HTTP server, graceful
  shutdown on SIGTERM (including the outbox relay goroutine).

## Running locally

```sh
# from backend-go/
docker compose up -d postgres nats   # see ../../docker-compose.yml
migrate -path services/usage-service/migrations \
  -database "$DATABASE_DSN" up       # golang-migrate; see architecture/05

cd services/usage-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/usage?sslmode=disable \
NATS_URL=nats://localhost:4222 \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **No `sqlc` codegen wired** — `adapter/postgres/repository.go` is
  hand-written SQL via `pgx`, which is a valid destination per the tech
  stack doc but not the codegen-checked path that doc describes as the
  default. Add a `sqlc.yaml` + regenerate once the query set stabilizes.
- ~~Event publishing bypasses the transactional outbox~~ — **closed** (Epic
  G, `docs/execution-plan.md`, 2026-08-17). `RecordUsageSession` now writes
  its outbox row inside the same transaction as the session/rollup write
  (`internal/adapter/postgres.Repository.SaveSession`); `common/outbox.Relay`
  polls and publishes it asynchronously. Note this was built on explicit
  user direction even though the item's own stated precondition ("a second
  real consumer exists beyond notification-service") is still not met as of
  this pass — `orca.usage.session.recorded` still has zero consumers.
- ~~`common/secrets` (Vault) is not wired into this service's `main.go`~~ —
  **closed** (`docs/execution-plan.md` §3 Phase 1): `main.go` now resolves
  the DSN via `secrets.DatabaseCredentialsFromFile(cfg.DatabaseCredentialsFile)`
  (env `DATABASE_CREDENTIALS_FILE`, defaulting to a Vault-Agent-rendered
  path), falling back to `DATABASE_DSN` itself when the file doesn't exist —
  which is what local dev / testcontainers still uses.
- ~~`common/tracing` has no OTLP exporter configured~~ — **closed**: `Init`
  now batches spans to a real `otlptracegrpc` exporter whenever
  `OTLP_ENDPOINT` is set; still exporter-less (spans created, not shipped)
  when it's empty, so local dev/tests are unaffected.
- **`sqlc` migration deliberately not done here** — `docs/execution-plan.md`
  §5 explicitly calls this out as "worth doing as one focused pass across
  all services... not per-service ad hoc"; doing it just for usage-service
  would contradict that guidance, so it stays hand-written `pgx` until that
  cross-service pass happens.
- Full RPC surface per the service doc's exhaustive API section isn't
  implemented — only the 3 core operations. Extend `proto/orca/usage/v1/usage.proto`
  and this service's usecase/adapter layers together as more of the surface
  is needed.
