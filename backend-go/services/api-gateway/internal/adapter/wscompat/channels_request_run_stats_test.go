package wscompat

import (
	"encoding/json"
	"strings"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

func TestRunStatsView_AMissingCostStaysMissingAndAReportedOneIsShown(t *testing.T) {
	noCost := runStatsView(&requestv1.RunStats{ToolCalls: map[string]int32{"Read": 2}, KnowledgeToolCalls: 0, Turns: 3})
	raw, _ := json.Marshal(noCost)
	if strings.Contains(string(raw), "costUsd") || !strings.Contains(string(raw), `"toolCalls":{"Read":2}`) {
		t.Fatalf("a cost nobody reported must not appear, even as zero: %s", raw)
	}
	free := runStatsView(&requestv1.RunStats{HasCost: true, CostUsd: 0})
	if free.CostUSD == nil || *free.CostUSD != 0 {
		t.Fatalf("a reported cost of zero is a real zero: %+v", free)
	}
	paid := runStatsView(&requestv1.RunStats{HasCost: true, CostUsd: 0.81, KnowledgeToolCalls: 5, OutputTokens: 9})
	if paid.CostUSD == nil || *paid.CostUSD != 0.81 || paid.KnowledgeToolCalls != 5 || paid.ToolCalls == nil {
		t.Fatalf("%+v", paid)
	}
	if runStatsView(nil) != nil {
		t.Fatal("no stats must stay no stats")
	}
}

func TestAnalysisRunViews_CarryTheEngineAndTheStatsOnlyWhenThereAreSome(t *testing.T) {
	views := analysisRunViews([]*requestv1.AnalysisRun{
		{Id: "r1", Status: "succeeded", Engine: "openspec", Stats: &requestv1.RunStats{KnowledgeToolCalls: 4, ToolCalls: map[string]int32{"mcp__tdd-agent__kg_search": 4}}},
		{Id: "r2", Status: "succeeded", Engine: "native"},
	})
	raw, _ := json.Marshal(views)
	got := string(raw)
	if !strings.Contains(got, `"engine":"openspec"`) || !strings.Contains(got, `"knowledgeToolCalls":4`) {
		t.Fatalf("%s", got)
	}
	if strings.Count(got, `"stats"`) != 1 {
		t.Fatalf("only the run that reported stats shows them: %s", got)
	}
}

func TestAddPlanEngineView_ShowsElapsedTimeAndStatsOfAnOpenSpecRun(t *testing.T) {
	out := map[string]any{}
	addPlanEngineView(out, &requestv1.GetPlanProposalResponse{Engine: "openspec", ElapsedMs: 83000, Stats: &requestv1.RunStats{HasCost: true, CostUsd: 1.5, KnowledgeToolCalls: 12}})
	if out["elapsedMs"] != int64(83000) {
		t.Fatalf("elapsed = %v", out["elapsedMs"])
	}
	st, ok := out["stats"].(*RunStatsView)
	if !ok || st.CostUSD == nil || *st.CostUSD != 1.5 || st.KnowledgeToolCalls != 12 {
		t.Fatalf("stats = %+v", out["stats"])
	}
	native := map[string]any{}
	addPlanEngineView(native, &requestv1.GetPlanProposalResponse{Engine: "native"})
	if _, has := native["stats"]; has {
		t.Fatalf("a native run has no stats to show: %v", native)
	}
	if _, has := native["elapsedMs"]; has {
		t.Fatalf("a run that has not finished has no elapsed time: %v", native)
	}
}
