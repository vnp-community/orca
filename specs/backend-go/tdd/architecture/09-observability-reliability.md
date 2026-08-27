# Observability & Reliability

## The three pillars, uniformly across all 17 services

| Pillar | Tooling | Convention |
|--------|---------|------------|
| Logs | `slog` → OTLP → Loki (or equivalent) | Structured JSON only, never `fmt.Println`-style logs. Every log line carries `trace_id`, `tenant_id` (when applicable), `service`, `version` via the shared `orca-go-common` logging middleware |
| Traces | OpenTelemetry SDK → OTLP → Jaeger/Tempo | Every inbound gRPC/HTTP request starts a span; every outbound gRPC call, DB query, Vault call, and NATS publish is a child span — a single request from `api-gateway` through 4 services and a DB call is one trace, not 5 disconnected logs |
| Metrics | Prometheus client, `/metrics` endpoint per service | RED metrics (Rate, Errors, Duration) on every gRPC method by default via interceptor — no service hand-instruments this per-handler |

## SLOs

Each service's own doc states its specific SLOs; the system-wide floor
every service must meet before being considered production-ready:

- **Availability**: 99.9% for services on the synchronous request path from
  `api-gateway` (`auth-service`, `tenant-service`, `project-service`,
  `task-service`, `workflow-service`); 99.5% for services only reachable via
  async events or admin-path operations (`usage-service`, `annotation-service`).
- **Latency**: p99 < 300ms for gRPC calls that don't themselves fan out to
  the execution plane (which has its own, looser latency budget since it's
  bound by real SSH/network conditions to a dev server).
- **Error budget policy**: burn-rate alerting (multi-window, per Google SRE
  practice) rather than a flat "5xx rate > X%" threshold, to catch both fast
  brownouts and slow degradation.

## Health checks

- Every service exposes `/healthz` (liveness — process is up) and
  `/readyz` (readiness — can serve traffic: DB pool healthy, Vault lease
  valid, NATS connection established). Kubernetes wires these to
  liveness/readiness probes; `readyz` failing pulls a pod out of the
  Service's endpoint list without restarting it (transient DB blip
  shouldn't cause a restart storm).

## Resilience patterns

| Pattern | Where applied |
|---------|-----------------|
| Timeouts on every outbound call | Mandatory, see [`08-inter-service-communication.md`](./08-inter-service-communication.md) |
| Retries with exponential backoff + jitter | Idempotent gRPC calls only (reads, and writes designed to be idempotent via request IDs) — mutating calls that aren't naturally idempotent do not auto-retry at the transport level, to avoid duplicate side effects |
| Circuit breaking | Enforced at the service-mesh layer (mesh-level outlier detection), not hand-rolled per service — one less thing for application code to get wrong |
| Bulkheading | Per-service connection pool sizing (DB, downstream gRPC clients) prevents one slow dependency from exhausting a caller's entire resource pool |
| Graceful degradation | Explicitly designed per critical path — e.g. if `credential-broker-service` is briefly unavailable, `ai-provider-service` should serve cached provider *metadata* (non-secret) rather than fail the entire request; documented per-service where it applies, not assumed universally |
| Backpressure on WS/streaming | `api-gateway`'s WS↔gRPC-stream bridge applies bounded buffering with drop/slow-consumer policy, replacing the TS system's own `ws-outbound-backpressure-queue.ts` concept in Go form |

## Dashboards & alerting (minimum viable set per service)

1. RED metrics (request rate, error rate, duration) — auto-generated from
   the shared interceptor's metrics, one dashboard panel set per service by
   convention (not hand-built per service).
2. DB pool saturation + Vault lease renewal failures.
3. Event-bus consumer lag (JetStream consumer lag per subject) — a lagging
   consumer is a leading indicator of a downstream service in trouble
   before it shows up as a synchronous-path error.
4. Business-relevant custom metrics per service (e.g. `task-service`:
   active tasks by status; `workflow-service`: executions by state) —
   specified per service in its own doc where meaningful, not mandated
   generically here.

## Chaos/DR expectations for "enterprise level"

- Documented, tested restore procedure per service's database (backup
  policy owned by whichever team/process manages the Postgres fleet —
  referenced, not re-specified, in
  [`10-deployment-infrastructure.md`](./10-deployment-infrastructure.md)).
- Vault itself must be deployed HA (Raft integrated storage or equivalent)
  with its own DR/unseal procedure documented — a Vault outage is a
  system-wide outage (every service's DB credential lease eventually
  expires without it), so its own availability bar is the highest in the
  system, higher than any individual microservice's.
- Game-day / chaos testing (e.g. killing a service's pods mid-request,
  partitioning a service from Vault) is part of the
  [production-readiness checklist](../standards/production-readiness-checklist.md)
  gate before a service is declared GA, not an optional nice-to-have.
