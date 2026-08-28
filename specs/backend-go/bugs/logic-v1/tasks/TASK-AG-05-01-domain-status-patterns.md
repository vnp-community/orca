# TASK-AG-05-01: `domain/agent_status_pattern.go` — rate-limit/waiting/completed text patterns (pure)

**From Solution:** SOL-AG-05
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/agent_status_pattern.go` (new)
**Depends on:** TASK-AG-01-02
**Status:** `[x]` DONE — domain/agent_status_pattern.go (RateLimitPatterns + ClassifyText) implemented verbatim; agent_status_pattern_test.go covers claude/codex/opencode rate-limit samples, waiting/completed matches, and an unrelated-string ok=false case — all passing.

---

## Context

Track-2 (OSC 133 + text-pattern) classification's pure text-matching rules — the same `RATE_LIMIT_PATTERNS` set `SwitchAgentAccount`'s rate-limit trigger (TASK-AG-04) consumes, not a duplicate. No I/O, unit-testable directly.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/domain/agent_status_pattern.go`:

```go
package domain

import "regexp"

// RateLimitPatterns mirrors BL-AG-04/05's RATE_LIMIT_PATTERNS 1:1 —
// deliberately the SAME pattern set SwitchAgentAccount's rate-limit trigger
// consumes (TASK-AG-04), not a duplicate.
var RateLimitPatterns = map[string]*regexp.Regexp{
	"claude":   regexp.MustCompile(`(?i)rate.?limit|quota.?exceed|too.?many.?request`),
	"codex":    regexp.MustCompile(`(?i)rate.?limit|429|quota`),
	"opencode": regexp.MustCompile(`(?i)rate.?limit|quota`),
}

var (
	waitingPattern   = regexp.MustCompile(`(?i)waiting for input`)
	completedPattern = regexp.MustCompile(`(?i)task completed`)
)

// ClassifyText applies track-2's text-pattern rules to one output chunk —
// pure function, no I/O. rateLimited is a side-channel signal, not itself
// an AgentStatus value (see agent_output_classifier.go's caller, TASK-AG-05-04:
// a rate-limited chunk is published as agent:rateLimited, never persisted
// as a session status).
func ClassifyText(agentKind, chunk string) (status AgentStatus, rateLimited bool, ok bool) {
	if pat, found := RateLimitPatterns[agentKind]; found && pat.MatchString(chunk) {
		return "", true, true
	}
	if completedPattern.MatchString(chunk) {
		return AgentStatusCompleted, false, true
	}
	if waitingPattern.MatchString(chunk) {
		return AgentStatusWaiting, false, true
	}
	return "", false, false
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/domain/... -run TestClassifyText -v
```

Add `agent_status_pattern_test.go`: each `RateLimitPatterns` regex against
known claude/codex/opencode sample strings (`"Error: rate limit exceeded"`,
`"429 Too Many Requests"`, `"quota exceeded"`); `"waiting for input"`/
`"task completed"` matches; an unrelated string returns `ok=false`.
