package domain

import "time"

type Outcome string

const (
	OutcomeStarted   Outcome = "started"
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeCancelled Outcome = "cancelled"
	OutcomePhaseDone Outcome = "phase_done"
	OutcomePlanDone  Outcome = "plan_done"
)

// CauseDispatchError marks a failed outcome that never reached the agent; it does not use up an attempt.
const CauseDispatchError = "dispatch_error"

func AllOutcomes() []Outcome {
	return []Outcome{OutcomeStarted, OutcomeSucceeded, OutcomeFailed, OutcomeCancelled, OutcomePhaseDone, OutcomePlanDone}
}

// CompletesContainer is true for the outcomes that may be recorded once per container.
func (o Outcome) CompletesContainer() bool { return o == OutcomePhaseDone || o == OutcomePlanDone }

// TaskRunOutcome is one row of task_run_outcomes.
type TaskRunOutcome struct {
	ID              string
	TenantID        string
	RequestID       string
	TaskID          string
	ContainerID     string
	Outcome         Outcome
	Cause           string
	ExecutionLinkID string
	ErrorMessage    string
	EventID         string
	OccurredAt      time.Time
}
