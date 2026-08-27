// Package metrics implements usecase.MetricsCollector as a Prometheus
// collector — exposes the poller's latest samples without re-querying
// Postgres per scrape: usecase.PollFleetHealth.pollOne writes into an
// in-process cache after every poll (see FleetCollector.Update), and
// Collect reads that cache when Prometheus scrapes GET /health/metrics
// (see cmd/server/main.go's mounting of promhttp.HandlerFor).
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// Metric names/labels match BL-FLEET-03-health-monitoring.md's Prometheus
// Metrics section verbatim.
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

// FleetCollector implements usecase.MetricsCollector and
// prometheus.Collector — one instance per process, registered once at
// bootstrap (cmd/server/main.go) and threaded into
// usecase.NewPollFleetHealth as the collector dependency.
type FleetCollector struct {
	cache *sync.Map // devServerID -> cachedSample
}

func NewFleetCollector() *FleetCollector { return &FleetCollector{cache: &sync.Map{}} }

// Update implements usecase.MetricsCollector — called by
// PollFleetHealth.pollOne right after computing each sample. The collector
// never polls on its own.
func (c *FleetCollector) Update(devServerID, host string, sample domain.DevServerHealth) {
	c.cache.Store(devServerID, cachedSample{Host: host, DevServerHealth: sample})
}

func (c *FleetCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- statusDesc
	ch <- cpuDesc
	ch <- ramDesc
	ch <- latencyDesc
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
