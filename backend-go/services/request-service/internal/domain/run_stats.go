package domain

import (
	"encoding/json"
	"sort"
	"strings"
)

// KnowledgeToolPrefix is how claude names the tools of the tdd-agent MCP server.
const KnowledgeToolPrefix = "mcp__tdd-agent__"

// RunUsage is what claude reported about a run. Every figure is optional: a missing cost is "not reported", never zero.
type RunUsage struct {
	Turns               int      `json:"turns,omitempty"`
	DurationMS          int64    `json:"duration_ms,omitempty"`
	CostUSD             *float64 `json:"cost_usd,omitempty"`
	InputTokens         int64    `json:"input_tokens,omitempty"`
	OutputTokens        int64    `json:"output_tokens,omitempty"`
	CacheReadTokens     int64    `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int64    `json:"cache_creation_tokens,omitempty"`
}

// RunStats is what an agent run says about itself: the tools it called (names and counts only, never what went in or came
// out, which can be document text) and claude's usage figures. It is evidence for a reviewer, not a verdict.
type RunStats struct {
	ToolCalls map[string]int `json:"tool_calls,omitempty"`
	Usage     *RunUsage      `json:"usage,omitempty"`
}

// maxStatsTools bounds the distinct tool names kept on a run row.
const maxStatsTools = 64

// Merge adds another call's figures: a run that retried has two agent calls and one set of numbers.
func (s RunStats) Merge(o RunStats) RunStats {
	out := RunStats{}
	if len(s.ToolCalls) > 0 || len(o.ToolCalls) > 0 {
		out.ToolCalls = map[string]int{}
		for _, src := range []map[string]int{s.ToolCalls, o.ToolCalls} {
			for name, n := range src {
				key := name
				if _, known := out.ToolCalls[key]; !known && len(out.ToolCalls) >= maxStatsTools {
					key = "(other)"
				}
				out.ToolCalls[key] += n
			}
		}
	}
	out.Usage = mergeUsage(s.Usage, o.Usage)
	return out
}

func mergeUsage(a, b *RunUsage) *RunUsage {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		c := *b
		return &c
	case b == nil:
		c := *a
		return &c
	}
	out := RunUsage{
		Turns: a.Turns + b.Turns, DurationMS: a.DurationMS + b.DurationMS, InputTokens: a.InputTokens + b.InputTokens,
		OutputTokens: a.OutputTokens + b.OutputTokens, CacheReadTokens: a.CacheReadTokens + b.CacheReadTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
	}
	if a.CostUSD != nil || b.CostUSD != nil {
		var sum float64
		if a.CostUSD != nil {
			sum += *a.CostUSD
		}
		if b.CostUSD != nil {
			sum += *b.CostUSD
		}
		out.CostUSD = &sum
	}
	return &out
}

// IsZero is true when nothing was recorded.
func (s RunStats) IsZero() bool { return len(s.ToolCalls) == 0 && s.Usage == nil }

// KnowledgeCalls counts the calls to the knowledge graph's tools.
func (s RunStats) KnowledgeCalls() int {
	n := 0
	for name, c := range s.ToolCalls {
		if strings.HasPrefix(name, KnowledgeToolPrefix) {
			n += c
		}
	}
	return n
}

// KnowledgeToolNames lists the knowledge tools that were called, without the server prefix, sorted.
func (s RunStats) KnowledgeToolNames() []string {
	var out []string
	for name := range s.ToolCalls {
		if short, ok := strings.CutPrefix(name, KnowledgeToolPrefix); ok {
			out = append(out, short)
		}
	}
	sort.Strings(out)
	return out
}

// EncodeRunStats is the column value: nil when there is nothing to store.
func EncodeRunStats(s *RunStats) ([]byte, error) {
	if s == nil || s.IsZero() {
		return nil, nil
	}
	return json.Marshal(s)
}

// DecodeRunStats reads the column; an empty or unreadable value is no stats rather than a failure to read the run.
func DecodeRunStats(raw []byte) *RunStats {
	if len(raw) == 0 {
		return nil
	}
	var s RunStats
	if err := json.Unmarshal(raw, &s); err != nil || s.IsZero() {
		return nil
	}
	return &s
}
