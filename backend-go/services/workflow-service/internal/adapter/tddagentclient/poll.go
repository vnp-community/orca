package tddagentclient

import (
	"context"
	"errors"
	"time"
)

// Failure codes a tdd_agent.* step reports in its output "error".
const (
	CodeUnreachable     = "TDD_AGENT_UNREACHABLE"
	CodeDeadline        = "TDD_AGENT_DEADLINE"
	CodeCancelled       = "TDD_AGENT_CANCELLED"
	CodeUnexpectedGate  = "TDD_AGENT_UNEXPECTED_GATE"
	CodeBlocked         = "TDD_AGENT_BLOCKED"
	CodeRunFailed       = "TDD_AGENT_RUN_FAILED"
	CodeAdvanceCrashed  = "TDD_AGENT_ADVANCE_CRASHED"
	CodeInvalidParams   = "TDD_AGENT_INVALID_PARAMS"
	CodeChildMissing    = "TDD_AGENT_CHILD_MISSING"
	CodeApprovalReject  = "TDD_AGENT_APPROVAL_REJECTED"
	CodeApprovalRefused = "TDD_AGENT_APPROVAL_REFUSED"
)

// MaxTransportErrors consecutive failures to reach the dev server fail the step.
const MaxTransportErrors = 5

// stepFailure ends a step as failed with a code (and the run view when there is one).
type stepFailure struct {
	Code    string
	Message string
}

func (f *stepFailure) Error() string { return f.Code + ": " + f.Message }

func fail(code, msg string) error { return &stepFailure{Code: code, Message: msg} }

// clock is replaced in tests.
type clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// pollSpec bounds one wait.
type pollSpec struct {
	Interval time.Duration
	Deadline time.Duration
}

// poll calls check until it reports done, returns a non-transport error, the
// deadline passes, the execution is cancelled (cancelled() true) or the dev
// server stays unreachable MaxTransportErrors times in a row.
func poll(ctx context.Context, c clock, spec pollSpec, cancelled func() bool, check func() (bool, error)) error {
	start := c.Now()
	transport := 0
	for {
		if cancelled != nil && cancelled() {
			return fail(CodeCancelled, "the workflow execution was cancelled")
		}
		done, err := check()
		switch {
		case err == nil:
			transport = 0
			if done {
				return nil
			}
		case errors.Is(err, ErrTransport):
			transport++
			if transport >= MaxTransportErrors {
				return fail(CodeUnreachable, err.Error())
			}
		default:
			return err
		}
		if c.Now().Sub(start) >= spec.Deadline {
			return fail(CodeDeadline, "gave up waiting after "+spec.Deadline.String()+"; the tdd-agent run keeps going")
		}
		if err := c.Sleep(ctx, spec.Interval); err != nil {
			return fail(CodeCancelled, "the workflow-service is stopping: "+err.Error())
		}
	}
}
