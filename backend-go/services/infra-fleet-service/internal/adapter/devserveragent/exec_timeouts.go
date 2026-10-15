package devserveragent

// Agent methods always self-expire before Go-side timeouts.

import (
	"strings"
	"time"
)

const (
	// The agent kills a prompt run at its own 30 minute ceiling and then reports it; the Go side waits one minute longer so that
	// report is read instead of racing a simultaneous deadline.
	execPromptTimeout = 31 * time.Minute
	// ai.complete may run a model for a minute or more (a Solution or Plan is a long structured answer). The agent bounds
	// its own call at 120s, so this must be longer or the Go side gives up on a call that was about to succeed.
	aiCompleteTimeout    = 150 * time.Second
	codeIntelReadTimeout = 90 * time.Second
	// mcp.callTool: the agent caps one tool call at 120s; long work (a tdd-agent advance) runs in the background there.
	mcpCallToolTimeout = 150 * time.Second
)

var execTimeoutOverrides = map[string]time.Duration{
	"agent.execPrompt":     execPromptTimeout,
	"ai.complete":          aiCompleteTimeout,
	"quality.listProfiles": 45 * time.Second,
	"mcp.callTool":         mcpCallToolTimeout,
}

// execTimeoutForMethod returns method-specific timeout overrides for Exec.
// Returning 0 indicates that cfg.RequestTimeout (default 30s) should be used.
func execTimeoutForMethod(method string) time.Duration {
	if d, ok := execTimeoutOverrides[method]; ok {
		return d
	}
	if strings.HasPrefix(method, "codeintel.") {
		switch method {
		case "codeintel.status", "codeintel.reindex", "codeintel.reindexStatus", "codeintel.reindexCancel", "codeintel.watch":
			return 0
		}
		return codeIntelReadTimeout
	}
	return 0
}

// isAgentRunMethod marks the calls where the agent is working for a caller for minutes: a task run and a model call.
// They are what makes a dev server "busy"; short probes (ports.scan, fs.*) are not.
func isAgentRunMethod(method string) bool {
	return method == "agent.execPrompt" || method == "ai.complete"
}
