package domain

import (
	"reflect"
	"testing"
)

func cost(v float64) *float64 { return &v }

func TestRunStats_MergeAddsTwoAgentCallsOfOneRun(t *testing.T) {
	a := RunStats{ToolCalls: map[string]int{KnowledgeToolPrefix + "kg_search": 2, "Read": 1}, Usage: &RunUsage{Turns: 3, DurationMS: 1000, CostUSD: cost(0.5), OutputTokens: 10}}
	b := RunStats{ToolCalls: map[string]int{KnowledgeToolPrefix + "kg_search": 1, "Glob": 4}, Usage: &RunUsage{Turns: 2, DurationMS: 500, InputTokens: 7}}
	got := a.Merge(b)
	if got.ToolCalls[KnowledgeToolPrefix+"kg_search"] != 3 || got.ToolCalls["Read"] != 1 || got.ToolCalls["Glob"] != 4 {
		t.Fatalf("tool calls = %v", got.ToolCalls)
	}
	u := got.Usage
	if u.Turns != 5 || u.DurationMS != 1500 || u.OutputTokens != 10 || u.InputTokens != 7 || u.CostUSD == nil || *u.CostUSD != 0.5 {
		t.Fatalf("usage = %+v: a cost reported by only one call stays that call's cost", u)
	}
	if a.ToolCalls["Glob"] != 0 || a.Usage.Turns != 3 {
		t.Fatal("Merge changed its receiver")
	}
}

func TestRunStats_NoCostIsNotZeroCost(t *testing.T) {
	merged := RunStats{Usage: &RunUsage{Turns: 1}}.Merge(RunStats{Usage: &RunUsage{Turns: 1}})
	if merged.Usage.CostUSD != nil {
		t.Fatalf("a cost nobody reported appeared: %v", *merged.Usage.CostUSD)
	}
	if one := (RunStats{}).Merge(RunStats{Usage: &RunUsage{CostUSD: cost(0.1)}}); one.Usage == nil || *one.Usage.CostUSD != 0.1 {
		t.Fatalf("%+v", one)
	}
}

func TestRunStats_KnowledgeCallsCountOnlyTheGraphsTools(t *testing.T) {
	s := RunStats{ToolCalls: map[string]int{KnowledgeToolPrefix + "kg_get_element": 4, KnowledgeToolPrefix + "kg_search": 2, "Read": 9, "mcp__other__x": 5}}
	if s.KnowledgeCalls() != 6 {
		t.Fatalf("got %d", s.KnowledgeCalls())
	}
	if names := s.KnowledgeToolNames(); !reflect.DeepEqual(names, []string{"kg_get_element", "kg_search"}) {
		t.Fatalf("names = %v", names)
	}
}

func TestRunStats_TheColumnRoundTripsAndAnEmptyOneIsNull(t *testing.T) {
	raw, err := EncodeRunStats(&RunStats{ToolCalls: map[string]int{"Read": 2}, Usage: &RunUsage{Turns: 4, CostUSD: cost(1.25)}})
	if err != nil || len(raw) == 0 {
		t.Fatalf("%s %v", raw, err)
	}
	got := DecodeRunStats(raw)
	if got == nil || got.ToolCalls["Read"] != 2 || got.Usage.Turns != 4 || *got.Usage.CostUSD != 1.25 {
		t.Fatalf("%+v", got)
	}
	for _, empty := range []*RunStats{nil, {}, {ToolCalls: map[string]int{}}} {
		if raw, _ := EncodeRunStats(empty); raw != nil {
			t.Fatalf("an empty set of stats must store NULL, got %s", raw)
		}
	}
	for _, bad := range [][]byte{nil, []byte(""), []byte("not json"), []byte("{}")} {
		if DecodeRunStats(bad) != nil {
			t.Fatalf("%q read as stats", bad)
		}
	}
}

func TestRunStats_MergeCapsDistinctToolNames(t *testing.T) {
	big := map[string]int{}
	for i := 0; i < 70; i++ {
		big[string(rune('A'+i%26))+string(rune('a'+i/26))+"x"] = 1
	}
	got := RunStats{ToolCalls: big}.Merge(RunStats{})
	if len(got.ToolCalls) > maxStatsTools+1 {
		t.Fatalf("kept %d distinct names", len(got.ToolCalls))
	}
}
