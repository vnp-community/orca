# Testing Strategy

## Test pyramid, mapped to the Clean Architecture layers

```
        ▲  fewer, slower
        │
   E2E  │  cross-service scenarios, full docker-compose stack, CI only (not per-PR for every service)
        │
Contract│  buf breaking + REST provider/consumer contracts, runs per-service in CI
        │
Integr. │  adapter/postgres, adapter/vault, adapter/eventbus — testcontainers-go, real deps
        │
  Unit  │  domain/ (pure) + usecase/ (fakes) — the bulk of the suite, fast, no I/O
        ▼  more, faster
```

This mirrors the layering in
[`../architecture/03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md)
directly — the architecture is what makes most of the pyramid fast: only
the `adapter/` layer needs real infrastructure to test meaningfully.

## Unit tests (`domain/`, `usecase/`)

- `domain/`: pure functions/methods, no test doubles needed at all — a
  `Task` entity's invariant-enforcing constructor either rejects a cyclic
  dependency or it doesn't; test it directly.
- `usecase/`: test against hand-written fakes or `mockgen`-generated mocks
  of the ports (`TaskRepository`, `EventPublisher`) defined in that same
  layer. No real Postgres, no real NATS, no network — these tests should
  run in milliseconds and make up the majority of each service's suite.
- Coverage bar: 80% for `domain/` + `usecase/` combined, enforced in CI,
  per service. (Deliberately not a system-wide blanket "90% everywhere"
  target — `adapter/` code is often thin translation where high coverage
  numbers are less meaningful than for business logic; see integration
  tests below for how that layer is actually validated.)

## Integration tests (`adapter/*`)

- `adapter/postgres/`: real Postgres via `testcontainers-go`, migrations
  run against it before tests, verifying the repository implementation
  actually satisfies the port contract defined in `usecase/` — not testing
  SQL syntax in isolation, testing that "given this repository call, the
  right rows exist afterward."
- `adapter/vault/`: real Vault dev-mode container via `testcontainers-go`,
  verifying secret write/read/rotate round-trips correctly and that a
  revoked lease is actually rejected.
- `adapter/eventbus/`: real NATS JetStream container, verifying publish →
  consume round-trips and that the outbox relay actually drains.
- `adapter/external/` (SCM/PM clients): recorded HTTP fixtures (`go-vcr`-style)
  rather than live calls to GitHub/GitLab/Jira/Linear in CI — live-service
  smoke tests run separately, on a schedule, not on every PR (avoiding
  flakiness from rate limits/third-party downtime blocking merges).

## Contract tests

- `buf breaking` on every `.proto` change (see
  [`api-design-guidelines.md`](./api-design-guidelines.md)) — this is the
  primary contract test for internal gRPC APIs.
- REST facade: consumer-driven contract tests (Pact or equivalent) between
  `frontend`'s API client expectations and `api-gateway`'s actual responses,
  run in CI on both repos so a backend change that would break the frontend
  fails before merge, not after deploy.

## End-to-end tests

- A small, deliberately limited set of cross-service scenarios (e.g. "create
  a task → AI-decompose → execute a subtask → workflow step completes →
  notification fires") run against a full `docker-compose` stack
  (mirroring the local-dev setup in
  [`../architecture/10-deployment-infrastructure.md`](../architecture/10-deployment-infrastructure.md)).
  These are expensive and slow by nature — kept to the handful of scenarios
  that actually span service boundaries in ways unit/integration tests
  structurally can't cover, not a substitute for thorough per-service
  testing.

## Load testing

- k6 scripts per service, targeting the stated SLOs in
  [`../architecture/09-observability-reliability.md`](../architecture/09-observability-reliability.md).
  Run against `staging` before any traffic-relevant change (new service
  cutover, a scaling-sensitive feature) is promoted to `production` — part
  of the [production-readiness checklist](./production-readiness-checklist.md).

## Chaos/resilience testing

- Pod-kill, network-partition, and dependency-unavailable scenarios
  (Vault unreachable, a downstream gRPC dependency timing out) run
  per-service before GA, verifying the resilience patterns in
  [`../architecture/09-observability-reliability.md`](../architecture/09-observability-reliability.md)
  actually degrade gracefully rather than cascading failure — not
  hypothetical, actually exercised.

## Migration-specific testing (during the TS→Go cutover)

- Dual-write comparison tests (see
  [`../migration/ts-to-go-migration-strategy.md`](../migration/ts-to-go-migration-strategy.md))
  — automated diffing of TS vs. Go responses for the same input during the
  soak period, not manual spot-checking.
- Data-migration backfill scripts get their own test suite run against a
  copy of production-shaped data in `staging` before touching real data.
