package agentrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Output formats ai.complete understands; the agent defaults to text.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// CompleteParams is the params object of ai.complete. There is deliberately no field for an API key: the agent
// resolves the account's credential itself, and a test fails if one is added.
type CompleteParams struct {
	Prompt    string `json:"prompt"`
	Format    string `json:"format,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
	Model     string `json:"model,omitempty"`
	MaxTokens int    `json:"maxTokens,omitempty"`
	AccountID string `json:"accountId,omitempty"`

	// Timeout bounds the whole call on this side; it is not sent. Zero leaves the caller's context in charge.
	Timeout time.Duration `json:"-"`
}

// CompleteResult is the agent's completion; everything but Content is optional.
type CompleteResult struct {
	Content   string
	Model     string
	Provider  string
	LatencyMS int64
	Usage     *CompleteUsage
}

// CompleteUsage holds the token counts the provider reported.
type CompleteUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// Complete runs ai.complete on the target's agent.
func (r *Relayer) Complete(ctx context.Context, t Target, p CompleteParams) (CompleteResult, error) {
	cctx, cancel, err := r.prepare(ctx, p.Timeout)
	if err != nil {
		return CompleteResult{}, err
	}
	defer cancel()
	raw, err := json.Marshal(p)
	if err != nil {
		return CompleteResult{}, fmt.Errorf("agentrelay: marshal %s params: %w", MethodComplete, err)
	}
	resultJSON, err := r.call(cctx, t, MethodComplete, raw)
	if err != nil {
		return CompleteResult{}, classifyCompleteError(err)
	}
	return parseCompleteResult(resultJSON)
}
