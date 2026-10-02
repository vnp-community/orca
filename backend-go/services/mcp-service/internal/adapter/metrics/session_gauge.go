// Package metrics exposes mcp-service's Prometheus metrics. Only series whose
// source of truth is this service live here; request, tool and auth series are
// recorded by the gateway edge (BE-MCP-SOL-015 section C).
package metrics

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SessionCounter counts non-closed MCP sessions across all tenants.
type SessionCounter interface {
	CountOpenSessions(ctx context.Context) (int64, error)
}

// Set owns the registry served at /metrics.
type Set struct {
	reg            *prometheus.Registry
	sessionsActive prometheus.Gauge
	countErrors    prometheus.Counter
	counter        SessionCounter
	log            *slog.Logger
}

// New builds the registry (Go and process collectors included). counter may be
// nil, in which case the sessions gauge stays 0 and is never refreshed.
func New(counter SessionCounter, log *slog.Logger) *Set {
	s := &Set{reg: prometheus.NewRegistry(), counter: counter, log: log}
	s.sessionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orca_mcp_sessions_active", Help: "MCP sessions in a non-closed state across all tenants and replicas (sampled from the database)."})
	s.countErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "orca_mcp_sessions_count_errors_total", Help: "Failed samples of the active-session count."})
	s.reg.MustRegister(s.sessionsActive, s.countErrors,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return s
}

// Handler serves the scrape endpoint.
func (s *Set) Handler() http.Handler { return promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}) }

// Refresh samples the session count once.
func (s *Set) Refresh(ctx context.Context) {
	if s.counter == nil {
		return
	}
	n, err := s.counter.CountOpenSessions(ctx)
	if err != nil {
		s.countErrors.Inc()
		s.log.WarnContext(ctx, "counting active mcp sessions failed", slog.Any("error", err))
		return
	}
	s.sessionsActive.Set(float64(n))
}

// Run refreshes every interval (30s in production) until ctx ends. A scrape
// never touches the database: it reads the last sample.
func (s *Set) Run(ctx context.Context, every time.Duration) {
	s.Refresh(ctx)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Refresh(ctx)
		}
	}
}
