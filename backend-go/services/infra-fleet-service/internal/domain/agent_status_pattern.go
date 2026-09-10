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
