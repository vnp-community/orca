package domain

const (
	SubjectPhaseStarted   = "orca.request.phase.started"
	SubjectPhaseCompleted = "orca.request.phase.completed"
)

// PhaseStartedPayload is the wire shape of orca.request.phase.started.
// The tasks it dispatched are in the StartPhase response, not here: the claim and the event commit together, before dispatch.
type PhaseStartedPayload struct {
	RequestID   string `json:"request_id"`
	PhaseTaskID string `json:"phase_task_id"`
	StartedBy   string `json:"started_by"`
}

// PhaseCompletedPayload is the wire shape of orca.request.phase.completed.
type PhaseCompletedPayload struct {
	RequestID   string `json:"request_id"`
	PhaseTaskID string `json:"phase_task_id"`
	TaskCount   int    `json:"task_count"`
}
