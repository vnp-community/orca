package metrics

import (
	"strings"
	"testing"

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
