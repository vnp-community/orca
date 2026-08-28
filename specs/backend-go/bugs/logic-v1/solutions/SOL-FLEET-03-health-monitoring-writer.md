# SOL-FLEET-03: Fleet-health poller (the missing `infra.fleet_health` writer), Prometheus metrics, webhook alerts

**Resolves:** [BUG-FLEET-03](../BUG-FLEET-03-health-monitoring-partial.md)
**Service:** `infra-fleet-service` only — no other service or `agent/` change needed (see rationale)
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/domain/dev_server_health.go` (add `Status`)
- `backend-go/services/infra-fleet-service/migrations/0008_fleet_health_status.up.sql` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/poll_fleet_health.go` (new — the writer)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend `FleetHealthPort` with a write side + advisory-lock port)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go` (`UpsertFleetHealth`, `TryLockDevServerForHealthPoll`)
- `backend-go/services/infra-fleet-service/internal/adapter/eventbus/health_publisher.go` (new — `dev_server.health_degraded`)
- `backend-go/services/infra-fleet-service/internal/adapter/webhook/alerter.go` (new — `FLEET_WEBHOOK_URL` POST)
- `backend-go/services/infra-fleet-service/internal/adapter/metrics/fleet_collector.go` (new — Prometheus)
- `backend-go/services/infra-fleet-service/cmd/server/main.go` (start the poller goroutine, mount `/health/metrics`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (`DevServerHealth.status`)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

The bug report's own instruction is explicit: design the missing writer, not
just "add a writer" — this section grounds each piece of that job in
`infra-fleet-service.md` §8 rather than inventing a cadence/ownership model
from scratch.

- **Cadence and per-target ownership are already specified**: "Fleet health
  polling cadence: 30s per dev server, matching TS's `FleetHealthMonitor`.
  Polling fans out across replicas — each dev server polled by exactly one
  replica per interval (leader-election-per-target or a distributed lock via
  Postgres advisory locks, not every replica polling every target)"
  (`specs/backend-go/tdd/services/infra-fleet-service.md:470-474`). This
  solution takes the doc's own suggested mechanism — Postgres advisory
  locks — since this service already depends on Postgres and adding a
  leader-election library would be a new, heavier dependency for the same
  guarantee.
- **Note the cadence discrepancy, resolved in favor of the TDD**:
  BL-FLEET-03 (the TS-legacy spec) says 60s / `FLEET_POLL_INTERVAL_SEC`
  (`docs/logic/fleet/BL-FLEET-03-health-monitoring.md:26`), while
  `infra-fleet-service.md` says 30s. Per this task's framing ("designs...
  grounded in the target architecture spec"), the TDD's 30s is the design
  default; `FLEET_POLL_INTERVAL_SEC` (the BL spec's env var name) is kept as
  the override knob so an operator can dial in either cadence without a code
  change — the name is preserved for continuity, the default value is not.
- **This service is the correct owner of the SSH-exec-shaped health check**:
  §2's bounded-context table classifies "Fleet health polling (CPU/RAM/disk/
  latency)" as owned by `infra-fleet-service`, explicitly because "SSH exec
  is a coordination-layer act — establishing/monitoring the connection
  itself, not doing dev-work on it"
  (`infra-fleet-service.md:69`) — the poller belongs in this service's
  `usecase/`, not a separate monitoring service and not `agent/`.
- **Events**: §7 already lists `dev_server.health_degraded` as a real,
  intended NATS JetStream event this service publishes via "the
  transactional outbox pattern" (`infra-fleet-service.md:389-391`) — this
  solution's status-change emission is not a new integration point, it's
  filling in a documented one that had no writer to trigger it, matching
  `common/outbox`'s existing package precedent already used by
  `usage-service`/`issue-tracking-service` (confirmed present in this repo,
  see References) and `tenant-service`'s existing
  `internal/adapter/eventbus/publisher.go` naming convention this solution's
  `internal/adapter/eventbus/health_publisher.go` follows.

**A genuine spec-vs-reality gap this solution resolves without new `agent/`
work**: BL-FLEET-03's step 2 is "Relay health check: `GET
http://127.0.0.1:<relayPort>/health` (via SSH tunnel)"
(`BL-FLEET-03-health-monitoring.md:29`) — but per SOL-FLEET-02's finding, no
such HTTP endpoint exists anywhere in `agent/`. This solution substitutes
the **already-real** `devserveragent.Client.Health` handshake check
(`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:276-283`,
"uniform across all three modes") as the reachability signal — functionally
equivalent ("is the relay up and answering"), just not delivered as an
HTTP GET through a tunnel. Metrics collection (CPU/RAM/disk, BL-FLEET-03's
step 3) is likewise satisfiable **without new `agent/` work**: `shell.exec`
is a real, already-implemented generic command-execution JSON-RPC method
reachable uniformly on all three of this service's connection modes (all of
them ultimately dispatch into `agent/src/relay/agent-rpc-dispatch.ts`'s Part
A handler set, confirmed via direct inspection of `agent/src` — see
References), so `devserveragent.Client.Exec(ctx, devServer, "shell.exec",
{"script": "..."})` running `cat /proc/stat; free -b; df -P ~` is a real,
available primitive today. **No `agent/` change is required to close this
bug** — a meaningful contrast with BUG-FLEET-02/04's daemon-model gap.

---

## Design — domain / migration

```go
// internal/domain/dev_server_health.go (extended)
type HealthStatus string

const (
    HealthStatusHealthy     HealthStatus = "healthy"
    HealthStatusDegraded    HealthStatus = "degraded"
    HealthStatusUnhealthy   HealthStatus = "unhealthy"
    HealthStatusUnreachable HealthStatus = "unreachable"
)

// ComputeHealthStatus applies BL-FLEET-03's threshold table verbatim
// (BL-FLEET-03-health-monitoring.md:16-21) — a pure function so its
// thresholds are unit-testable without any I/O.
func ComputeHealthStatus(reachable, relayReachable bool, cpuPercent, ramPercent float64) HealthStatus {
    if !reachable {
        return HealthStatusUnreachable // SSH/agent connect itself failed
    }
    if !relayReachable {
        return HealthStatusUnhealthy // SSH/session alive, agent handshake not
    }
    if cpuPercent > 80 || ramPercent > 85 {
        return HealthStatusDegraded
    }
    return HealthStatusHealthy
}

type DevServerHealth struct {
    DevServerID string
    Reachable   bool
    CPUPercent, RAMPercent, DiskPercent float64
    LatencyMS   int64
    Status      HealthStatus // new
}
```

```sql
-- migrations/0008_fleet_health_status.up.sql
ALTER TABLE infra.fleet_health
  ADD COLUMN status TEXT NOT NULL DEFAULT 'unreachable'
    CHECK (status IN ('healthy','degraded','unhealthy','unreachable'));
```

## Design — the writer job (usecase/poll_fleet_health.go)

**What runs it**: a single background goroutine started in
`cmd/server/main.go` alongside the existing gRPC/HTTP server goroutines
(`backend-go/services/infra-fleet-service/cmd/server/main.go:226-243`'s
pattern), stopped on the same `ctx` the service's SIGTERM handler already
cancels (`main.go:53`). Not a separate binary/CronJob — this keeps the
poller's dependencies (the already-constructed `agentClient`, `repo`) in
the same process, avoiding a second Postgres/Vault connection pool for a
job that's small and fast.

**How often**: `time.NewTicker(cfg.FleetPollInterval)` — `FleetPollInterval`
loaded from `FLEET_POLL_INTERVAL_SEC` (default `30`, see rationale above).
Each tick fans out one bounded-concurrency pass over every dev server this
replica can claim (not every dev server every replica — see locking below).

**What it writes**: one row per dev server in `infra.fleet_health` — an
upsert on the existing `dev_server_id` primary key (`migrations/0001_init.up.sql:59-67`),
i.e. latest-sample-only, matching this service's existing "no sampled-over-time
history" scope note (`0001_init.up.sql:9-12`); `checked_at` bumps to `now()`
on every write.

```go
// internal/usecase/poll_fleet_health.go
type PollFleetHealth struct {
    devServers DevServerRepository
    health     FleetHealthWriter // extended FleetHealthPort, see ports.go below
    agent      DevServerAgentClient
    lock       PollLockPort // Postgres advisory lock, see ports.go below
    events     HealthEventPublisher
    webhook    WebhookAlerter
    logger     *slog.Logger
}

// Run ticks every interval until ctx is cancelled — called once from
// main.go as `go pollFleetHealth.Run(ctx, interval)`.
func (uc *PollFleetHealth) Run(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            uc.pollOnce(ctx)
        }
    }
}

func (uc *PollFleetHealth) pollOnce(ctx context.Context) {
    servers, err := uc.devServers.ListAllForPolling(ctx) // cross-tenant — the poller is not a per-request, per-tenant call
    if err != nil {
        uc.logger.ErrorContext(ctx, "poll_fleet_health: listing dev servers failed", slog.Any("error", err))
        return
    }
    var wg sync.WaitGroup
    sem := make(chan struct{}, pollConcurrency) // default 10 — bounded fan-out, not one goroutine per server unbounded
    for _, ds := range servers {
        wg.Add(1)
        sem <- struct{}{}
        go func(ds domain.DevServer) {
            defer wg.Done()
            defer func() { <-sem }()
            // Postgres advisory lock keyed on dev_server_id — "each dev
            // server polled by exactly one replica per interval"
            // (infra-fleet-service.md:472-474). Non-blocking: a replica
            // that loses the race simply skips this server this tick
            // rather than queueing, since the next tick is 30s away
            // regardless.
            locked, unlock, err := uc.lock.TryLock(ctx, ds.ID)
            if err != nil || !locked {
                return
            }
            defer unlock()
            uc.pollOne(ctx, ds)
        }(ds)
    }
    wg.Wait()
}

func (uc *PollFleetHealth) pollOne(ctx context.Context, ds domain.DevServer) {
    pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // explicit deadline distinct from the 5s intra-cluster default, per infra-fleet-service.md §8
    defer cancel()

    start := time.Now()
    reachable, err := uc.agent.Health(pollCtx, ds) // real handshake check, all 3 modes — see rationale
    latencyMS := time.Since(start).Milliseconds()

    var cpu, ram, disk float64
    relayReachable := reachable
    if reachable {
        result, execErr := uc.agent.Exec(pollCtx, ds, "shell.exec", map[string]any{
            "script":    fleetMetricsScript, // "cat /proc/stat; echo ---; free -b; echo ---; df -P ~"
            "timeoutMs": 5000,
        })
        if execErr != nil {
            relayReachable = false // handshake alive but shell.exec failed — treat as unhealthy, not unreachable
        } else {
            cpu, ram, disk = parseFleetMetrics(result) // pure parsing, unit-tested independently
        }
    }

    status := domain.ComputeHealthStatus(reachable, relayReachable, cpu, ram)
    sample, _ := domain.NewDevServerHealth(ds.ID, reachable, cpu, ram, disk, latencyMS)
    sample.Status = status

    previous, hadPrevious, _ := uc.health.GetPrevious(ctx, ds.ID)
    if err := uc.health.UpsertFleetHealth(ctx, sample); err != nil {
        uc.logger.ErrorContext(ctx, "poll_fleet_health: upsert failed", slog.String("devServerId", ds.ID), slog.Any("error", err))
        return
    }

    if hadPrevious && previous.Status != status {
        uc.events.PublishStatusChange(ctx, ds, previous.Status, status) // dev_server.health_degraded via outbox
        uc.webhook.NotifyStatusChange(ctx, ds, previous.Status, status, sample) // best-effort, errors logged not propagated
    }
}
```

**Concurrency default** (`pollConcurrency = 10`): bounded so one tick's fan-out
doesn't open unbounded simultaneous SSH/WS sessions against the fleet —
mirrors SOL-FLEET-02's `BulkProvisionFleet` concurrency cap reasoning.

## Design — ports (extended)

```go
// internal/usecase/ports.go
type FleetHealthPort interface {
    GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error) // unchanged, read side
}

// FleetHealthWriter is the write side PollFleetHealth needs — split from
// FleetHealthPort the same way ConnectionRepository/ConnectionResolver are
// already two narrow ports over one Repository (ports.go:53-100's existing
// convention).
type FleetHealthWriter interface {
    UpsertFleetHealth(ctx context.Context, sample domain.DevServerHealth) error
    GetPrevious(ctx context.Context, devServerID string) (sample domain.DevServerHealth, found bool, err error)
}

// PollLockPort wraps a Postgres session-level advisory lock keyed by a
// hash of devServerID — TryLock is non-blocking (pg_try_advisory_lock, not
// pg_advisory_lock), matching pollOnce's "skip this tick, not queue" design.
type PollLockPort interface {
    TryLock(ctx context.Context, devServerID string) (locked bool, unlock func(), err error)
}

// DevServerRepository gains one more read method:
//   ListAllForPolling(ctx context.Context) ([]domain.DevServer, error)
// — cross-tenant by design (the poller is not answering one tenant's
// request), unlike every other DevServerRepository method's tenantID
// parameter.

type HealthEventPublisher interface {
    PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus)
}
type WebhookAlerter interface {
    NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth)
}
```

`adapter/postgres`'s `TryLock` implementation:
`SELECT pg_try_advisory_lock(hashtext($1))` on a dedicated held connection
(not the pool — advisory locks are session-scoped, so this needs
`pool.Acquire` and holds the `*pgxpool.Conn` until `unlock()` releases it
via `pg_advisory_unlock` then returns the conn to the pool).

## Design — Prometheus metrics (`GET /health/metrics`)

```go
// internal/adapter/metrics/fleet_collector.go
// fleetCollector implements prometheus.Collector, reading an in-process
// cache PollFleetHealth updates after every pollOne — NOT re-querying
// Postgres on every scrape, since Prometheus scrape intervals (typically
// 15-30s) shouldn't add load proportional to scrape frequency on top of
// the poller's own Postgres writes.
type fleetCollector struct {
    cache *sync.Map // devServerID -> cachedSample{host string; domain.DevServerHealth}
}

func (c *fleetCollector) Collect(ch chan<- prometheus.Metric) {
    c.cache.Range(func(_, v any) bool {
        s := v.(cachedSample)
        ch <- prometheus.MustNewConstMetric(statusDesc, prometheus.GaugeValue, healthyAsFloat(s.Status), s.Host)
        ch <- prometheus.MustNewConstMetric(cpuDesc, prometheus.GaugeValue, s.CPUPercent, s.Host)
        ch <- prometheus.MustNewConstMetric(ramDesc, prometheus.GaugeValue, s.RAMPercent, s.Host)
        ch <- prometheus.MustNewConstMetric(latencyDesc, prometheus.GaugeValue, float64(s.LatencyMS), s.Host)
        return true
    })
}
```

Metric names/labels match BL-FLEET-03's exact spec
(`orca_fleet_server_status{server=...}`, `orca_fleet_cpu_percent{server=...}`,
`orca_fleet_ram_percent{server=...}`, `orca_fleet_ssh_latency_ms{server=...}`,
`BL-FLEET-03-health-monitoring.md:44-56`). `PollFleetHealth.pollOne` writes
into the same cache right after computing `sample`, so the collector never
needs its own polling loop. `cmd/server/main.go` mounts
`mux.Handle("/health/metrics", promhttp.HandlerFor(registry, ...))` on the
**existing** HTTP server (same port as `/`'s liveness probe and `/agent`'s
WS endpoint, `main.go:214-222`) — not a new port, following that file's
existing "share this service's HTTP server" convention
(`internal/adapter/agentwsserver`'s doc comment cited at `main.go:199-204`
for the same reasoning applied to a different endpoint).

## Design — webhook alerts

```go
// internal/adapter/webhook/alerter.go
type Alerter struct {
    url    string // FLEET_WEBHOOK_URL, empty = disabled
    client *http.Client
}

func (a *Alerter) NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth) {
    if a.url == "" {
        return
    }
    body, _ := json.Marshal(map[string]any{
        "event": "fleet.server.status_change", "server": ds.Host,
        "from": from, "to": to, "timestamp": time.Now().UTC().Format(time.RFC3339),
        "metrics": map[string]any{"cpu": sample.CPUPercent, "ram": sample.RAMPercent},
    }) // exact shape from BL-FLEET-03-health-monitoring.md:60-70
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp, err := a.client.Do(req) // best-effort: log-and-continue on any failure, never blocks pollOne
    if err != nil {
        slog.Default().WarnContext(ctx, "webhook: status-change delivery failed", slog.Any("error", err))
        return
    }
    _ = resp.Body.Close()
}
```

Only the service-wide `FLEET_WEBHOOK_URL` env var is implemented in this
pass — BL-FLEET-03 also mentions "per-server trong fleet YAML"
(`BL-FLEET-03-health-monitoring.md:72`) as an alternative config surface;
that requires `SshTarget`/`DevServer` to carry a per-server webhook override
field, which this solution does not add (no evidence any current caller
needs per-server granularity yet) — flagged as a natural, small follow-up
rather than bundled in here.

---

## Test plan

- `domain/dev_server_health_test.go` — `ComputeHealthStatus` table test over
  BL-FLEET-03's exact threshold matrix (CPU 79/81, RAM 84/86, unreachable,
  handshake-alive-but-relay-down).
- `usecase/poll_fleet_health_test.go` — fake `DevServerAgentClient`/
  `FleetHealthWriter`/`PollLockPort`/`HealthEventPublisher`/`WebhookAlerter`:
  a healthy server writes one `UpsertFleetHealth` call with `status=healthy`;
  a status transition (`healthy`→`degraded` via fake `GetPrevious`) triggers
  exactly one `PublishStatusChange` and one `NotifyStatusChange` call; no
  transition triggers neither; a `TryLock` returning `locked=false` results
  in **zero** calls to `agent.Health`/`agent.Exec` for that server (assert
  on the fake) — the concurrency-safety regression guard.
- `parseFleetMetrics` unit tests — real `/proc/stat`/`free -b`/`df -P`
  sample output (fixture strings) parse into expected CPU/RAM/disk percents;
  malformed output degrades to zero values, not a panic.
- `adapter/postgres/health_lock_test.go` (integration, testcontainers) — two
  concurrent `TryLock` calls for the same `devServerID` from two separate
  connections: exactly one succeeds; after `unlock()`, a subsequent `TryLock`
  succeeds again.
- `adapter/metrics/fleet_collector_test.go` — `Collect` emits the four named
  metrics with the `server` label matching BL-FLEET-03's exact metric names.
- `adapter/webhook/alerter_test.go` — `httptest.Server` receives the exact
  JSON shape from BL-FLEET-03; `FLEET_WEBHOOK_URL=""` sends nothing; a
  webhook server returning 500 does not propagate an error to the caller.
- End-to-end (testcontainers Postgres + a fake agent transport): one
  `pollOnce` cycle against 3 dev servers (1 healthy, 1 degraded-by-CPU, 1
  unreachable) results in 3 `infra.fleet_health` rows with correct `status`,
  and `fleet.health.checkAll` (the existing read path,
  `backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:487-519`)
  returns real, non-empty data for the first time — the literal regression
  test for this bug's headline symptom.

## References

- `docs/logic/fleet/BL-FLEET-03-health-monitoring.md` — status thresholds,
  poll flow, Prometheus metric names, webhook payload shape
- `specs/backend-go/bugs/logic-v1/BUG-FLEET-03-health-monitoring-partial.md`
- `specs/backend-go/tdd/services/infra-fleet-service.md:69` (§2, SSH exec as
  coordination-layer act), `:389-391` (§7, `dev_server.health_degraded`
  event already anticipated), `:446-484` (§8, cadence/locking/deadline
  rules this solution implements directly)
- `backend-go/services/infra-fleet-service/internal/usecase/get_fleet_health.go:11-14`,
  `internal/usecase/ports.go:102-108`,
  `internal/adapter/postgres/repository.go:329-359`,
  `migrations/0001_init.up.sql:52-69` — the existing read-only path and its
  own doc comments confirming no writer, all cited verbatim in the bug report
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:242-283`
  (`Exec`/`Health`, uniform across all 3 connection modes — the mechanism
  this solution's metrics collection and reachability check reuse)
- `specs/agent/api/agent-rpc-catalog-runtime.md:169` (`shell.exec`'s real
  params/response shape, confirmed via direct `agent/src/relay/
  fs-agent-extensions.ts:535-586` inspection) — no `agent/` change needed
- `backend-go/common/outbox/outbox.go`, `backend-go/common/eventbus/eventbus.go`
  — existing transactional-outbox/event-bus packages this solution's
  `adapter/eventbus/health_publisher.go` reuses, following
  `backend-go/services/tenant-service/internal/adapter/eventbus/publisher.go`'s
  naming precedent
- `backend-go/services/infra-fleet-service/cmd/server/main.go:192-222`
  (existing HTTP mux/health-server wiring this solution extends)
- [SOL-FLEET-02](./SOL-FLEET-02-bulk-provisioning.md) — shared finding that
  no HTTP `/health` endpoint or daemon model exists in `agent/`
