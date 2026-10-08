// Package metrics exposes issue-status-sync's Prometheus series on the health
// port's /metrics, following notification-service's adapter/metrics layout.
// No label carries a tenant, request or issue id.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Set implements usecase.SyncObserver and owns the scrape registry.
type Set struct {
	reg        *prometheus.Registry
	events     *prometheus.CounterVec
	ownedSkips *prometheus.CounterVec
	transition prometheus.Histogram
}

func New() *Set {
	s := &Set{
		reg: prometheus.NewRegistry(),
		events: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "orca_issuesync_request_events_total",
			Help: "Handled issue sync events by event and result.",
		}, []string{"event", "result"}),
		ownedSkips: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "orca_issuesync_skipped_request_owned_total",
			Help: "Worktree/PR events skipped because an open Request owns the issue.",
		}, []string{"source"}),
		transition: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "orca_issuesync_jira_transition_seconds",
			Help:    "Latency of a Request-driven Jira transition including the category check.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}),
	}
	s.reg.MustRegister(s.events, s.ownedSkips, s.transition,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return s
}

func (s *Set) ObserveRequestEvent(event, result string) {
	s.events.WithLabelValues(event, result).Inc()
}

func (s *Set) ObserveRequestOwnedSkip(source string) { s.ownedSkips.WithLabelValues(source).Inc() }

func (s *Set) ObserveJiraTransition(elapsed time.Duration) { s.transition.Observe(elapsed.Seconds()) }

func (s *Set) Handler() http.Handler { return promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}) }
