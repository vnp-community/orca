package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// Series of the engines (CR-REQ-026). Labels are small fixed sets: the engine, the run kind, a result, a violation or
// blocker code, and the name of a knowledge tool. None carries what a tool read.
const (
	RunsTotal              = "request_engine_runs_total"
	PreflightFailuresTotal = "request_engine_preflight_failures_total"
	KnowledgeToolCalls     = "request_knowledge_tool_calls_total"
	SpecViolationsTotal    = "request_spec_check_violations_total"
	TasksDriftTotal        = "request_engine_drift_total"
	TasksSyncTotal         = "request_engine_sync_total"

	// Series of the Outcome Review drawings (CR-OR-011). The hit ratio is lookups{result="hit"} over all lookups.
	ReviewArtifactLookups = "request_review_artifact_lookups_total"
	ReviewArtifactBytes   = "request_review_artifact_bytes_stored_total"
	ReviewArtifactDeduped = "request_review_artifact_blobs_deduped_total"
)

var knownArtifactResults = map[string]bool{"hit": true, "miss": true, "failed": true, "corrupt": true}

// knownKnowledgeTools is what a read-only tdd-agent server offers. A tool name outside it, from a stream Orca does not
// control, is counted as "other" so the label set cannot grow without bound.
var knownKnowledgeTools = map[string]bool{
	"impact_get": true, "kg_get_element": true, "kg_neighbors": true, "kg_query": true, "kg_resolve_scope": true, "kg_search": true,
	"kg_snapshots": true, "metrics_get": true, "readiness_get": true, "run_get": true, "run_list": true, "source_get": true, "trace_get": true,
	"changeset_propose": true,
}

// engineSeries are the counters of the engines; they register with the Set's registry.
type engineSeries struct {
	runs       *prometheus.CounterVec
	preflight  *prometheus.CounterVec
	toolCalls  *prometheus.CounterVec
	violations *prometheus.CounterVec
	drift      prometheus.Counter
	sync       *prometheus.CounterVec
	lookups    *prometheus.CounterVec
	stored     prometheus.Counter
	deduped    prometheus.Counter
}

func newEngineSeries(reg *prometheus.Registry) engineSeries {
	e := engineSeries{
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{Name: RunsTotal,
			Help: "Agent and engine runs that ended, by engine (native, openspec), run kind and result (succeeded, failed)."}, []string{"engine", "kind", "result"}),
		preflight: prometheus.NewCounterVec(prometheus.CounterOpts{Name: PreflightFailuresTotal,
			Help: "Blockers found by the engine readiness check, by code."}, []string{"code"}),
		toolCalls: prometheus.NewCounterVec(prometheus.CounterOpts{Name: KnowledgeToolCalls,
			Help: "Calls an agent made to the knowledge graph's tools, by tool name."}, []string{"tool"}),
		violations: prometheus.NewCounterVec(prometheus.CounterOpts{Name: SpecViolationsTotal,
			Help: "Structure problems found in an authored OpenSpec change, by code."}, []string{"code"}),
	}
	e.drift = prometheus.NewCounter(prometheus.CounterOpts{Name: TasksDriftTotal,
		Help: "Plan regions of tasks.md found edited by hand and restored."})
	e.sync = prometheus.NewCounterVec(prometheus.CounterOpts{Name: TasksSyncTotal,
		Help: "tasks.md sync and archive steps that ended, by step (tasks, archive) and result (done, failed, gave_up)."}, []string{"step", "result"})
	e.lookups = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewArtifactLookups,
		Help: "Lookups of an Outcome Review drawing, by result (hit, miss, failed, corrupt)."}, []string{"result"})
	e.stored = prometheus.NewCounter(prometheus.CounterOpts{Name: ReviewArtifactBytes, Help: "Bytes of Outcome Review drawings stored."})
	e.deduped = prometheus.NewCounter(prometheus.CounterOpts{Name: ReviewArtifactDeduped, Help: "Outcome Review drawings whose content was already stored."})
	reg.MustRegister(e.runs, e.preflight, e.toolCalls, e.violations, e.drift, e.sync, e.lookups, e.stored, e.deduped)
	return e
}

var (
	_ usecase.EngineObserver         = (*Set)(nil)
	_ usecase.SyncObserver           = (*Set)(nil)
	_ usecase.ReviewArtifactObserver = (*Set)(nil)
)

func (s *Set) ArtifactLookup(result string) {
	if !knownArtifactResults[result] {
		result = "other"
	}
	s.engine.lookups.WithLabelValues(result).Inc()
}

func (s *Set) ArtifactStored(bytes int, deduped bool) {
	if bytes > 0 {
		s.engine.stored.Add(float64(bytes))
	}
	if deduped {
		s.engine.deduped.Inc()
	}
}

func (s *Set) RunFinished(engine, kind, result string) {
	s.engine.runs.WithLabelValues(engine, kind, result).Inc()
}

func (s *Set) PreflightFailed(code string) { s.engine.preflight.WithLabelValues(code).Inc() }

func (s *Set) KnowledgeToolCalls(stats domain.RunStats) {
	for name, n := range stats.ToolCalls {
		short, ok := strings.CutPrefix(name, domain.KnowledgeToolPrefix)
		if !ok || n <= 0 {
			continue
		}
		if !knownKnowledgeTools[short] {
			short = "other"
		}
		s.engine.toolCalls.WithLabelValues(short).Add(float64(n))
	}
}

func (s *Set) SpecViolation(code string) { s.engine.violations.WithLabelValues(code).Inc() }

func (s *Set) TasksDrift() { s.engine.drift.Inc() }

func (s *Set) SyncFinished(step, result string) { s.engine.sync.WithLabelValues(step, result).Inc() }
