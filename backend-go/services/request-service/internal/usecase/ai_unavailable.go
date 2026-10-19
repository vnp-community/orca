package usecase

import "errors"

// Codes for "the AI step cannot run, and here is what to fix". They are stored as the reason of a failed
// classification or solution run and shown to the user, so each names one remedy.
const (
	// AICodeNotConfigured: the dev server has neither an API key nor a Claude Code login that ai.complete can use.
	AICodeNotConfigured = "REQUEST_AI_NOT_CONFIGURED"
	// AICodeAgentUnavailable: no dev server agent is connected, or its build cannot run ai.complete.
	AICodeAgentUnavailable = "REQUEST_AI_AGENT_UNAVAILABLE"
	// AICodeSessionExpired: the Claude Code login on the dev server stopped working mid-run.
	AICodeSessionExpired = "REQUEST_AI_SESSION_EXPIRED"
	// AICodeQuotaExhausted: the account behind the dev server's login ran out of quota.
	AICodeQuotaExhausted = "REQUEST_AI_QUOTA_EXHAUSTED"
	// AICodeBusy: the dev server is already running as many AI calls as it allows.
	AICodeBusy = "REQUEST_AI_BUSY"
)

// AIUnavailableError is returned by the AI adapters when a run failed for a reason the user can act on.
type AIUnavailableError struct {
	Code   string
	Detail string
	Cause  error
}

func (e *AIUnavailableError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

func (e *AIUnavailableError) Unwrap() error { return e.Cause }

// AIUnavailableCode reports the code of an AIUnavailableError anywhere in err's chain.
func AIUnavailableCode(err error) (string, bool) {
	var ae *AIUnavailableError
	if errors.As(err, &ae) {
		return ae.Code, true
	}
	return "", false
}
