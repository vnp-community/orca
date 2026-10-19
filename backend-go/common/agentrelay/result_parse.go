package agentrelay

import (
	"encoding/json"
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	// maxToolNames and maxToolNameBytes bound what a hostile or buggy stream can grow in a caller's row; the agent
	// already caps them, but the sizes come over the wire.
	maxToolNames     = 128
	maxToolNameBytes = 100
)

// execResultWire mirrors the agent's PrintModeExecResult plus its optional additions. Unknown keys are ignored.
type execResultWire struct {
	Stdout    string         `json:"stdout"`
	Stderr    string         `json:"stderr"`
	ExitCode  *int           `json:"exitCode"`
	TimedOut  bool           `json:"timedOut"`
	StepID    string         `json:"stepId"`
	Warnings  []string       `json:"warnings"`
	Applied   *Applied       `json:"applied"`
	Changes   *Changes       `json:"changes"`
	Parsed    *parsedWire    `json:"parsed"`
	Truncated *Truncated     `json:"truncated"`
	ToolCalls map[string]int `json:"toolCalls"`
	Usage     *usageWire     `json:"usage"`
}

type parsedWire struct {
	OK     bool            `json:"ok"`
	Value  json.RawMessage `json:"value"`
	Code   string          `json:"code"`
	Detail string          `json:"detail"`
}

// usageWire reads every figure as a float because the agent serialises JavaScript numbers.
type usageWire struct {
	Turns               float64  `json:"turns"`
	DurationMS          float64  `json:"durationMs"`
	CostUSD             *float64 `json:"costUsd"`
	InputTokens         float64  `json:"inputTokens"`
	OutputTokens        float64  `json:"outputTokens"`
	CacheReadTokens     float64  `json:"cacheReadTokens"`
	CacheCreationTokens float64  `json:"cacheCreationTokens"`
}

// parseExecResult decodes the relay's resultJson. It tolerates an agent that predates usage, toolCalls, changes,
// parsed or truncated, and a null exit code.
func parseExecResult(resultJSON string) (ExecResult, error) {
	var w execResultWire
	if err := json.Unmarshal([]byte(resultJSON), &w); err != nil {
		return ExecResult{}, fmt.Errorf("%w: %s: %w", ErrMalformedResult, MethodExecPrompt, err)
	}
	out := ExecResult{
		Stdout: w.Stdout, Stderr: w.Stderr, ExitCode: w.ExitCode, TimedOut: w.TimedOut, StepID: w.StepID,
		Warnings: w.Warnings, Applied: w.Applied, Changes: w.Changes, Truncated: w.Truncated,
		Stats: statsFromWire(w),
	}
	if p := w.Parsed; p != nil {
		out.Parsed = &Parsed{OK: p.OK, Value: []byte(p.Value), Code: p.Code, Detail: p.Detail}
	}
	return out, nil
}

func statsFromWire(w execResultWire) Stats {
	var s Stats
	if len(w.ToolCalls) > 0 {
		s.ToolCalls = make(map[string]int, len(w.ToolCalls))
		for name, n := range w.ToolCalls {
			if n > 0 && len(s.ToolCalls) < maxToolNames {
				s.ToolCalls[TruncateUTF8(name, maxToolNameBytes)] += n
			}
		}
	}
	if u := w.Usage; u != nil {
		s.Usage = &Usage{
			Turns: int(clampFloat(u.Turns)), DurationMS: int64(clampFloat(u.DurationMS)), CostUSD: u.CostUSD,
			InputTokens: int64(clampFloat(u.InputTokens)), OutputTokens: int64(clampFloat(u.OutputTokens)),
			CacheReadTokens: int64(clampFloat(u.CacheReadTokens)), CacheCreationTokens: int64(clampFloat(u.CacheCreationTokens)),
		}
	}
	return s
}

// clampFloat keeps a float-to-integer conversion defined for values the agent should never send.
func clampFloat(f float64) float64 {
	if f < 0 || math.IsNaN(f) {
		return 0
	}
	return math.Min(f, 1<<53)
}

// TruncateUTF8 cuts s to at most limit bytes without splitting a rune. Callers apply their own output caps with it;
// the package returns the agent's text whole.
func TruncateUTF8(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// completeWire mirrors ai.complete's result {content, model?, provider?, latencyMs?, usage?}.
type completeWire struct {
	Content   string  `json:"content"`
	Model     string  `json:"model"`
	Provider  string  `json:"provider"`
	LatencyMS float64 `json:"latencyMs"`
	Usage     *struct {
		PromptTokens     float64 `json:"promptTokens"`
		CompletionTokens float64 `json:"completionTokens"`
		TotalTokens      float64 `json:"totalTokens"`
	} `json:"usage"`
}

func parseCompleteResult(resultJSON string) (CompleteResult, error) {
	var w completeWire
	if err := json.Unmarshal([]byte(resultJSON), &w); err != nil {
		return CompleteResult{}, fmt.Errorf("%w: %s: %w", ErrMalformedResult, MethodComplete, err)
	}
	out := CompleteResult{Content: w.Content, Model: w.Model, Provider: w.Provider, LatencyMS: int64(clampFloat(w.LatencyMS))}
	if u := w.Usage; u != nil {
		out.Usage = &CompleteUsage{
			PromptTokens: int64(clampFloat(u.PromptTokens)), CompletionTokens: int64(clampFloat(u.CompletionTokens)),
			TotalTokens: int64(clampFloat(u.TotalTokens)),
		}
	}
	return out, nil
}
