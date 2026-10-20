package metrics

import (
	"regexp"
	"strings"
	"time"

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

	// Series of the Outcome Review runs (CR-OR-030). Ratios are computed in the dashboard from these counters; no threshold is set
	// before ten pilot Requests have been measured.
	ReviewContextLookups   = "request_review_context_lookups_total"
	ReviewRunSeconds       = "request_review_run_duration_seconds"
	ReviewTokens           = "request_review_tokens_total"
	ReviewCostError        = "request_review_cost_estimate_error_ratio"
	ReviewDiagrams         = "request_review_diagrams_total"
	ReviewDrawFailures     = "request_review_draw_failures_total"
	ReviewRegenerations    = "request_review_regenerations_total"
	ReviewFirstFactsSecs   = "request_review_time_to_first_facts_seconds"
	ReviewAgentItems       = "request_review_agent_items_total"
	ReviewDesignReviewedFr = "request_review_design_reviewed_ratio"
)

var (
	knownRegenReasons = map[string]bool{"looks_wrong": true, "kg_changed": true, "code_changed": true, "style_changed": true, "retry_failed": true, "other": true}
	knownRunKinds     = map[string]bool{"context": true, "draw": true}
	knownRunResults   = map[string]bool{"succeeded": true, "failed": true, "cancelled": true}
	reviewErrorCode   = regexp.MustCompile(`^REQUEST_REVIEW_[A-Z_]{1,40}$`)
)

func closed(set map[string]bool, v string) string {
	if set[v] {
		return v
	}
	return "other"
}

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

	ctxLookups   *prometheus.CounterVec
	runSeconds   *prometheus.HistogramVec
	tokens       *prometheus.CounterVec
	costError    *prometheus.HistogramVec
	diagrams     *prometheus.CounterVec
	drawFailures *prometheus.CounterVec
	regens       *prometheus.CounterVec
	firstFacts   prometheus.Histogram
	agentItems   *prometheus.CounterVec
	designRatio  prometheus.Histogram
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
	e.ctxLookups = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewContextLookups,
		Help: "Searches of the Outcome Review context cache, by result (hit, miss)."}, []string{"result"})
	e.runSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: ReviewRunSeconds, Buckets: []float64{5, 15, 30, 60, 120, 240, 480, 900},
		Help: "How long an Outcome Review agent run took, by kind and result."}, []string{"kind", "result"})
	e.tokens = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewTokens,
		Help: "Tokens an Outcome Review agent run reported, by kind and direction (input, output). A run that reports none adds nothing."}, []string{"kind", "direction"})
	e.costError = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: ReviewCostError, Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		Help: "How far the cost of a run was from the one predicted from the run before it: |actual - predicted| / predicted."}, []string{"kind"})
	e.diagrams = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewDiagrams,
		Help: "Diagrams a draw run handled, by result (drawn, failed). The failure ratio is failed over both."}, []string{"result"})
	e.drawFailures = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewDrawFailures,
		Help: "Diagrams that could not be drawn, by the code of the first failure of the run."}, []string{"error_code"})
	e.regens = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewRegenerations,
		Help: "Regenerations a person asked for, by reason."}, []string{"reason"})
	e.firstFacts = prometheus.NewHistogram(prometheus.HistogramOpts{Name: ReviewFirstFactsSecs, Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
		Help: "Time from asking for a review until the facts Orca measures were stored."})
	e.agentItems = prometheus.NewCounterVec(prometheus.CounterOpts{Name: ReviewAgentItems,
		Help: "Review items the agent wrote, by whether their evidence could be checked (verified yes or no)."}, []string{"verified"})
	e.designRatio = prometheus.NewHistogram(prometheus.HistogramOpts{Name: ReviewDesignReviewedFr, Buckets: []float64{0.1, 0.25, 0.5, 0.75, 0.9, 1},
		Help: "Share of the knowledge-graph elements a review read that a person had approved."})
	reg.MustRegister(e.runs, e.preflight, e.toolCalls, e.violations, e.drift, e.sync, e.lookups, e.stored, e.deduped,
		e.ctxLookups, e.runSeconds, e.tokens, e.costError, e.diagrams, e.drawFailures, e.regens, e.firstFacts, e.agentItems, e.designRatio)
	return e
}

var (
	_ usecase.EngineObserver         = (*Set)(nil)
	_ usecase.SyncObserver           = (*Set)(nil)
	_ usecase.ReviewArtifactObserver = (*Set)(nil)
	_ usecase.ReviewObserver         = (*Set)(nil)
)

func (s *Set) ReviewContextLookup(hit bool) {
	result := "miss"
	if hit {
		result = "hit"
	}
	s.engine.ctxLookups.WithLabelValues(result).Inc()
}

func (s *Set) ReviewRunEnded(kind, result string, d time.Duration) {
	s.engine.runSeconds.WithLabelValues(closed(knownRunKinds, kind), closed(knownRunResults, result)).Observe(d.Seconds())
}

func (s *Set) ReviewTokens(kind string, input, output int64) {
	kind = closed(knownRunKinds, kind)
	if input > 0 {
		s.engine.tokens.WithLabelValues(kind, "input").Add(float64(input))
	}
	if output > 0 {
		s.engine.tokens.WithLabelValues(kind, "output").Add(float64(output))
	}
}

func (s *Set) ReviewCostEstimateError(kind string, ratio float64) {
	s.engine.costError.WithLabelValues(closed(knownRunKinds, kind)).Observe(ratio)
}

func (s *Set) ReviewDrawn(drawn, failed int, code string) {
	if drawn > 0 {
		s.engine.diagrams.WithLabelValues("drawn").Add(float64(drawn))
	}
	if failed > 0 {
		s.engine.diagrams.WithLabelValues("failed").Add(float64(failed))
		if !reviewErrorCode.MatchString(code) {
			code = "other"
		}
		s.engine.drawFailures.WithLabelValues(code).Add(float64(failed))
	}
}

func (s *Set) ReviewRegenerated(reason string) {
	s.engine.regens.WithLabelValues(closed(knownRegenReasons, reason)).Inc()
}

func (s *Set) ReviewFirstFacts(d time.Duration) { s.engine.firstFacts.Observe(d.Seconds()) }

func (s *Set) ReviewItems(total, unverified int) {
	if unverified > total {
		unverified = total
	}
	if unverified > 0 {
		s.engine.agentItems.WithLabelValues("no").Add(float64(unverified))
	}
	if total-unverified > 0 {
		s.engine.agentItems.WithLabelValues("yes").Add(float64(total - unverified))
	}
}

func (s *Set) ReviewDesignReviewed(ratio float64) {
	if ratio >= 0 && ratio <= 1 {
		s.engine.designRatio.Observe(ratio)
	}
}

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
