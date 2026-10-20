package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestEngineMetrics_ServeTheFourSeriesWithTheRightLabels(t *testing.T) {
	s := New()
	s.RunFinished("openspec", "plan", "succeeded")
	s.RunFinished("openspec", "plan", "failed")
	s.RunFinished("native", "solution", "succeeded")
	s.PreflightFailed("REQUEST_ENGINE_OPENSPEC_MISSING")
	s.SpecViolation("TASKSMD_DEPENDENCY_UNKNOWN")
	s.KnowledgeToolCalls(domain.RunStats{ToolCalls: map[string]int{
		domain.KnowledgeToolPrefix + "kg_search": 4, domain.KnowledgeToolPrefix + "kg_get_element": 2, "Read": 9, "mcp__other__thing": 3,
	}})
	body := scrape(t, s)
	for _, want := range []string{
		`request_engine_runs_total{engine="openspec",kind="plan",result="succeeded"} 1`,
		`request_engine_runs_total{engine="openspec",kind="plan",result="failed"} 1`,
		`request_engine_runs_total{engine="native",kind="solution",result="succeeded"} 1`,
		`request_engine_preflight_failures_total{code="REQUEST_ENGINE_OPENSPEC_MISSING"} 1`,
		`request_spec_check_violations_total{code="TASKSMD_DEPENDENCY_UNKNOWN"} 1`,
		`request_knowledge_tool_calls_total{tool="kg_search"} 4`,
		`request_knowledge_tool_calls_total{tool="kg_get_element"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(body, `tool="Read"`) || strings.Contains(body, "mcp__other") {
		t.Fatal("a tool that is not the knowledge graph's was counted as one")
	}
}

func TestEngineMetrics_AnUnknownKnowledgeToolNameIsCountedAsOtherSoTheLabelsStayBounded(t *testing.T) {
	s := New()
	s.KnowledgeToolCalls(domain.RunStats{ToolCalls: map[string]int{
		domain.KnowledgeToolPrefix + "made_up_one": 1, domain.KnowledgeToolPrefix + "made_up_two": 2,
	}})
	body := scrape(t, s)
	if !strings.Contains(body, `request_knowledge_tool_calls_total{tool="other"} 3`) || strings.Contains(body, "made_up") {
		t.Fatalf("labels were not bounded:\n%s", body)
	}
}

func TestEngineMetrics_NoCallsNoSeries(t *testing.T) {
	s := New()
	s.KnowledgeToolCalls(domain.RunStats{})
	s.KnowledgeToolCalls(domain.RunStats{ToolCalls: map[string]int{domain.KnowledgeToolPrefix + "kg_search": 0}})
	if strings.Contains(scrape(t, s), "request_knowledge_tool_calls_total{") {
		t.Fatal("a zero count created a series")
	}
}

func TestEngineMetrics_ReviewDrawingsAreCountedByAClosedSetOfResults(t *testing.T) {
	s := New()
	for _, r := range []string{"hit", "hit", "miss", "failed", "corrupt", "surprise"} {
		s.ArtifactLookup(r)
	}
	s.ArtifactStored(1500, false)
	s.ArtifactStored(500, true)
	s.ArtifactStored(0, false)
	body := scrape(t, s)
	for _, want := range []string{
		`request_review_artifact_lookups_total{result="hit"} 2`, `request_review_artifact_lookups_total{result="miss"} 1`,
		`request_review_artifact_lookups_total{result="failed"} 1`, `request_review_artifact_lookups_total{result="corrupt"} 1`,
		`request_review_artifact_lookups_total{result="other"} 1`, `request_review_artifact_bytes_stored_total 2000`, `request_review_artifact_blobs_deduped_total 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s in\n%s", want, body)
		}
	}
	if strings.Contains(body, "surprise") {
		t.Error("a result outside the set must not become a label")
	}
}

func TestEngineMetrics_ReviewSeriesHaveClosedLabelsAndNoContent(t *testing.T) {
	s := New()
	s.ReviewContextLookup(true)
	s.ReviewContextLookup(false)
	s.ReviewContextLookup(false)
	s.ReviewRunEnded("context", "succeeded", 42*time.Second)
	s.ReviewRunEnded("my-kind", "exploded", time.Second)
	s.ReviewTokens("draw", 1200, 300)
	s.ReviewTokens("draw", 0, 0)
	s.ReviewCostEstimateError("context", 0.2)
	s.ReviewDrawn(3, 1, "REQUEST_REVIEW_ARTIFACT_UNSAFE")
	s.ReviewDrawn(0, 2, "some free text with a secret")
	s.ReviewRegenerated("looks_wrong")
	s.ReviewRegenerated("because I said so")
	s.ReviewFirstFacts(300 * time.Millisecond)
	s.ReviewItems(10, 3)
	s.ReviewItems(2, 9)
	s.ReviewDesignReviewed(0.5)
	s.ReviewDesignReviewed(7)
	body := scrape(t, s)
	for _, want := range []string{
		`request_review_context_lookups_total{result="hit"} 1`, `request_review_context_lookups_total{result="miss"} 2`,
		`request_review_run_duration_seconds_count{kind="context",result="succeeded"} 1`,
		`request_review_run_duration_seconds_count{kind="other",result="other"} 1`,
		`request_review_tokens_total{direction="input",kind="draw"} 1200`, `request_review_tokens_total{direction="output",kind="draw"} 300`,
		`request_review_cost_estimate_error_ratio_count{kind="context"} 1`,
		`request_review_diagrams_total{result="drawn"} 3`, `request_review_diagrams_total{result="failed"} 3`,
		`request_review_draw_failures_total{error_code="REQUEST_REVIEW_ARTIFACT_UNSAFE"} 1`, `request_review_draw_failures_total{error_code="other"} 2`,
		`request_review_regenerations_total{reason="looks_wrong"} 1`, `request_review_regenerations_total{reason="other"} 1`,
		`request_review_time_to_first_facts_seconds_count 1`,
		`request_review_agent_items_total{verified="no"} 5`, `request_review_agent_items_total{verified="yes"} 7`,
		`request_review_design_reviewed_ratio_count 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, leak := range []string{"secret", "free text", "because I said so", "my-kind", "exploded"} {
		if strings.Contains(body, leak) {
			t.Errorf("a value outside the closed sets became a label: %q", leak)
		}
	}
}
