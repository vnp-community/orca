# TASK-FLEET-03-08: Prometheus `fleetCollector` + `GET /health/metrics` + start poller goroutine

**From Solution:** SOL-FLEET-03
**Priority:** P2
**Service:** `infra-fleet-service` (metrics adapter + cmd/server)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/metrics/fleet_collector.go` (new), `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-FLEET-03-05, TASK-FLEET-03-06, TASK-FLEET-03-07
**Status:** [x] DONE — added `github.com/prometheus/client_golang` as a new dependency (zero prior Prometheus usage anywhere in this backend-go monorepo). Implemented FleetCollector verbatim to spec; added usecase.MetricsCollector port (Dependency Inversion — usecase never imports adapter/metrics) and threaded it through NewPollFleetHealth (nil-safe). Wired at bootstrap: NATS/outbox relay (mirrors usage-service's Connect/EnsureStream/Relay.Run pattern, graceful-degrades if NATS unreachable), HealthPublisher, webhook.Alerter (from FLEET_WEBHOOK_URL), FleetCollector registered on a dedicated prometheus.Registry, PollFleetHealth started via `go pollFleetHealthUC.Run(ctx, cfg.FleetPollInterval)`, `/health/metrics` mounted on the existing HTTP mux/port. Config gained FleetPollInterval (FLEET_POLL_INTERVAL_SEC, default 30, fail-safe on bad input) and FleetWebhookURL. All builds/vet/tests (incl. `-race`) pass across the whole service.

---

## Context

Exposes the poller's latest samples as Prometheus metrics without
re-querying Postgres per scrape — `PollFleetHealth.pollOne` writes into an
in-process cache after every poll, and the collector reads that cache. This
task also wires everything from TASK-FLEET-03-05/06/07 together at service
startup: constructs `PollFleetHealth`, starts its `Run` goroutine, and
mounts the metrics endpoint on the service's existing HTTP server.

## Changes to make

```go
// internal/adapter/metrics/fleet_collector.go
package metrics

var (
    statusDesc  = prometheus.NewDesc("orca_fleet_server_status", "Dev server health status (1=healthy)", []string{"server"}, nil)
    cpuDesc     = prometheus.NewDesc("orca_fleet_cpu_percent", "Dev server CPU percent", []string{"server"}, nil)
    ramDesc     = prometheus.NewDesc("orca_fleet_ram_percent", "Dev server RAM percent", []string{"server"}, nil)
    latencyDesc = prometheus.NewDesc("orca_fleet_ssh_latency_ms", "Dev server SSH/handshake latency ms", []string{"server"}, nil)
)

type cachedSample struct {
    Host string
    domain.DevServerHealth
}

type FleetCollector struct {
    cache *sync.Map // devServerID -> cachedSample
}

func NewFleetCollector() *FleetCollector { return &FleetCollector{cache: &sync.Map{}} }

// Update is called by PollFleetHealth.pollOne right after computing each
// sample — the collector never polls on its own.
func (c *FleetCollector) Update(devServerID, host string, sample domain.DevServerHealth) {
    c.cache.Store(devServerID, cachedSample{Host: host, DevServerHealth: sample})
}

func (c *FleetCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- statusDesc; ch <- cpuDesc; ch <- ramDesc; ch <- latencyDesc
}

func (c *FleetCollector) Collect(ch chan<- prometheus.Metric) {
    c.cache.Range(func(_, v any) bool {
        s := v.(cachedSample)
        ch <- prometheus.MustNewConstMetric(statusDesc, prometheus.GaugeValue, healthyAsFloat(s.Status), s.Host)
        ch <- prometheus.MustNewConstMetric(cpuDesc, prometheus.GaugeValue, s.CPUPercent, s.Host)
        ch <- prometheus.MustNewConstMetric(ramDesc, prometheus.GaugeValue, s.RAMPercent, s.Host)
        ch <- prometheus.MustNewConstMetric(latencyDesc, prometheus.GaugeValue, float64(s.LatencyMS), s.Host)
        return true
    })
}

func healthyAsFloat(s domain.HealthStatus) float64 {
    if s == domain.HealthStatusHealthy {
        return 1
    }
    return 0
}
```

`internal/usecase/poll_fleet_health.go`'s `pollOne` gains a call to
`uc.collector.Update(ds.ID, ds.Host, sample)` right after computing
`sample` (add a `collector *metrics.FleetCollector` field to
`PollFleetHealth`, threaded through `NewPollFleetHealth`).

`cmd/server/main.go`: construct the registry/collector, register it,
mount `mux.Handle("/health/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))`
on the **existing** HTTP server (same port as `/` liveness and `/agent` WS —
not a new port), then construct `PollFleetHealth` with all its dependencies
and start it: `go pollFleetHealth.Run(ctx, cfg.FleetPollInterval)`, stopped
on the same `ctx` the SIGTERM handler cancels. `FleetPollInterval` is loaded
from `FLEET_POLL_INTERVAL_SEC` (default `30`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/metrics/... -run TestFleetCollector -v
```

Expected: `Collect` emits the four named metrics with the `server` label
matching BL-FLEET-03's exact metric names. End-to-end (manual or
integration): one `pollOnce` cycle against 3 dev servers (1 healthy, 1
degraded-by-CPU, 1 unreachable) results in 3 `infra.fleet_health` rows with
correct `status`, and `fleet.health.checkAll`
(`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go:487-519`)
returns real, non-empty data for the first time.
