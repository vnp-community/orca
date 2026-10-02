package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpmetrics"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver/tools"
)

// newMCPMetrics builds the orca_mcp_* registry plus Go/process collectors.
func newMCPMetrics() *mcpmetrics.Metrics {
	m := mcpmetrics.New()
	m.Registry().MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// withMCPMetrics plugs the Prometheus recorder into the handler and counts
// every tools/call. It must be applied AFTER withMCPTools (it wraps the
// executor that option installed).
func withMCPMetrics(m *mcpmetrics.Metrics) func(*mcpserver.Deps) {
	return func(d *mcpserver.Deps) {
		d.Recorder = m
		var known func(string) bool
		if cat, ok := d.Catalog.(interface {
			Lookup(string) (*tools.ToolSpec, bool)
		}); ok {
			known = func(name string) bool { _, ok := cat.Lookup(name); return ok }
		}
		d.Executor = mcpmetrics.InstrumentExecutor(d.Executor, m, known)
	}
}

// healthAndMetricsMux serves /healthz and /readyz (unchanged) plus the
// Prometheus scrape endpoint on the internal health port, never on the
// public edge.
func healthAndMetricsMux(health http.Handler, m *mcpmetrics.Metrics) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", health)
	mux.Handle("/metrics", m.Handler())
	return mux
}
