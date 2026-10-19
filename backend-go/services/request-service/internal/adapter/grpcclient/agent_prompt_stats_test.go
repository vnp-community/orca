package grpcclient

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestParseAgentResult_ReadsToolCallsAndUsageAndAnOlderAgentSendsNeither(t *testing.T) {
	r, err := parseAgentResult(`{"exitCode":0,"stdout":"x","toolCalls":{"mcp__tdd-agent__kg_search":3,"Read":2,"Bad":0},
		"usage":{"turns":5,"durationMs":1234,"costUsd":0.81,"inputTokens":100,"outputTokens":50,"cacheReadTokens":9000}}`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stats.KnowledgeCalls() != 3 || r.Stats.ToolCalls["Read"] != 2 || len(r.Stats.ToolCalls) != 2 {
		t.Fatalf("tool calls = %v", r.Stats.ToolCalls)
	}
	u := r.Stats.Usage
	if u == nil || u.Turns != 5 || u.DurationMS != 1234 || u.CostUSD == nil || *u.CostUSD != 0.81 || u.InputTokens != 100 || u.CacheReadTokens != 9000 || u.CacheCreationTokens != 0 {
		t.Fatalf("usage = %+v", u)
	}
	old, err := parseAgentResult(`{"exitCode":0,"stdout":"x"}`)
	if err != nil || !old.Stats.IsZero() {
		t.Fatalf("an older agent has no stats: %+v %v", old.Stats, err)
	}
}

func TestParseAgentResult_UsageWithoutACostLeavesTheCostUnreported(t *testing.T) {
	r, _ := parseAgentResult(`{"exitCode":0,"usage":{"turns":2}}`)
	if r.Stats.Usage == nil || r.Stats.Usage.CostUSD != nil {
		t.Fatalf("%+v", r.Stats.Usage)
	}
}

func TestParseAgentResult_BoundsWhatComesOverTheWire(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"exitCode":0,"toolCalls":{`)
	for i := 0; i < 300; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"tool_` + strings.Repeat("x", 3) + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676)) + `":1`)
	}
	b.WriteString(`,"` + strings.Repeat("n", 500) + `":1}}`)
	r, err := parseAgentResult(b.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Stats.ToolCalls) > 128 {
		t.Fatalf("kept %d tool names", len(r.Stats.ToolCalls))
	}
	for name := range r.Stats.ToolCalls {
		if len(name) > 100 {
			t.Fatalf("a %d byte tool name was kept", len(name))
		}
	}
	var _ domain.RunStats = r.Stats
}
