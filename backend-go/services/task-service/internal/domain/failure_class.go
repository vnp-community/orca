package domain

import (
	"context"
	"errors"
)

// FailureClass tells request-service how to react to a failed run.
type FailureClass string

const (
	FailureRetryable   FailureClass = "retryable"
	FailureNeedsInfo   FailureClass = "needs_info"
	FailureSpecDefect  FailureClass = "spec_defect"
	FailureEnvDefect   FailureClass = "env_defect"
	FailureAgentDefect FailureClass = "agent_defect"
)

func (f FailureClass) Valid() bool {
	switch f {
	case FailureRetryable, FailureNeedsInfo, FailureSpecDefect, FailureEnvDefect, FailureAgentDefect:
		return true
	}
	return false
}

// TransientError marks a relay error worth retrying (peer unavailable, deadline) without the
// domain knowing about gRPC; the adapter wraps such errors.
type TransientError interface{ Transient() bool }

// ClassifyRunFailure turns the raw outcome of one run into (class, code); a clean run returns
// ("", ""). Transport trouble outranks the agent's own words: with no output nothing else can be said.
func ClassifyRunFailure(timedOut bool, exit *int, p ParsedExecution, relayErr error) (FailureClass, string) {
	if relayErr != nil {
		var te TransientError
		if errors.Is(relayErr, context.DeadlineExceeded) || (errors.As(relayErr, &te) && te.Transient()) {
			return FailureRetryable, "RELAY_UNAVAILABLE"
		}
		return FailureEnvDefect, "RELAY_FAILED"
	}
	if timedOut {
		return FailureRetryable, "TIMED_OUT"
	}
	switch p.Status {
	case ParseStatusMissing:
		return FailureAgentDefect, ResultCodeBlockMissing
	case ParseStatusInvalid:
		return FailureAgentDefect, p.Code
	}
	if p.Result == nil {
		return FailureAgentDefect, ResultCodeBlockMissing
	}
	switch p.Result.Status {
	case ResultNeedsInfo:
		return FailureNeedsInfo, "RESULT_NEEDS_INFO"
	case ResultBlocked:
		return FailureNeedsInfo, "RESULT_BLOCKED"
	case ResultFailed:
		return FailureAgentDefect, "RESULT_FAILED"
	}
	if exit == nil {
		return FailureAgentDefect, "EXIT_MISSING"
	}
	if *exit != 0 {
		return FailureAgentDefect, "EXIT_NONZERO_WITH_DONE"
	}
	return "", ""
}
