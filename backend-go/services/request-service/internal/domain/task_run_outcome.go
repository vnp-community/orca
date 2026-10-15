package domain

import (
	"strings"
	"time"
)

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

// TransportFailureMarker is what task-service writes into the message of a run that failed only because the agent could not be
// reached (its failure code RELAY_UNAVAILABLE). Such a run says nothing about the task, so it is recorded like a dispatch error.
const TransportFailureMarker = "RELAY_UNAVAILABLE"

// CauseOf is the cause to record: a failed run lost to transport trouble is a dispatch error, which does not use up an attempt and is
// bounded by the dispatch retry window instead. Without that, a connection that drops for a second burns every attempt of the task.
func CauseOf(outcome Outcome, cause, errorMessage string) string {
	if outcome == OutcomeFailed && strings.Contains(errorMessage, TransportFailureMarker) {
		return CauseDispatchError
	}
	return cause
}

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
