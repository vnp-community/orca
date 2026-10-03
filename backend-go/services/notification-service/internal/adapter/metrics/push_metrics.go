// Package metrics exposes notification-service's Prometheus series on the
// health port's /metrics, following mcp-service's adapter/metrics layout.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Set implements usecase.PushObserver and owns the scrape registry.
type Set struct {
	reg        *prometheus.Registry
	deliveries *prometheus.CounterVec
	latency    *prometheus.HistogramVec
	vapidProv  *prometheus.CounterVec
}

func New() *Set {
	s := &Set{
		reg: prometheus.NewRegistry(),
		deliveries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "orca_notification_push_deliveries_total",
			Help: "Push deliveries by channel and outcome (sent, expired, failed).",
		}, []string{"channel", "outcome"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "orca_notification_push_delivery_seconds",
			Help:    "Per-subscription push delivery latency including VAPID signing.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"channel"}),
		vapidProv: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "orca_notification_vapid_provision_total",
			Help: "Automatic per-tenant VAPID key provisioning by outcome (created, existing, forbidden, error).",
		}, []string{"outcome"}),
	}
	s.reg.MustRegister(s.deliveries, s.latency, s.vapidProv,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return s
}

func (s *Set) ObserveDelivery(channel, outcome string, elapsed time.Duration) {
	s.deliveries.WithLabelValues(channel, outcome).Inc()
	s.latency.WithLabelValues(channel).Observe(elapsed.Seconds())
}

func (s *Set) ObserveVapidProvision(outcome string) { s.vapidProv.WithLabelValues(outcome).Inc() }

func (s *Set) Handler() http.Handler { return promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}) }
