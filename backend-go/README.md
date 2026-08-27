# orca-go

Go implementation of Orca's backend, per the target design in
[`specs/backend-go/`](../specs/backend-go/). **Status: scaffold** — every
service builds, vets, and tests green, and the architecturally-critical
pieces (Clean Architecture layering, database-per-service, Vault-mediated
secrets, one real cross-service gRPC call, one real event-driven consumer)
are genuinely implemented and tested. Most services' full business-logic
surface is not yet complete — see each service's own README for exactly
what's real vs. stubbed, and
[`docs/execution-plan.md`](./docs/execution-plan.md) for the ordered plan to
take this from scaffold to production.

## Layout

```
backend-go/
├── go.work                  # ties all 19 modules together for local dev
├── common/                  # orca-go-common — cross-cutting only, no business logic
├── proto/                   # buf module, orca.<service>.v1 packages, generated Go stubs in proto/gen/go/
├── services/<name>-service/ # one Go module per service, Clean Architecture layout (see any service's internal/ tree)
├── docker-compose.yml       # local dev: postgres (17 DBs), vault (dev mode), nats
├── Makefile                 # build/vet/test/lint/fmt/proto-gen/migrate — see `make help`
└── docs/execution-plan.md   # the detailed task breakdown for finishing this
```

## Quickstart

```sh
make dev-up          # postgres :5432, vault :8200, nats :4222
make build            # go build every module
make test             # go test every module (unit tests, no Docker needed)
make test-integration # go test -tags=integration every module (needs make dev-up first)
```

To run one service against the local stack, see that service's own README
(e.g. [`services/usage-service/README.md`](./services/usage-service/README.md))
for its exact env vars — every service follows the same `DATABASE_DSN` /
`GRPC_PORT` / `HTTP_PORT` convention from `common/config`.

## Why this exists

This is a from-scratch Go rewrite of `backend/`'s TypeScript coordination-plane
role — see [`specs/backend/api/backend-agent-target-architecture.md`](../specs/backend/api/backend-agent-target-architecture.md)
for what that role is, and [`specs/backend-go/README.md`](../specs/backend-go/README.md)
for the full target architecture (microservices decomposition, Clean
Architecture, PostgreSQL, Vault, production-readiness bar) this codebase
implements against. It does not replace anything running today — see
[`specs/backend-go/migration/ts-to-go-migration-strategy.md`](../specs/backend-go/migration/ts-to-go-migration-strategy.md)
for how a real cutover would be sequenced.

## What's genuinely real in this scaffold (not just structure)

- **`usage-service`** — fully-implemented reference service: real Postgres
  repository with RLS + write-idempotency, real NATS event publishing, real
  gRPC server, passes unit + integration tests.
- **`task-service`** — the BFS grant-resolution algorithm and cycle
  detector are real, pure, and covered by ~20 test cases.
- **`orchestration-service`** — the `KeyedSerializer` concurrency primitive
  is real and proven correct under `go test -race` with a genuinely racy
  workload; the atomic task-promotion transaction is real SQL.
- **`credential-broker-service`** — the Vault adapter (Transit/KV) is real,
  calling `common/secrets` for real; a compile-time test proves no
  secret-value field exists anywhere in its schema; audit-before-return
  ordering is tested.
- **`automation-service` → `workflow-service`** — the one fully-real
  cross-service gRPC call in this scaffold: `automation-service`'s `RunNow`
  genuinely dials and calls `workflow-service.ExecuteAdHocStep`.
- **`notification-service`** — the event-bus consumer and in-process
  broadcaster fan-out are real, with per-user/per-tenant isolation tested.
- **`api-gateway`** — the `usage-service` REST reverse-proxy and the
  `notification-service` WebSocket↔gRPC-stream bridge are real end-to-end.
- **`workflow-service`** — the Condition-step boolean evaluator and the
  Webhook-step executor (with SSRF IP-range blocking) are real.
- **`git-gateway-service`** — local git status/diff via `os/exec` against a
  real git binary is real.

## What's intentionally stubbed (and why that's honest, not incomplete work hidden)

Every service that depends on another not-yet-real service (most
cross-service calls point at `infra-fleet-service`'s Dev Server Agent relay,
which requires porting the existing TS wire protocol — a substantial
standalone effort explicitly out of scope for this pass) has a clearly
commented stub adapter satisfying the right interface, so the calling
service's own logic is real and tested against that interface even though
the live call isn't wired. Every such stub is called out in that service's
own README's "Known gaps" section — read those before assuming a route
works end-to-end. **Do not deploy any service in this repository to a
production environment without first closing the gaps listed in its
README and clearing [`specs/backend-go/standards/production-readiness-checklist.md`](../specs/backend-go/standards/production-readiness-checklist.md).**
