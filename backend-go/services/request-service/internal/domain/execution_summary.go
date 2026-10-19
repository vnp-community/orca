package domain

import "time"

// ExecutionSummary is what the Request detail screen needs to follow a run in one read: how far it got, what runs now,
// what failed and why, and what waits for a person. Nothing in it is stored; it is computed from the task tree, the run
// outcomes, the approvals and the clarification of the Request.
type ExecutionSummary struct {
	RequestID     string
	Status        RequestStatus
	Engine        EngineName
	ExecutionMode ExecutionMode
	Progress      ExecutionProgress
	CurrentPhase  string
	NextPhase     string
	Running       []RunningTask
	Failed        []FailedTask
	NeedsYou      []NeedsYouItem
	Handover      Handover
	MaxAttempts   int
	// StartedAt is when the first task of this run started; nil before any did.
	StartedAt *time.Time
}

type ExecutionProgress struct {
	PhasesTotal, PhasesDone                          int
	TasksTotal, TasksDone, TasksFailed, TasksRunning int
}

type RunningTask struct {
	TaskID, PhaseID, Title string
	Attempt, MaxAttempts   int
	StartedAt              time.Time
}

type FailedTask struct {
	TaskID, PhaseID, Title string
	// Attempts is how many runs ended in failure and used up an attempt.
	Attempts, MaxAttempts int
	LastCause             FailureCause
	LastError             string
	LastAt                time.Time
}

type NeedsYouKind string

const (
	NeedsApproval      NeedsYouKind = "approval"
	NeedsClarification NeedsYouKind = "clarification"
	NeedsFailedTask    NeedsYouKind = "failed_task"
	NeedsPreDeploy     NeedsYouKind = "pre_deploy"
	NeedsDrift         NeedsYouKind = "drift"
)

type NeedsYouItem struct {
	Kind        NeedsYouKind
	RefID       string
	SubjectType string
	Title       string
	PhaseID     string
	Since       time.Time
	DueAt       *time.Time
}

type HandoverState string

const (
	HandoverNone      HandoverState = "none"
	HandoverArchiving HandoverState = "archiving"
	HandoverReady     HandoverState = "ready"
	HandoverFailed    HandoverState = "failed"
)

type Handover struct {
	State           HandoverState
	ArchiveAttempts int
	// SummaryRef names the change that was archived (its directory under openspec/changes/archive).
	SummaryRef string
}
