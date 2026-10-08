// Package metrics exposes request-service's Prometheus series (CR-REQ-024 section 2.9). Most counters are
// derived from outbox events so the use cases stay unaware of metrics; none of the labels carries a tenant,
// request or approval id, or any text typed by a user.
package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// Set owns the scrape registry. It is a usecase.OutboxTap (event-derived series) and a usecase.RequestObserver.
type Set struct {
	reg             *prometheus.Registry
	created         *prometheus.CounterVec
	transitions     *prometheus.CounterVec
	classification  *prometheus.CounterVec
	confidence      prometheus.Histogram
	approvals       *prometheus.CounterVec
	approvalWait    *prometheus.HistogramVec
	approvalsPend   *prometheus.GaugeVec
	stuck           *prometheus.GaugeVec
	returned        *prometheus.CounterVec
	aiGeneration    *prometheus.HistogramVec
	taskOutcome     *prometheus.CounterVec
	outboxPending   prometheus.Gauge
	pendingSubjects map[string]bool
}

var (
	_ usecase.OutboxTap       = (*Set)(nil)
	_ usecase.RequestObserver = (*Set)(nil)
)

func New() *Set {
	s := &Set{
		reg:             prometheus.NewRegistry(),
		pendingSubjects: map[string]bool{},
		created: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "orca_request_created_total",
			Help: "Requests created, by source provider."}, []string{"source_provider"}),
		transitions: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "orca_request_transitions_total",
			Help: "Request status transitions."}, []string{"type", "from", "to"}),
		classification: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "orca_request_classification_total",
			Help: "Classification results: confirmed_as_proposed, changed or failed."}, []string{"outcome"}),
		confidence: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "orca_request_classification_confidence",
			Help: "AI confidence of successful classification proposals.", Buckets: []float64{0.1, 0.3, 0.5, 0.7, 0.8, 0.9, 0.95, 1}}),
		approvals: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "orca_request_approvals_total",
			Help: "Approvals closed, by subject type and outcome."}, []string{"subject_type", "outcome"}),
		approvalWait: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "orca_request_approval_wait_seconds",
			Help: "Time from approval request to its decision.", Buckets: []float64{60, 300, 900, 3600, 14400, 86400, 259200}}, []string{"subject_type"}),
		approvalsPend: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "orca_request_approvals_pending",
			Help: "Pending approvals, sampled from the database."}, []string{"subject_type"}),
		stuck: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "orca_request_stuck",
			Help: "Requests in a status for longer than its threshold, sampled from the database."}, []string{"status"}),
		returned: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "orca_request_returned_total",
			Help: "Requests returned to the backlog, by the stage they came from."}, []string{"returned_from_stage"}),
		aiGeneration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "orca_request_ai_generation_seconds",
			Help: "AI generation latency by kind and outcome.", Buckets: []float64{1, 5, 15, 30, 60, 120, 300}}, []string{"kind", "outcome"}),
		taskOutcome: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "orca_request_task_outcome_total",
			Help: "Task results reported back to request-service."}, []string{"outcome"}),
		outboxPending: prometheus.NewGauge(prometheus.GaugeOpts{Name: "orca_request_outbox_pending",
			Help: "Outbox rows not yet published, sampled from the database."}),
	}
	s.reg.MustRegister(s.created, s.transitions, s.classification, s.confidence, s.approvals, s.approvalWait, s.approvalsPend,
		s.stuck, s.returned, s.aiGeneration, s.taskOutcome, s.outboxPending,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return s
}

func (s *Set) Handler() http.Handler { return promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}) }

// label keeps a series per value bounded: an empty value (a Request not classified yet) becomes "none".
func label(v string) string {
	if v == "" {
		return "none"
	}
	return v
}

type eventFields struct {
	SourceProvider string  `json:"source_provider"`
	Type           string  `json:"type"`
	TypeSource     string  `json:"type_source"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	Stage          string  `json:"stage"`
	Failed         bool    `json:"failed"`
	Confidence     float64 `json:"confidence"`
	Decision       string  `json:"decision"`
	SubjectType    string  `json:"subject_type"`
	WaitSeconds    float64 `json:"wait_seconds"`
}

// OnEvent counts the committed event; unreadable payloads are ignored (metrics never fail a decision).
func (s *Set) OnEvent(_ context.Context, ev domain.OutboxEvent) {
	var f eventFields
	if err := json.Unmarshal(ev.Payload, &f); err != nil {
		return
	}
	switch ev.Subject {
	case domain.SubjectRequestCreated:
		s.created.WithLabelValues(label(f.SourceProvider)).Inc()
	case domain.SubjectRequestStatusChanged:
		s.transitions.WithLabelValues(label(f.Type), label(f.From), label(f.To)).Inc()
	case domain.SubjectRequestClassified:
		if f.Failed {
			s.classification.WithLabelValues("failed").Inc()
			return
		}
		s.confidence.Observe(f.Confidence)
	case domain.SubjectRequestTypeConfirmed:
		outcome := "changed"
		if f.TypeSource == string(domain.TypeSourceAI) {
			outcome = "confirmed_as_proposed"
		}
		s.classification.WithLabelValues(outcome).Inc()
	case domain.SubjectRequestTypeChanged:
		s.classification.WithLabelValues("changed").Inc()
	case domain.SubjectRequestReturned:
		s.returned.WithLabelValues(label(f.Stage)).Inc()
	case domain.SubjectApprovalDecided:
		s.approvals.WithLabelValues(label(f.SubjectType), label(f.Decision)).Inc()
		if f.WaitSeconds > 0 {
			s.approvalWait.WithLabelValues(label(f.SubjectType)).Observe(f.WaitSeconds)
		}
	}
}

func (s *Set) ObserveAIGeneration(kind, outcome string, elapsed time.Duration) {
	s.aiGeneration.WithLabelValues(label(kind), label(outcome)).Observe(elapsed.Seconds())
}

func (s *Set) ObserveTaskOutcome(outcome string) { s.taskOutcome.WithLabelValues(label(outcome)).Inc() }
