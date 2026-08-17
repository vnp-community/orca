package domain

import "errors"

// StepExecutionStatus is one StepExecution's lifecycle state — a narrower
// enum than WorkflowExecution's Status since a step never pauses: it's
// pending until dispatched, running while its StepExecutor call is
// in-flight, then completed or failed. See workflow-service.md §4/§5.
type StepExecutionStatus string

const (
	StepExecutionStatusPending   StepExecutionStatus = "pending"
	StepExecutionStatusRunning   StepExecutionStatus = "running"
	StepExecutionStatusCompleted StepExecutionStatus = "completed"
	StepExecutionStatusFailed    StepExecutionStatus = "failed"
)

var (
	// ErrStepExecutionEmptyExecutionID guards a step execution with no
	// owning execution — every row must join back to exactly one
	// WorkflowExecution.
	ErrStepExecutionEmptyExecutionID = errors.New("domain: step execution requires an execution id")
	// ErrStepExecutionEmptyStepID guards a step execution with no step id —
	// the join key back to the owning execution's DAG step (see
	// workflow-service.md §5: step_executions.step_id references the
	// definition snapshot, not a foreign-keyed row).
	ErrStepExecutionEmptyStepID = errors.New("domain: step execution requires a step id")
)

// StepExecution is one step's outcome within a WorkflowExecution's
// wave-dispatch run — see workflow-service.md §4/§5. usecase's
// wave-dispatch engine persists one row per step per wave (status=pending)
// before dispatching, then updates it as the step runs.
//
// DispatchToken is an idempotency key for a future boot-time recovery scan
// (§8's hard requirement) to de-dupe a re-dispatch after a crash lands
// between "step sent to StepExecutor" and "row marked terminal" — that
// recovery scan itself is out of this pass's scope (see README "Known
// gaps"), but the token is generated and persisted from day one so a later
// pass doesn't need a data migration to add it.
type StepExecution struct {
	ID            string
	ExecutionID   string
	StepID        string
	Wave          int
	Status        StepExecutionStatus
	DispatchToken string
	OutputJSON    string
	Error         string
}

// NewStepExecution constructs a StepExecution in StepExecutionStatusPending
// — the state usecase's wave-dispatch engine persists before dispatching
// the step to its StepExecutor.
func NewStepExecution(id, executionID, stepID, dispatchToken string, wave int) (StepExecution, error) {
	if executionID == "" {
		return StepExecution{}, ErrStepExecutionEmptyExecutionID
	}
	if stepID == "" {
		return StepExecution{}, ErrStepExecutionEmptyStepID
	}
	return StepExecution{
		ID:            id,
		ExecutionID:   executionID,
		StepID:        stepID,
		Wave:          wave,
		Status:        StepExecutionStatusPending,
		DispatchToken: dispatchToken,
	}, nil
}

// MarkRunning transitions a pending step execution to running — set right
// before its StepExecutor.Execute call, so a concurrent GetExecution-style
// read (not implemented over the wire yet, see README) would see
// "in-flight," not "pending," while the call is outstanding.
func (s *StepExecution) MarkRunning() {
	s.Status = StepExecutionStatusRunning
}

// FromResult records a StepExecutor call's business-level outcome — status
// (completed/failed) and OutputJSON verbatim from the StepResult. Use Fail
// instead when the StepExecutor call itself errored (no StepResult was
// produced at all) rather than running and reporting failure normally.
func (s *StepExecution) FromResult(result StepResult) {
	if result.Status == ResultStatusCompleted {
		s.Status = StepExecutionStatusCompleted
	} else {
		s.Status = StepExecutionStatusFailed
	}
	s.OutputJSON = result.OutputJSON
}

// Fail records a hard StepExecutor error (the call couldn't run the step
// at all) — distinct from FromResult's business-level "failed" outcome,
// see that method's doc comment.
func (s *StepExecution) Fail(errMsg string) {
	s.Status = StepExecutionStatusFailed
	s.Error = errMsg
}

// Terminal reports whether this step execution has reached a status the
// wave-dispatch engine's wave gate waits for (completed or failed) — a
// pending or running step is not yet terminal.
func (s StepExecution) Terminal() bool {
	return s.Status == StepExecutionStatusCompleted || s.Status == StepExecutionStatusFailed
}
