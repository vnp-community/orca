package domain

import "strings"

// FailureCause is why a task run ended badly, as a person can act on it. The stored `cause` of a task_run_outcomes row says
// which path wrote it (execution_failed, dispatch_error...), and the error message says what task-service reported; this is the
// reading of both.
type FailureCause string

const (
	FailureTimedOut   FailureCause = "timed_out"
	FailureModelError FailureCause = "model_error"
	FailureVerify     FailureCause = "verify_failed"
	FailureEnv        FailureCause = "env_defect"
	FailureSpec       FailureCause = "spec_defect"
	FailureAgent      FailureCause = "agent_error"
	FailureNeedsInfo  FailureCause = "needs_info"
	FailureCancelled  FailureCause = "cancelled"
	FailureUnknown    FailureCause = "unknown"
)

// ClassifyFailureCause reads what the run reported. Messages look like "execution failed: <class> <CODE>" (for example
// "execution failed: retryable TIMED_OUT" or "execution failed: agent_defect RESULT_BLOCK_MISSING"); a dispatch error is just a
// code such as TASK_EXECUTE_NO_CONNECTION. Anything not recognised is unknown, never guessed.
func ClassifyFailureCause(cause, errorMessage string) FailureCause {
	m := strings.ToUpper(errorMessage)
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(m, s) {
				return true
			}
		}
		return false
	}
	switch {
	case has("TIMED_OUT", "TIMEOUT"):
		return FailureTimedOut
	case has("UNRECOGNIZED_MODEL", "MODEL_NOT_FOUND", "MODEL_UNAVAILABLE", "MODEL_ERROR"):
		return FailureModelError
	case has("VERIFY_NOT_PASSED", "VERIFY_FAILED"):
		return FailureVerify
	case has("NEEDS_INFO"):
		return FailureNeedsInfo
	case has("SPEC_DEFECT"):
		return FailureSpec
	case has("ENV_DEFECT", "NO_CONNECTION", "RELAY_UNAVAILABLE", "DEV_SERVER"):
		return FailureEnv
	case has("AGENT_DEFECT", "RESULT_BLOCK_MISSING", "RESULT_"):
		return FailureAgent
	}
	if cause == CauseDispatchError {
		return FailureEnv // the agent never ran: nothing about the task itself failed
	}
	return FailureUnknown
}
