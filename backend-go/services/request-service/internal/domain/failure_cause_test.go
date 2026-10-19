package domain

import "testing"

// The messages are the ones request #1242 stored.
func TestClassifyFailureCause(t *testing.T) {
	cases := []struct {
		cause, msg string
		want       FailureCause
	}{
		{"execution_failed", "execution failed: retryable TIMED_OUT", FailureTimedOut},
		{"execution_failed", "execution failed: agent_defect RESULT_BLOCK_MISSING", FailureAgent},
		{"execution_failed", "execution failed: needs_info RESULT_NEEDS_INFO", FailureNeedsInfo},
		{"dispatch_error", "TASK_EXECUTE_NO_CONNECTION", FailureEnv},
		{"dispatch_error", "execution failed: retryable RELAY_UNAVAILABLE: simple_executor: relayByDevServer agent.execPrompt", FailureEnv},
		{"execution_failed", "[claude-code:unrecognized_model] {\"model\":\"claude-sonnet-5-5\"}", FailureModelError},
		{"execution_failed", "execution failed: verify_not_passed VERIFY_NOT_PASSED", FailureVerify},
		{"execution_failed", "execution failed: spec_defect SPEC_DEFECT", FailureSpec},
		{"execution_failed", "execution failed: env_defect ENV_DEFECT", FailureEnv},
		{"dispatch_error", "something we never saw", FailureEnv},
		{"execution_failed", "something we never saw", FailureUnknown},
		{"", "", FailureUnknown},
	}
	for _, c := range cases {
		if got := ClassifyFailureCause(c.cause, c.msg); got != c.want {
			t.Errorf("ClassifyFailureCause(%q, %q) = %q, want %q", c.cause, c.msg, got, c.want)
		}
	}
}
