package domain

import (
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// Plan generation and the test-spec lane start together when a Request enters planning and do not wait for each other.
// They meet before execution: the first code task of the Request is not dispatched until the lane has finished, so the
// coding agent can read the test cases and gaps. TestSpecGate says how strict that join is.
type TestSpecGate string

const (
	// TestSpecGateWait holds execution until the lane has finished, whatever the outcome: a blocked or failed lane is shown on
	// the screen and execution goes on without test specs (the lane is advisory).
	TestSpecGateWait TestSpecGate = "wait"
	// TestSpecGateRequire holds execution until the lane completed (or was skipped on purpose); a blocked or failed lane
	// stops execution until a person runs it again.
	TestSpecGateRequire TestSpecGate = "require"
	// TestSpecGateWarn never holds execution; the state is only shown.
	TestSpecGateWarn TestSpecGate = "warn"
	TestSpecGateOff  TestSpecGate = "off"
)

const (
	DefaultTestSpecGate        = TestSpecGateWait
	DefaultTestSpecGateTimeout = 60 * time.Minute
)

func ParseTestSpecGate(s string) (TestSpecGate, error) {
	switch g := TestSpecGate(s); g {
	case "":
		return DefaultTestSpecGate, nil
	case TestSpecGateWait, TestSpecGateRequire, TestSpecGateWarn, TestSpecGateOff:
		return g, nil
	}
	return "", fmt.Errorf("unknown test spec gate %q (want wait, require, warn or off)", s)
}

// Usable is true when execution can read the test specs, or has none on purpose.
func (s TestSpecStatus) Usable() bool { return s == TestSpecCompleted || s == TestSpecSkipped }

type JoinVerdict string

const (
	JoinGo   JoinVerdict = "go"   // execution may start
	JoinWait JoinVerdict = "wait" // the lane is still generating
	JoinStop JoinVerdict = "stop" // the lane ended without test specs and the gate does not let execution go on
)

type JoinDecision struct {
	Verdict JoinVerdict
	// Warn is set when execution goes on although the test specs are not usable.
	Warn   bool
	Status TestSpecStatus
	// Cause is the reason code of the lane (or REQUEST_TESTSPEC_DEADLINE when the wait ran out).
	Cause string
}

// DecideJoin is the join of the two parallel tracks as execution sees it. run is nil when the lane never started for the
// Request. timedOut says the lane has been going longer than the gate timeout.
func DecideJoin(gate TestSpecGate, run *TestSpecRun, timedOut bool) JoinDecision {
	if gate == TestSpecGateOff || run == nil {
		return JoinDecision{Verdict: JoinGo}
	}
	st := run.State
	if st.Usable() {
		return JoinDecision{Verdict: JoinGo, Status: st}
	}
	if gate == TestSpecGateWarn {
		return JoinDecision{Verdict: JoinGo, Warn: true, Status: st, Cause: run.ReasonCode}
	}
	if st.Active() && !timedOut {
		return JoinDecision{Verdict: JoinWait, Status: st}
	}
	cause := run.ReasonCode
	if st.Active() {
		cause = TestSpecDeadline
	}
	if gate == TestSpecGateWait {
		return JoinDecision{Verdict: JoinGo, Warn: true, Status: st, Cause: cause}
	}
	return JoinDecision{Verdict: JoinStop, Status: st, Cause: cause}
}

// ErrTestSpecNotReady is what a person pressing Start gets while the join holds execution.
func ErrTestSpecNotReady(status TestSpecStatus, cause string) error {
	msg := fmt.Sprintf("test specs are %s; execution starts when they have finished", status)
	if cause != "" {
		msg += " (" + cause + ")"
	}
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_TESTSPEC_NOT_READY", msg, nil)
}
