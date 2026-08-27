# Orca Backend — Go Rewrite (Target Design)

**Status:** 🚧 Proposed target architecture — no Go code exists yet. This is
a design/planning doc set, not a description of running systems (contrast
with [`specs/backend/`](../backend/), which documents the TypeScript system
actually in production).

**Scope:** A ground-up redesign of `backend/`'s responsibilities
(coordination/control-plane — see
[`backend/api/backend-agent-target-architecture.md`](../backend/api/backend-agent-target-architecture.md))
as a set of Go microservices, backed by PostgreSQL for data and HashiCorp
Vault for secrets, each organized internally with Clean Architecture, built
to production-ready/enterprise standards. `agent/` (the Dev Server Agent —
the execution plane) and `frontend/`/`desktop/`/`mobile/` are **out of
scope** here; the Go backend talks to the same execution-plane role the TS
backend does today (re-architecting that plane, if ever undertaken, is a
separate effort).

## Why this isn't a from-scratch design

Two pieces of prior art already answer the hardest questions and are treated
as binding inputs, not just references:

- **Service boundaries** — [`specs/backend/models/08-postgres-microservices-target-architecture.md`](../backend/models/08-postgres-microservices-target-architecture.md)
  and [ADR-021](../../docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md)
  already worked out a 13-schema, database-per-service decomposition for the
  *existing* TypeScript system's data plane. This doc set reuses those exact
  service boundaries for the data-owning Go services (renamed to Go-idiomatic
  service names where useful) and **extends** them with the
  execution/gateway-facing domains ADR-021 didn't cover (it's scoped to
  Postgres-backed data only) — git dispatch, SCM/issue-tracker integration,
  fleet/PTY connectivity, the public API edge. See
  [`architecture/02-microservices-decomposition.md`](./architecture/02-microservices-decomposition.md)
  for the full mapping and the reasoning for each addition.
- **What "no central vault" costs today** — [`specs/backend/models/05-credential-secret-stores.md`](../backend/models/05-credential-secret-stores.md)
  documents 5 independent, inconsistent secret-storage mechanisms in the TS
  system (per-user AES-256-GCM files, Electron `safeStorage`, OS Keychain,
  encrypted-blob-via-relay, in-memory-only). This is precisely the fragmentation
  this redesign's Vault integration ([`architecture/06-secrets-vault-architecture.md`](./architecture/06-secrets-vault-architecture.md))
  replaces with one consistent mechanism.

## Document map

| Doc | Content |
|-----|---------|
| **Architecture** | |
| [01-c4-hld.md](./architecture/01-c4-hld.md) | C4 Context/Container/Component for the target Go system |
| [02-microservices-decomposition.md](./architecture/02-microservices-decomposition.md) | Service boundaries, bounded contexts, the ADR-021 mapping + extensions, what's deliberately *not* a separate service |
| [03-clean-architecture-guidelines.md](./architecture/03-clean-architecture-guidelines.md) | Per-service package layout, dependency rule, layer contracts, shared-library policy |
| [04-tech-stack.md](./architecture/04-tech-stack.md) | Go version, frameworks, libraries, and the rationale for each choice |
| [05-data-architecture.md](./architecture/05-data-architecture.md) | PostgreSQL strategy: database-per-service, schema/migration conventions, multi-tenancy, cross-service consistency (outbox/saga) |
| [06-secrets-vault-architecture.md](./architecture/06-secrets-vault-architecture.md) | HashiCorp Vault integration: dynamic DB credentials, Transit encryption, KV, per-service auth, replaces all 5 legacy secret mechanisms |
| [07-security-architecture.md](./architecture/07-security-architecture.md) | AuthN/AuthZ, mTLS, RBAC/OPA, tenant isolation, audit |
| [08-inter-service-communication.md](./architecture/08-inter-service-communication.md) | gRPC (sync), NATS JetStream (async/events), the edge protocol, service discovery |
| [09-observability-reliability.md](./architecture/09-observability-reliability.md) | Logging, tracing, metrics, SLOs, resilience patterns |
| [10-deployment-infrastructure.md](./architecture/10-deployment-infrastructure.md) | Kubernetes, Helm, CI/CD, environments |
| **Services** | |
| [services/00-service-catalog.md](./services/00-service-catalog.md) | One-page index of all 17 services — owns-what, talks-to-whom, replaces-what |
| `services/<name>-service.md` × 16 | Per-service deep dive: responsibilities, API surface, domain model, DB schema, dependencies, NFRs, migration notes |
| **Migration** | |
| [migration/domain-capability-service-mapping.md](./migration/domain-capability-service-mapping.md) | Every TS RPC namespace / business capability → target Go service |
| [migration/ts-to-go-migration-strategy.md](./migration/ts-to-go-migration-strategy.md) | Strangler-fig rollout plan, phase sequencing, cutover/rollback criteria |
| **Standards** | |
| [standards/go-coding-standards.md](./standards/go-coding-standards.md) | Style, error handling, concurrency, package conventions |
| [standards/api-design-guidelines.md](./standards/api-design-guidelines.md) | REST + gRPC conventions, versioning, error model |
| [standards/testing-strategy.md](./standards/testing-strategy.md) | Unit/integration/contract/e2e, testcontainers, coverage bars |
| [standards/production-readiness-checklist.md](./standards/production-readiness-checklist.md) | Enterprise-level bar every service must clear before GA |

## How to read this if you're starting the rewrite

1. [`architecture/02-microservices-decomposition.md`](./architecture/02-microservices-decomposition.md) —
   what the services are and why.
2. [`architecture/03-clean-architecture-guidelines.md`](./architecture/03-clean-architecture-guidelines.md) +
   [`architecture/04-tech-stack.md`](./architecture/04-tech-stack.md) — how any one service is built internally.
3. [`architecture/05-data-architecture.md`](./architecture/05-data-architecture.md) +
   [`architecture/06-secrets-vault-architecture.md`](./architecture/06-secrets-vault-architecture.md) — how state and secrets are held.
4. [`migration/ts-to-go-migration-strategy.md`](./migration/ts-to-go-migration-strategy.md) — the order to build things in and how to cut traffic over without a big-bang rewrite.
5. The relevant `services/<name>-service.md` for whichever service you're implementing.

## Related (TypeScript system — source of truth for current behavior)

- [`specs/backend/api/business-capabilities.md`](../backend/api/business-capabilities.md) — every capability this redesign must preserve.
- [`specs/backend/api/backend-hld-c4.md`](../backend/api/backend-hld-c4.md) — C4 model of the system being replaced.
- [`specs/backend/api/backend-agent-target-architecture.md`](../backend/api/backend-agent-target-architecture.md) — the coordination/execution split this redesign inherits.
- [`specs/backend/models/`](../backend/models/) — full current-state data-layer survey (storage mechanisms, schema, credential stores).
- [ADR-021](../../docs/adrs/v2/ADR-021-unified-postgres-microservices-platform.md) — the service/schema decomposition this doc set builds on.
