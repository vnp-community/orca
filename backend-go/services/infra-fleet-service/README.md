# infra-fleet-service

The system of record for "which host owns this `connectionId`, and how do I
reach it" — see
[`specs/backend-go/services/infra-fleet-service.md`](../../../specs/backend-go/services/infra-fleet-service.md)
for the full design and
[`usage-service`](../usage-service/) for the reference package layout this
service follows.

`ResolveConnection` is the core call: `git-gateway-service` invokes it on
every `git.*` dispatch to decide local-exec vs. relay, `project-service`
invokes it to validate a dev-server binding, and every other
`connectionId`-bound feature in the system (terminal sessions, filesystem
ops, port scans) reduces to the same call.

## What's implemented

- `internal/domain/` — `DevServer` (host + `ConnectionMode` enum:
  `relay-ssh` / `relay-websocket` / `direct-websocket`), `SshTarget`
  (host/user + a Vault SSH-role pointer, never raw key material),
  `DevServerHealth` (CPU/RAM/disk/latency snapshot) — invariant-enforcing
  constructors, pure unit tests.
- `internal/usecase/` — `RegisterDevServer`, `ResolveConnection` (the
  important one — tested for both its found and not-found branches),
  `CreateSshTarget`, `GetFleetHealth`, `ScanWorkspacePorts`, each tested
  against in-memory fakes for `DevServerRepository` / `SshTargetRepository`
  / `ConnectionResolver` / `FleetHealthPort` / `DevServerAgentClient`, no
  real Postgres or agent connection needed.
- `internal/adapter/postgres/` — real `pgx`-backed repository implementing
  all four usecase ports off one `Repository` type (mirrors usage-service's
  single-`Repository` shape). Hand-written SQL, same rationale as
  usage-service (`sqlc` codegen is the eventual target, not wired yet).
- `internal/adapter/devserveragent/` — **stub**, see "Known gaps" below.
- `internal/adapter/grpc/` — implements the generated
  `infrafleetv1.InfraFleetServiceServer`, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — real DDL: `infra.dev_servers`,
  `infra.ssh_targets`, `infra.fleet_health`, RLS policies on the two
  tenant-scoped tables.
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM. Wires the
  devserveragent stub in cleanly (see below) so the dependency graph's shape
  is correct even though that adapter does nothing useful yet.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/infra-fleet-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/infra-fleet-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/infra?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
go test ./...                 # unit tests (domain/, usecase/) — no external deps
go test -tags=integration ./internal/adapter/postgres/...   # requires Docker (testcontainers-go)
```

## Known gaps / follow-ups (tracked, not silently skipped)

- **The Dev Server Agent relay protocol client is stubbed — the biggest gap
  in this service.** `internal/adapter/devserveragent/client.go` implements
  the `usecase.DevServerAgentClient` port, but every method (`Exec`,
  `Health`) returns `ErrNotImplemented`. The real implementation needs a Go
  port of the **existing** TS wire protocol (Option A per
  [`08-inter-service-communication.md`](../../../specs/backend-go/architecture/08-inter-service-communication.md)):
  the 13-byte-framed JSON-RPC header (`[TYPE u8][SEQ u32BE][ACK u32BE][LEN u32BE]`)
  shared by `agent-wire.ts` and `protocol.ts`, three connection-mode
  transports (`direct-websocket`/`relay-websocket`/`relay-ssh`), and the
  Stack A/Part B method-surface split documented in
  [`specs/agent/api/README.md`](../../../specs/agent/api/README.md) and
  [`specs/agent/api/gaps-and-findings.md`](../../../specs/agent/api/gaps-and-findings.md).
  This is a substantial standalone effort, deliberately out of scope for
  this scaffold — see the package doc comment in `client.go` for the full
  breakdown (`wire/`, `directws/`, `relayws/`, `relayssh/`, `handshake.go`,
  `methods.go` per the design doc §6). Because of this stub, `ScanWorkspacePorts`
  correctly *routes* (always relays when a `connectionId` resolves — closing
  TS Gap 7, see below) but the relay call itself fails until this adapter is
  implemented for real.
- **SSH credential handling via Vault's SSH secrets engine is not wired.**
  `ssh_targets.vault_ssh_role` is stored as a pointer per the security
  invariant in the design doc §9 (no raw key material, ever), but nothing in
  this scaffold actually calls Vault's SSH secrets engine to issue a
  short-lived certificate from that role, or falls back to KV v2 for
  static-key targets. `common/secrets` is only wired for the DB-credential
  bootstrap path (and not even that — see the next gap), not the SSH
  cert-issuance path.
- **`common/secrets` (Vault) is not wired into `main.go` at all** —
  `DATABASE_DSN` is read directly from the environment for local dev, same
  as usage-service's equivalent gap. Wire
  `secrets.DatabaseCredentialsFromFile` before this service is deployed
  anywhere Vault is actually running.
- **No `connections`, `port_forwards`, `provider_registry_entries`, or
  `terminal_sessions` tables.** This scaffold's schema is the proto-scoped
  subset (`dev_servers`, `ssh_targets`, `fleet_health`) needed by the 5 RPCs
  `infrafleet.proto` currently defines. `ResolveConnection` therefore
  resolves a `connectionId` directly against `dev_servers.id` within the
  caller's tenant scope (i.e. `connectionId == dev_server_id` today) instead
  of joining through a separate `connections` table recording live
  transport/session state — see the doc comment on
  `postgres.Repository.ResolveConnection`. The full design doc's connection
  lifecycle (`EstablishConnection`/`TeardownConnection`/`CheckConnectionHealth`),
  port forwarding, provider-registry audit table, and terminal session
  routing RPCs are not implemented — only what `infrafleet.proto` exposes
  today: `RegisterDevServer`, `ResolveConnection`, `CreateSshTarget`,
  `GetFleetHealth`, `ScanWorkspacePorts`.
- **No fleet health polling job.** `infra.fleet_health` stores the latest
  sample per dev server and `GetFleetHealth` reads it, but nothing writes
  to it — the 30s-cadence poller described in the design doc §8
  (leader-election-per-target via Postgres advisory locks) isn't
  implemented.
- **No NATS eventbus wiring.** The design doc §7 calls for
  `connection.established` / `connection.lost` /
  `dev_server.health_degraded` events via NATS JetStream; this scaffold's
  `internal/config` and `cmd/server/main.go` have no NATS dependency at all
  (unlike usage-service, which does publish events) since none of the 5
  implemented usecases need to publish anything yet.
- **`ScanWorkspacePorts` never performs a local port scan.** Per the design
  doc §10, this closes TS Gap 7 for the *relay* path — a bound
  `connectionId` always relays to the agent, never silently short-circuits
  to an empty result (see `usecase.ScanWorkspacePorts`'s tests for the
  regression coverage). The *local* path (no `connectionId`, or one that
  doesn't resolve) intentionally returns an empty slice rather than actually
  scanning localhost's open ports — this service's contract is routing the
  scan, not executing it, and a real local-scan implementation was out of
  scope here.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in, same as
  usage-service.
