package tddagentclient

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics are this service's first Prometheus metrics; own registry, served at /metrics by main.
type metrics struct {
	reg          *prometheus.Registry
	calls        *prometheus.CounterVec
	pollDuration *prometheus.HistogramVec
	approvalWait *prometheus.HistogramVec
}

func newMetrics() *metrics {
	m := &metrics{
		reg: prometheus.NewRegistry(),
		calls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "workflow_tdd_agent_calls_total", Help: "tdd-agent tool calls from workflow steps, by tool and result",
		}, []string{"tool", "result"}),
		pollDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "workflow_tdd_agent_poll_duration_seconds", Help: "How long a tdd_agent.* step waited, by action and outcome",
			Buckets: []float64{1, 10, 60, 300, 900, 1800, 3600, 4 * 3600, 24 * 3600},
		}, []string{"action", "outcome"}),
		approvalWait: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "workflow_tdd_agent_approval_wait_seconds", Help: "Time from a gate awaiting approval to its decision in Orca",
			Buckets: []float64{60, 600, 3600, 4 * 3600, 24 * 3600, 3 * 24 * 3600},
		}, []string{"gate", "decision"}),
	}
	m.reg.MustRegister(m.calls, m.pollDuration, m.approvalWait)
	return m
}

var defaultMetrics = newMetrics()

// MetricsHandler serves the tdd_agent metrics in Prometheus text format.
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(defaultMetrics.reg, promhttp.HandlerOpts{})
}

func (m *metrics) call(tool string, err error) {
	result := "ok"
	var te *ToolError
	var be *BridgeError
	switch {
	case err == nil:
	case errors.As(err, &te):
		result = "tool_error"
	case errors.As(err, &be):
		result = "bridge_refused"
	case errors.Is(err, ErrTransport):
		result = "unreachable"
	default:
		result = "error"
	}
	m.calls.WithLabelValues(tool, result).Inc()
}

func (m *metrics) waited(action string, started time.Time, err error) {
	outcome := "completed"
	var sf *stepFailure
	if errors.As(err, &sf) {
		outcome = sf.Code
	} else if err != nil {
		outcome = "error"
	}
	m.pollDuration.WithLabelValues(action, outcome).Observe(time.Since(started).Seconds())
}

// observeGates logs each gate whose ledger status changed between two looks at a run.
func (m *metrics) observeGates(ctx context.Context, runID string, before, after runView) {
	prev := before.gateStatuses()
	for gate, status := range after.gateStatuses() {
		if prev[gate] != status {
			slog.InfoContext(ctx, "tdd_agent: gate status changed", slog.String("run_id", runID), slog.String("gate", gate),
				slog.String("from", prev[gate]), slog.String("to", status))
		}
	}
	if before.Run.Status != "" && before.Run.Status != after.Run.Status {
		slog.InfoContext(ctx, "tdd_agent: run status changed", slog.String("run_id", runID), slog.String("from", before.Run.Status), slog.String("to", after.Run.Status))
	}
}
