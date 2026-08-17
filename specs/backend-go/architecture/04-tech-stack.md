# Tech Stack

Every choice below is the **default for all 17 services** unless a
service's own doc under [`services/`](../services/) explicitly overrides it
with a stated reason. Consistency across services is itself a
production-readiness property — an operator debugging any service at 3am
should find the same shape every time.

## Language & runtime

| Concern | Choice | Why |
|---------|--------|-----|
| Language | Go 1.23+ | Generics-stable, current LTS-equivalent tooling, native structured concurrency primitives |
| Module strategy | One Go module per service (`go.mod` per service directory), plus one shared `orca-go-common` module | Independent versioning/release per service — a breaking change in one service's dependencies can't force a rebuild of all 17 |
| Build | Multi-stage Docker builds, static binaries (`CGO_ENABLED=0`) | Minimal runtime images (`distroless` or `scratch`-based), no libc drift between build/runtime |

## API layer

| Concern | Choice | Why |
|---------|--------|-----|
| Internal service-to-service | gRPC + Protocol Buffers, schemas managed with `buf` (lint + breaking-change detection in CI) | Strong typing across service boundaries, codegen for client/server stubs, `buf breaking` catches incompatible proto changes before merge |
| Public edge API | REST/JSON via `grpc-gateway` (generates a REST facade from the same `.proto`, hosted in `api-gateway`) | One schema (the proto) defines both the internal gRPC contract and the public REST contract — no hand-maintained OpenAPI drifting from the real API |
| Real-time (terminal streams, agent status, notifications) | WebSocket at the edge (`api-gateway`), backed by gRPC server-streaming to the owning service | Matches the TS system's WS surface for `frontend`/`mobile` without changing client expectations |
| HTTP router (edge only — internal services are gRPC-only) | `chi` | Minimal, stdlib-`net/http`-compatible, no framework magic — matches "delivery layer should be thin" from Clean Architecture |
| API schema governance | Centralized `buf` module (`proto/orca/v1/...`), semantic versioning per package | See [`standards/api-design-guidelines.md`](../standards/api-design-guidelines.md) |

## Data access

| Concern | Choice | Why |
|---------|--------|-----|
| Driver | `pgx` (v5), used directly and via `sqlc`-generated code | Fastest, most complete Postgres driver in the Go ecosystem; native support for the Postgres wire protocol (no `database/sql` abstraction tax) |
| Query layer | `sqlc` — SQL-first, compile-time-checked Go code generation from `.sql` files | Keeps SQL visible and reviewable (vs. an ORM's generated queries), catches type mismatches at build time, no runtime reflection cost. **Exception**: `task-service` and `workflow-service` (DAG-heavy, recursive-query-heavy domains) may use `ent` instead where the graph-traversal codegen pays for itself — decide per-service, documented in that service's doc |
| Migrations | `golang-migrate`, one migration directory per service, numbered sequentially (mirrors the TS system's own `0001, 0002, …` convention for continuity) | Battle-tested, dialect-agnostic enough to keep a TiDB escape hatch open the way ADR-002/ADR-021 did for the TS system |
| Connection pooling | `pgxpool`, sized per service based on expected concurrency, credentials rotated via Vault's dynamic secrets engine (not a static pool-wide password) | See [`06-secrets-vault-architecture.md`](./06-secrets-vault-architecture.md) |

## Secrets

| Concern | Choice | Why |
|---------|--------|-----|
| Secret store | HashiCorp Vault (self-hosted or HCP Vault) | Single source of truth for all secret material — see [`06-secrets-vault-architecture.md`](./06-secrets-vault-architecture.md) for the full design |
| Service auth to Vault | Kubernetes auth method (service account token → Vault token) | No static Vault tokens baked into images/config; matches how the deployment target (K8s) already establishes service identity |
| Client library | `github.com/hashicorp/vault/api` + Vault Agent sidecar for auto-renewal | Avoids hand-rolled token-refresh logic in every service |

## Async messaging

| Concern | Choice | Why |
|---------|--------|-----|
| Event bus | NATS JetStream | Lower operational surface than Kafka at this system's scale (17 services, not hundreds), built-in at-least-once delivery + persistence, native Go client with no JVM dependency. **Enterprise-scale alternative**: Kafka, if the deployment already standardizes on it — the publish/consume port abstraction in each service's `adapter/eventbus/` makes this swappable without touching `usecase/` |
| Delivery guarantee pattern | Transactional outbox per service (write domain row + outbox row in the same Postgres transaction; a relay process publishes from the outbox) | Avoids the classic dual-write inconsistency (DB commit succeeds, event publish fails, or vice versa) without needing distributed transactions across service databases |

## Observability

| Concern | Choice | Why |
|---------|--------|-----|
| Tracing/metrics/logs | OpenTelemetry SDK (Go), OTLP export | Vendor-neutral; backend can be Jaeger/Tempo + Prometheus + Loki, or a managed equivalent, without code changes |
| Structured logging | `slog` (stdlib, Go 1.21+) with an OTel-correlation handler in `orca-go-common` | Stdlib-first, no third-party logging framework lock-in |
| Metrics | Prometheus client library, `/metrics` per service | Standard scrape target for the deployment's existing Prometheus/Grafana stack |
| Full design | [`09-observability-reliability.md`](./09-observability-reliability.md) | |

## Auth & policy

| Concern | Choice | Why |
|---------|--------|-----|
| Session auth (browser) | Signed session cookie issued by `auth-service`, validated at `api-gateway` | Preserves the TS system's existing browser UX (cookie-based, not a bearer token the SPA has to manage) |
| Service-to-service / mobile / CLI auth | Short-lived JWTs (RS256), issued by `auth-service`, validated by every service via a shared JWKS endpoint | Stateless validation at each service, no per-request call back to `auth-service` |
| Authorization | Open Policy Agent (OPA), policies evaluated at `api-gateway` and/or in-process via the Go OPA SDK for service-level checks | Replaces the TS system's fragmented `resolveUserPermissions()` / `TaskGrantService.resolvePermission()` split with one policy language and one place decisions are written down — see [`07-security-architecture.md`](./07-security-architecture.md) |

## Testing & CI

| Concern | Choice | Why |
|---------|--------|-----|
| Unit/assertions | stdlib `testing` + `testify/assert` | Minimal, widely understood |
| Integration | `testcontainers-go` (real Postgres, Vault, NATS containers per test run) | Tests the actual adapter code against the actual technology, not a mock of it |
| Contract testing | `buf breaking` (proto compatibility) + provider/consumer contract tests for the REST edge | Catches breaking API changes in CI, not in production |
| Load testing | k6 | Scriptable, CI-friendly, good gRPC support |
| Full policy | [`standards/testing-strategy.md`](../standards/testing-strategy.md) | |

## Deployment

| Concern | Choice | Why |
|---------|--------|-----|
| Orchestration | Kubernetes | Matches "production-ready/enterprise" scaling, rolling-update, and multi-environment requirements |
| Packaging | Helm charts, one chart per service + an umbrella chart | Standard for K8s-native enterprise deployments |
| CI/CD | GitHub Actions (matches the existing repo's CI provider) → GitOps deploy (ArgoCD) | Keeps deploy state declarative and auditable |
| Full design | [`10-deployment-infrastructure.md`](./10-deployment-infrastructure.md) | |

## Explicitly rejected options (and why)

| Option | Rejected in favor of | Reason |
|--------|------------------------|--------|
| A monolithic Go binary with internal packages instead of real services | 17 independently deployable services | The user's explicit requirement is microservices with independent production-readiness — a modular monolith doesn't meet "must be organized as microservices" |
| GORM (or another full ORM) as the default data layer | `sqlc` + `pgx` | ORMs hide the SQL a reviewer needs to see for a data layer at this scale, and generate less predictable query plans; `ent` is kept as an opt-in for graph-heavy services only |
| A hand-maintained OpenAPI spec alongside hand-maintained gRPC `.proto` | Single proto-derived schema via `grpc-gateway` | Avoids two schemas drifting apart — a known failure mode in systems that maintain both independently |
| Kafka as the default event bus | NATS JetStream as default, Kafka as a documented alternative | Kafka's operational overhead (ZooKeeper/KRaft, partition management, JVM ops) isn't justified at 17 services' event volume; revisit if event throughput requirements change |
