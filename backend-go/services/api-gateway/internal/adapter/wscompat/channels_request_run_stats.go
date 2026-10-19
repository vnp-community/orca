package wscompat

import requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"

// RunStatsView is what an agent run reported about itself: tool call counts (names and numbers only) and claude's usage.
// CostUSD is absent when claude did not report one: a missing cost is not a free run.
type RunStatsView struct {
	ToolCalls           map[string]int32 `json:"toolCalls"`
	KnowledgeToolCalls  int32            `json:"knowledgeToolCalls"`
	Turns               int32            `json:"turns,omitempty"`
	DurationMs          int64            `json:"durationMs,omitempty"`
	CostUSD             *float64         `json:"costUsd,omitempty"`
	InputTokens         int64            `json:"inputTokens,omitempty"`
	OutputTokens        int64            `json:"outputTokens,omitempty"`
	CacheReadTokens     int64            `json:"cacheReadTokens,omitempty"`
	CacheCreationTokens int64            `json:"cacheCreationTokens,omitempty"`
}

// runStatsView is nil (and so omitted) for a run that reported nothing.
func runStatsView(s *requestv1.RunStats) *RunStatsView {
	if s == nil {
		return nil
	}
	v := &RunStatsView{
		ToolCalls: s.GetToolCalls(), KnowledgeToolCalls: s.GetKnowledgeToolCalls(), Turns: s.GetTurns(), DurationMs: s.GetDurationMs(),
		InputTokens: s.GetInputTokens(), OutputTokens: s.GetOutputTokens(), CacheReadTokens: s.GetCacheReadTokens(), CacheCreationTokens: s.GetCacheCreationTokens(),
	}
	if v.ToolCalls == nil {
		v.ToolCalls = map[string]int32{}
	}
	if s.GetHasCost() {
		c := s.GetCostUsd()
		v.CostUSD = &c
	}
	return v
}
