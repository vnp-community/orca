# Observability & Reliability

## The three pillars, uniformly across all 18 services

| Pillar | Tooling | Convention |
|--------|---------|------------|
| Logs | `slog` → OTLP → Loki (or equivalent) | Structured JSON only, never `fmt.Println`-style logs. Every log line carries `trace_id`, `tenant_id` (when applicable), `service`, `version` via the shared `orca-go-common` logging middleware |
| Traces | OpenTelemetry SDK → OTLP → Jaeger/Tempo | Every inbound gRPC/HTTP request starts a span; every outbound gRPC call, DB query, Vault call, and NATS publish is a child span — a single request from `api-gateway` through 4 services and a DB call is one trace, not 5 disconnected logs |
| Metrics | Prometheus client, `/metrics` endpoint per service | RED metrics (Rate, Errors, Duration) on every gRPC method by default via interceptor — no service hand-instruments this per-handler |

### Naming prefixes and scrape path (T8)

- Domain-specific Prometheus metrics use a per-domain prefix; MCP uses `orca_mcp_*`, and MCP env vars use `MCP_*` (loaded via `common/config`).
- **Scrape path: `/metrics` = health mux + promhttp.** The service's HTTP health port (api-gateway `HTTP_PORT`, mcp-service's HTTP port) serves `/healthz`, `/readyz` and `/metrics` from one mux (`promhttp.HandlerFor` over a private registry, not the default registerer). In api-gateway `/metrics` is therefore NOT on the public edge listener. The ServiceMonitor/annotation that scrapes it belongs to the deployment chart.
- MCP metric families (api-gateway `adapter/mcpmetrics` and mcp-service): `orca_mcp_requests_total`, `orca_mcp_tool_calls_total`, `orca_mcp_tool_duration_seconds`, `orca_mcp_sessions_active`, `orca_mcp_sessions_closed_total`, `orca_mcp_sessions_count_errors_total`, `orca_mcp_sse_streams_active`, `orca_mcp_sse_events_dropped_total`, `orca_mcp_resume_total`, `orca_mcp_approvals_total`, `orca_mcp_policy_denials_total`, `orca_mcp_auth_failures_total`, `orca_mcp_principal_resolve_total`, `orca_mcp_session_identity_mismatch_total`, `orca_mcp_rate_limited_total`, `orca_mcp_requests_cancelled_total`. OTel spans: `mcp.request`, `mcp.policy`, `mcp.dispatch` (no tool arguments, tokens, secrets or cookies as span attributes).

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
