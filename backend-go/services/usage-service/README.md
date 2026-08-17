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
  each tested against an in-memory fake `Repository`/`EventPublisher`, no
  real Postgres/NATS needed.
- `internal/adapter/postgres/` — real `pgx`-backed repository, hand-written
  SQL (see `architecture/04-tech-stack.md` — `sqlc` codegen is the eventual
  target; this scaffold writes the equivalent queries directly to avoid an
  extra build-time toolchain dependency).
- `internal/adapter/grpc/` — implements the generated
  `usagev1.UsageServiceServer`, pure wire<->usecase translation.
- `internal/adapter/eventbus/` — publishes `orca.usage.session.recorded` via
  `common/eventbus` (NATS JetStream). Called directly from the usecase in
  this scaffold; production wiring should go through the transactional
  outbox pattern from `architecture/05-data-architecture.md` instead — see
  "Known gaps" below.
- `migrations/0001_init.{up,down}.sql` — real DDL: `usage.sessions`,
  `usage.daily_rollups`, RLS policies, the `(tenant_id, request_id)`
  idempotency constraint.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, NATS connection (degrades gracefully if unavailable),
  gRPC server with the shared interceptor chain, health/readiness HTTP
  server, graceful shutdown on SIGTERM.

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
- **Event publishing bypasses the transactional outbox** — `RecordUsageSession`
  calls the event publisher directly after the DB write commits, not via an
  outbox row in the same transaction (see `architecture/05-data-architecture.md`).
  Acceptable for this pilot given usage-service's explicitly relaxed
  consistency SLO (see the service doc §8), but should not be copied as-is
  into a service where a missed publish matters (e.g. `notification-service`).
- **`common/secrets` (Vault) is not wired into this service's `main.go`** —
  `DATABASE_DSN` is read directly from the environment for local dev; the
  Vault-Agent-rendered-credentials-file path
  (`secrets.DatabaseCredentialsFromFile`) exists in `common/secrets` but
  isn't called here yet. Wire it before this service is deployed anywhere
  Vault is actually running.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in
  (see that package's doc comment).
- Full RPC surface per the service doc's exhaustive API section isn't
  implemented — only the 3 core operations. Extend `proto/orca/usage/v1/usage.proto`
  and this service's usecase/adapter layers together as more of the surface
  is needed.
