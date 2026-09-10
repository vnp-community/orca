package domain

import "testing"

func TestClassifyText_RateLimitPatterns(t *testing.T) {
	cases := []struct {
		agentKind string
		chunk     string
	}{
		{"claude", "Error: rate limit exceeded"},
		{"claude", "quota exceeded for this account"},
		{"claude", "too many requests, please slow down"},
		{"codex", "429 Too Many Requests"},
		{"codex", "rate limit hit"},
		{"codex", "quota exceeded"},
		{"opencode", "rate-limit reached"},
		{"opencode", "quota exceeded"},
	}
	for _, tc := range cases {
		status, rateLimited, ok := ClassifyText(tc.agentKind, tc.chunk)
		if !ok || !rateLimited {
			t.Errorf("ClassifyText(%q, %q) = (%q, %v, %v), want rateLimited=true, ok=true", tc.agentKind, tc.chunk, status, rateLimited, ok)
		}
		if status != "" {
			t.Errorf("ClassifyText(%q, %q): expected empty status for a rate-limit signal, got %q", tc.agentKind, tc.chunk, status)
		}
	}
}

func TestClassifyText_WaitingAndCompleted(t *testing.T) {
	status, rateLimited, ok := ClassifyText("claude", "Waiting for input...")
	if !ok || rateLimited || status != AgentStatusWaiting {
		t.Errorf("expected AgentStatusWaiting, got (%q, %v, %v)", status, rateLimited, ok)
	}

	status, rateLimited, ok = ClassifyText("claude", "Task completed successfully")
	if !ok || rateLimited || status != AgentStatusCompleted {
		t.Errorf("expected AgentStatusCompleted, got (%q, %v, %v)", status, rateLimited, ok)
	}
}

func TestClassifyText_UnrelatedString_ReturnsNotOK(t *testing.T) {
	status, rateLimited, ok := ClassifyText("claude", "just some regular output")
	if ok || rateLimited || status != "" {
		t.Errorf("expected ok=false for an unrelated string, got (%q, %v, %v)", status, rateLimited, ok)
	}
}
