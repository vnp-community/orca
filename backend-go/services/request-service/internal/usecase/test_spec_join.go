package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TestSpecJoin is the meeting point of the two parallel tracks, Plan generation and the test-spec lane, at the start of
// execution. Generation never waits for the other track; execution does, because the test cases and gaps feed the coding agent
// and cannot be added to a task that already ran.
type TestSpecJoin struct {
	Runs    TestSpecRunRepository
	Gate    domain.TestSpecGate
	Timeout time.Duration
	// Settings says whether the lane is on for the project; a project without test specs is never held.
	Settings TestSpecSettings
	Now      func() time.Time
}

func (j *TestSpecJoin) now() time.Time {
	if j.Now != nil {
		return j.Now().UTC()
	}
	return time.Now().UTC()
}

func (j *TestSpecJoin) gate() domain.TestSpecGate {
	if j.Gate == "" {
		return domain.DefaultTestSpecGate
	}
	return j.Gate
}

func (j *TestSpecJoin) timeout() time.Duration {
	if j.Timeout <= 0 {
		return domain.DefaultTestSpecGateTimeout
	}
	return j.Timeout
}

// Decide reads the lane of the Request and applies the gate.
func (j *TestSpecJoin) Decide(ctx context.Context, req domain.Request) (domain.JoinDecision, error) {
	if j.Settings != nil && !j.Settings.Enabled(ctx, req.ProjectID) {
		return domain.JoinDecision{Verdict: domain.JoinGo}, nil
	}
	run, found, err := j.Runs.Current(ctx, req.ID)
	if err != nil {
		return domain.JoinDecision{}, err
	}
	if !found {
		return domain.JoinDecision{Verdict: domain.JoinGo}, nil
	}
	timedOut := run.State.Active() && j.now().Sub(run.StartedAt) > j.timeout()
	return domain.DecideJoin(j.gate(), &run, timedOut), nil
}

// Enforce is Decide as a Start button needs it: go -> nil, otherwise REQUEST_TESTSPEC_NOT_READY.
func (j *TestSpecJoin) Enforce(ctx context.Context, req domain.Request) error {
	d, err := j.Decide(ctx, req)
	if err != nil {
		return err
	}
	if d.Verdict == domain.JoinGo {
		return nil
	}
	return domain.ErrTestSpecNotReady(d.Status, d.Cause)
}

// View is the join as the screen shows it next to the lane.
type TestSpecJoinView struct {
	Gate    domain.TestSpecGate
	Verdict domain.JoinVerdict
	Warn    bool
	Cause   string
}

func (j *TestSpecJoin) View(ctx context.Context, req domain.Request) (TestSpecJoinView, error) {
	d, err := j.Decide(ctx, req)
	if err != nil {
		return TestSpecJoinView{}, err
	}
	return TestSpecJoinView{Gate: j.gate(), Verdict: d.Verdict, Warn: d.Warn, Cause: d.Cause}, nil
}
