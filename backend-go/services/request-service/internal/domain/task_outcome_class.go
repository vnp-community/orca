package domain

const (
	TaskTypePlan  = "plan"
	TaskTypePhase = "phase"

	// Wire values of the task-service statuschanged "cause" field.
	CauseExecuteClaim       = "execute_claim"
	CauseExecutionCompleted = "execution_completed"
	CauseExecutionFailed    = "execution_failed"
	CauseRecovery           = "recovery"
	CauseUserUpdate         = "user_update"
	CauseDerived            = "derived"

	TaskStatusOpen       = "open"
	TaskStatusBlocked    = "blocked"
	TaskStatusInProgress = "in_progress"
	TaskStatusReview     = "review"
	TaskStatusDone       = "done"
	TaskStatusCancelled  = "cancelled"
)

// IsContainerTaskType is true for the derived-status nodes of a Plan tree.
func IsContainerTaskType(taskType string) bool {
	return taskType == TaskTypePlan || taskType == TaskTypePhase
}

// ClassifyTaskOutcome is the table of CR-REQ-013 section 2.5.1. ok=false means the event is outside the
// table and only worth a log line.
func ClassifyTaskOutcome(taskType, cause, newStatus string) (Outcome, bool) {
	switch taskType {
	case TaskTypePhase:
		if cause == CauseDerived && newStatus == TaskStatusDone {
			return OutcomePhaseDone, true
		}
		return "", false
	case TaskTypePlan:
		if cause == CauseDerived && newStatus == TaskStatusDone {
			return OutcomePlanDone, true
		}
		return "", false
	}
	switch {
	case cause == CauseExecuteClaim && newStatus == TaskStatusInProgress:
		return OutcomeStarted, true
	case cause == CauseExecutionCompleted && newStatus == TaskStatusReview:
		return OutcomeSucceeded, true
	case cause == CauseUserUpdate && newStatus == TaskStatusDone:
		return OutcomeSucceeded, true
	case cause == CauseExecutionFailed || cause == CauseRecovery:
		return OutcomeFailed, true
	case newStatus == TaskStatusCancelled:
		return OutcomeCancelled, true
	}
	return "", false
}
