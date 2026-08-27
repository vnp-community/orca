# Production Readiness Checklist

Every service must clear every item below before its
[migration cutover](../migration/ts-to-go-migration-strategy.md) is
considered final (dual-write torn down, TS path retired for that
namespace) — and before any *new* capability (not a TS port) ships to
`production` for the first time. This is the "enterprise level" bar the
user's request asked for made concrete and checkable, not an aspiration.

## Architecture & code

- [ ] Follows the standard Clean Architecture package layout
      ([`../architecture/03-clean-architecture-guidelines.md`](../architecture/03-clean-architecture-guidelines.md))
      — `domain/` has zero framework imports, verified by a CI import-graph
      check, not just code review.
- [ ] Own Go module, own `go.mod`, builds and tests independently of every
      other service.
- [ ] No direct database access to another service's database (verified —
      the service's Vault-issued DB role literally cannot connect to any
      other service's database).
- [ ] `golangci-lint` clean, `govulncheck` clean.

## Data

- [ ] Own PostgreSQL database (or isolated logical database per
      [`../architecture/05-data-architecture.md`](../architecture/05-data-architecture.md)'s
      per-service decision).
- [ ] Every migration has a tested, working `down` migration.
- [ ] Every tenant-scoped table has `tenant_id NOT NULL`, application-layer
      filtering verified in every repository method, RLS enabled as
      secondary defense.
- [ ] Automated backups configured, restore procedure tested at least once
      (not just "backups are running").

## Secrets

- [ ] Zero secrets in config files, environment variables, or container
      images — all secret material sourced from Vault at runtime via the
      Vault Agent sidecar pattern
      ([`../architecture/06-secrets-vault-architecture.md`](../architecture/06-secrets-vault-architecture.md)).
- [ ] Vault policy for this service scoped to exactly what it needs (its
      own DB credential path, and — only for the services that need it —
      the specific `credential-broker-service`-mediated paths), reviewed,
      not a broad wildcard grant.
- [ ] Dynamic DB credential rotation verified working (kill the current
      lease, confirm the service re-fetches without a restart).

## API

- [ ] `.proto` reviewed and merged through `buf breaking` gate.
- [ ] Every mutating RPC that could reasonably be retried supports
      idempotency via `request_id`.
- [ ] Every RPC has an explicit deadline enforced on the client side by
      every known caller.
- [ ] Input validation via `protovalidate` on every field that needs it —
      not left to `usecase/` layer to discover a malformed request.

## Reliability

- [ ] `/healthz` and `/readyz` implemented and wired to K8s probes.
- [ ] Resource requests/limits set (not defaulted/unset) — right-sized from
      load-test data, not guessed.
- [ ] `PodDisruptionBudget` configured — service tolerates a node
      drain/rolling upgrade without capacity loss.
- [ ] Graceful shutdown verified (SIGTERM drains in-flight requests within
      the configured termination grace period).
- [ ] Chaos scenario run at least once (dependency-unavailable, pod-kill)
      with documented, acceptable behavior — see
      [`../architecture/09-observability-reliability.md`](../architecture/09-observability-reliability.md).

## Observability

- [ ] Structured logs with `trace_id`/`tenant_id` correlation.
- [ ] OpenTelemetry tracing wired for every inbound/outbound call.
- [ ] RED metrics dashboard exists (auto-provisioned via the shared
      convention, not hand-built).
- [ ] At least one service-specific business metric dashboarded if the
      service has meaningful domain state to observe.
- [ ] Alerting configured for SLO burn-rate, not just raw error-rate
      thresholds.
- [ ] On-call runbook exists: what each alert means, first diagnostic steps,
      who to escalate to.

## Security

- [ ] mTLS verified between this service and its callers/dependencies (mesh
      policy applied, not just "the mesh is installed cluster-wide").
- [ ] `NetworkPolicy` default-deny with explicit allows matching the
      dependency graph — verified by attempting (and failing) a connection
      from a service that shouldn't be able to reach this one.
- [ ] OPA policy coverage reviewed for every authorization decision this
      service makes — no ad hoc `if role == "admin"` checks outside the
      policy bundle.
- [ ] Dependency/container image scans clean (or documented accepted-risk
      exceptions with an owner and a re-review date, not silently ignored).
- [ ] Audit events emitted for every security-relevant action in this
      service's domain.

## Testing

- [ ] Unit test coverage ≥ 80% on `domain/` + `usecase/`.
- [ ] Integration tests cover every `adapter/postgres/`,
      `adapter/vault/`, `adapter/eventbus/` method against real
      infrastructure (`testcontainers-go`), not mocked.
- [ ] Load test run against `staging` at production-comparable traffic,
      SLOs from
      [`../architecture/09-observability-reliability.md`](../architecture/09-observability-reliability.md)
      met.

## Migration-specific (only for services replacing a TS namespace)

- [ ] Dual-write comparison run for the full soak window with zero
      unexplained divergence.
- [ ] Data backfill script tested against `staging`-scale data, rollback
      path for the backfill itself documented.
- [ ] Rollback-to-TS proxy flag tested (flip it, confirm traffic serves
      correctly from the legacy path, flip back).

## Documentation

- [ ] Service's own doc under [`../services/`](../services/) is current —
      API surface, data model, dependencies, NFRs all match what actually
      shipped, not what was originally planned.
- [ ] Runbook linked from the on-call rotation's tooling, not just sitting
      in the repo.
