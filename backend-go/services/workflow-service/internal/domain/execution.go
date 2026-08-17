package domain

import (
	"errors"
	"time"
)

// Status is a WorkflowExecution's lifecycle state — see workflow-service.md
// §4. Transitions enforced by Pause/Resume below are the "real invariant
// check" this service's build instructions call out explicitly: pause only
// from running, resume only from paused.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

var (
	// ErrExecutionEmptyTenant guards an execution with no owning tenant.
	ErrExecutionEmptyTenant = errors.New("domain: tenant_id is required")
	// ErrExecutionEmptyTemplate guards an execution with no source template.
	ErrExecutionEmptyTemplate = errors.New("domain: template_id is required")
	// ErrCannotPauseNotRunning is the invariant PauseExecution enforces:
	// only a running execution can be paused (matching TS migration 0014's
	// user-triggered pause semantics, workflow-service.md §4-5 — pausing a
	// pending/paused/completed/failed/cancelled execution is not a
	// meaningful state transition).
	ErrCannotPauseNotRunning = errors.New("domain: cannot pause an execution that is not running")
	// ErrCannotResumeNotPaused is the invariant ResumeExecution enforces:
	// only a paused execution can resume.
	ErrCannotResumeNotPaused = errors.New("domain: cannot resume an execution that is not paused")
	// ErrExecutionNotFound is the sentinel adapter/postgres returns (wrapped)
	// when a lookup finds no row — usecase maps it to apperrors.KindNotFound.
	ErrExecutionNotFound = errors.New("domain: workflow execution not found")
)

// WorkflowExecution is one run of a WorkflowTemplate's DAG — see
// workflow-service.md §4/§5. RootTraceID and PausedAt are carried forward
// unconditionally, per the design doc's "hard requirement, not aspiration"
// framing for resumability (§8) and user-triggered pause (§4-5).
type WorkflowExecution struct {
	ID          string
	TenantID    string
	TemplateID  string
	Status      Status
	RootTraceID string
	PausedAt    *time.Time
}

// NewWorkflowExecution constructs a WorkflowExecution in StatusRunning —
// matching workflow-service.md §7's dependency diagram, which persists
// status=running immediately after building/validating the DAG, before
// wave dispatch begins (wave dispatch itself is not implemented in this
// scaffold, see README "Known gaps": the execution is recorded but never
// progresses past running on its own).
func NewWorkflowExecution(id, tenantID, templateID, rootTraceID string) (WorkflowExecution, error) {
	if tenantID == "" {
		return WorkflowExecution{}, ErrExecutionEmptyTenant
	}
	if templateID == "" {
		return WorkflowExecution{}, ErrExecutionEmptyTemplate
	}
	return WorkflowExecution{
		ID:          id,
		TenantID:    tenantID,
		TemplateID:  templateID,
		Status:      StatusRunning,
		RootTraceID: rootTraceID,
	}, nil
}

// Pause transitions a running execution to paused, recording PausedAt —
// the user-triggered pause path (TS's paused_at, migration 0014). Returns
// ErrCannotPauseNotRunning for any other current status: a deliberate user
// action must not silently "succeed" against, say, an already-completed
// execution.
func (e *WorkflowExecution) Pause(now time.Time) error {
	if e.Status != StatusRunning {
		return ErrCannotPauseNotRunning
	}
	e.Status = StatusPaused
	e.PausedAt = &now
	return nil
}

// Resume transitions a paused execution back to running, clearing
// PausedAt. Returns ErrCannotResumeNotPaused otherwise — in particular, a
// restart must never "resume" a running execution through this path (see
// workflow-service.md §8: the boot-time recovery scan re-attaches to a
// running execution's root_trace_id directly, it does not call Resume).
func (e *WorkflowExecution) Resume() error {
	if e.Status != StatusPaused {
		return ErrCannotResumeNotPaused
	}
	e.Status = StatusRunning
	e.PausedAt = nil
	return nil
}
