package devserveragent

// Agent methods always self-expire before Go-side timeouts.
// ai.complete = 120s will be added by BE-CV-SOL-093.

import (
	"strings"
	"time"
)

const (
	execPromptTimeout    = 15 * time.Minute
	codeIntelReadTimeout = 90 * time.Second
)

var execTimeoutOverrides = map[string]time.Duration{
	"agent.execPrompt":     execPromptTimeout,
	"quality.listProfiles": 45 * time.Second,
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
