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
	// ErrCannotCancelTerminal is the invariant CancelExecution enforces:
	// only a non-terminal execution (pending/running/paused) can be
	// cancelled — cancelling an already-completed/failed/cancelled
	// execution is not a meaningful state transition, same reasoning as
	// ErrCannotPauseNotRunning above.
	ErrCannotCancelTerminal = errors.New("domain: cannot cancel an execution that has already reached a terminal status")
	// ErrExecutionNotFound is the sentinel adapter/postgres returns (wrapped)
	// when a lookup finds no row — usecase maps it to apperrors.KindNotFound.
	ErrExecutionNotFound = errors.New("domain: workflow execution not found")
)

// WorkflowExecution is one run of a WorkflowTemplate's DAG — see
// workflow-service.md §4/§5. RootTraceID and PausedAt are carried forward
// unconditionally, per the design doc's "hard requirement, not aspiration"
// framing for resumability (§8) and user-triggered pause (§4-5). ProjectID
// is optional — an ad-hoc execution with no project binding is valid — and
// exists so HasActiveExecutions (Epic C, backend-go/docs/execution-plan.md)
// can answer "does this project have an active execution".
type WorkflowExecution struct {
	ID          string
	TenantID    string
	TemplateID  string
	Status      Status
	RootTraceID string
	PausedAt    *time.Time
	ProjectID   string
}

// NewWorkflowExecution constructs a WorkflowExecution in StatusRunning —
// matching workflow-service.md §7's dependency diagram, which persists
// status=running immediately after building/validating the DAG, before
// wave dispatch begins (wave dispatch itself is not implemented in this
// scaffold, see README "Known gaps": the execution is recorded but never
// progresses past running on its own). projectID is optional and not
// validated — see WorkflowExecution's doc comment.
func NewWorkflowExecution(id, tenantID, templateID, rootTraceID, projectID string) (WorkflowExecution, error) {
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
		ProjectID:   projectID,
	}, nil
}

// NewAdHocWorkflowExecution constructs a synthetic WorkflowExecution for
// ExecuteAdHocStep's single-step run — see workflow-service.md §3.1: "a
// real ad hoc step run should create one executions row ... so it gets the
// same observability/resumability/history as a template-driven step."
// Unlike NewWorkflowExecution, TemplateID is deliberately left empty: an ad
// hoc run has no backing WorkflowTemplate, and forcing a throwaway
// template row into existence just to satisfy a foreign key is exactly the
// indirection ExecuteAdHocStep exists to avoid (see that usecase's doc
// comment, TS Gap 3). Persisted as a NULL template_id (migration
// 0005_execution_ad_hoc_template makes the column nullable for this).
func NewAdHocWorkflowExecution(id, tenantID, rootTraceID string) (WorkflowExecution, error) {
	if tenantID == "" {
		return WorkflowExecution{}, ErrExecutionEmptyTenant
	}
	return WorkflowExecution{
		ID:          id,
		TenantID:    tenantID,
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

// Cancel transitions a pending/running/paused execution to cancelled — a
// terminal state (unlike Pause, which is resumable). Returns
// ErrCannotCancelTerminal for an execution already in a terminal status.
// Clears PausedAt: a cancelled execution was never "paused" in any
// meaningful sense a caller would query for, whichever status it was
// cancelled from.
func (e *WorkflowExecution) Cancel() error {
	switch e.Status {
	case StatusPending, StatusRunning, StatusPaused:
	default:
		return ErrCannotCancelTerminal
	}
	e.Status = StatusCancelled
	e.PausedAt = nil
	return nil
}
