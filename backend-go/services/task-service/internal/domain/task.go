// Package domain holds task-service's entities and pure domain services. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, no context.Context.
package domain

import (
	"errors"
	"time"
)

// Status values a Task can hold. Kept as an open-ish small set matching the
// TS source's status strings (see specs/backend-go/services/task-service.md
// §10 — faithful port, not a redesign).
const (
	StatusOpen       = "open"
	StatusBlocked    = "blocked" // new — see TASK-TG-01-07's auto-block design
	StatusInProgress = "in_progress"
	StatusReview     = "review" // new — SOL-TG-04's completion target
	StatusDone       = "done"
	StatusCancelled  = "cancelled"
)

var (
	// ErrEmptyTenant is returned when TenantID is empty — a task with no
	// owning tenant is never a valid domain state, per
	// architecture/05-data-architecture.md's tenant-isolation rule.
	ErrEmptyTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyTitle guards against a title-less task.
	ErrEmptyTitle = errors.New("domain: title is required")
	// ErrInvalidStatus is returned by NewTask/SetStatus for an unrecognized
	// status string.
	ErrInvalidStatus = errors.New("domain: invalid task status")
	// ErrSelfParent guards against a task being its own parent, the
	// smallest possible cycle in the parent-child hierarchy.
	ErrSelfParent = errors.New("domain: a task cannot be its own parent")
	// ErrTerminalStatus is returned by SetStatus when the task is already
	// Done or Cancelled — those are terminal states in this scaffold's
	// status machine.
	ErrTerminalStatus = errors.New("domain: cannot transition out of a terminal task status")
	// ErrCannotSetInProgress is returned by SetStatus for a transition into
	// StatusInProgress — that transition is ExecuteTask's job only (it goes
	// through TaskRepository.UpdateStatus directly, not through this
	// method). Letting SetStatus's caller (UpdateTask) double as a
	// completion-callback surface would let a client mark a still-running
	// task done early or fake a dispatch it never made — see TASK-223's
	// Context note.
	ErrCannotSetInProgress = errors.New("domain: cannot set status to in_progress via UpdateTask — only ExecuteTask may transition a task into in_progress")
)

// Task is task-service's central entity — see
// specs/backend-go/services/task-service.md §4 and §5. The proto's Task
// message (id, tenant_id, title, status, parent_id, project_id) is this
// scaffold's authoritative field set; the design doc's broader schema
// (description, complexity, assignee, active_execution_id) is intentionally
// deferred, see this service's README.
type Task struct {
	ID       string
	TenantID string
	Title    string
	Status   string
	// ParentID is empty for a root task. Hierarchy is stored directly on
	// the task row (denormalized) rather than requiring a task_edges
	// parent_child row to exist before GetAncestors can walk it — see
	// adapter/postgres's GetAncestors doc comment.
	ParentID string
	// ProjectID is optional and denotes which project (owned by
	// project-service, a foreign concept this service never validates)
	// this task belongs to. Added for Epic C (backend-go/docs/execution-plan.md)
	// so usecase.HasActiveExecutions can answer "does this project have a
	// task currently in_progress" — see that usecase's doc comment for the
	// honest limit on what "in_progress" currently means here.
	ProjectID string

	Description    string
	Type           string // task|bug|feature|epic
	Priority       string
	AssigneeID     string
	OwnerID        string // see SOL-TG-03 — intrinsic-owner short-circuit
	DueDate        *time.Time
	EstimatedHours *float64
	ActualHours    *float64 // see SOL-TG-04
	PromptTemplate string   // see SOL-TG-02
	AIContext      string
	AIPlanJSON     string // see SOL-TG-02
	Visibility     string
	WorktreeID     string // see SOL-TG-04
	AgentSessionID string // see SOL-TG-04
	// ActiveExecutionID is the complex path's ComplexExecutor.Execute
	// return value (an orchestration-service coordinator_run id) — set
	// right after StartCoordinatorRun succeeds (TASK-TG-04-04), read by
	// ReportTaskExecutionResult (TASK-TG-04-05) to reject a stale/
	// duplicate callback (retried delivery, or a callback for a run this
	// task was re-dispatched away from) rather than erroring on it, per
	// 05-data-architecture.md's at-least-once consumer idempotence note.
	ActiveExecutionID string
	// LastExecutionOutput is this task's most recent successful run's
	// stdout, truncated to 8KB at the application layer before persisting
	// (SOL-TG-04's product-tradeoff decision, see migration 0007's doc
	// comment) — read by a LATER batch wave's buildExecutePrompt
	// (TASK-TG-04-06/07) to resolve `{{outputs.<taskId>.*}}` interpolation
	// against an EARLIER wave's completed dependency.
	LastExecutionOutput string
	ProgressPercent     int
}

func validStatus(s string) bool {
	switch s {
	case StatusOpen, StatusBlocked, StatusInProgress, StatusReview, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}

// NewTask constructs a Task, enforcing the invariants a record must satisfy
// to be meaningful — mirrors usage-service's NewUsageSession pattern
// (invariant-enforcing constructor, not scattered validation in the gRPC
// handler). projectID is optional (may be empty) and carries no validation
// of its own — task-service never validates that a project_id refers to a
// real project-service project, per the bounded-context rule.
func NewTask(id, tenantID, title, status, parentID, projectID string) (Task, error) {
	if tenantID == "" {
		return Task{}, ErrEmptyTenant
	}
	if title == "" {
		return Task{}, ErrEmptyTitle
	}
	if status == "" {
		status = StatusOpen
	}
	if !validStatus(status) {
		return Task{}, ErrInvalidStatus
	}
	if parentID != "" && parentID == id {
		return Task{}, ErrSelfParent
	}
	return Task{ID: id, TenantID: tenantID, Title: title, Status: status, ParentID: parentID, ProjectID: projectID}, nil
}

// SetStatus enforces the (small, currently permissive-by-design) set of
// valid status transitions. Per §4's note that "status transitions [are]
// enforced in methods" — kept simple in this scaffold (any known status to
// any other known status is allowed except leaving a terminal state or
// entering in_progress), since the TS source's exact workflow-status graph
// isn't part of this build task's scope; extend here as more of the state
// machine is ported.
//
// StatusInProgress is deliberately excluded from what this method accepts
// (see ErrCannotSetInProgress) — TASK-223 wires this into UpdateTask, the
// one client-facing status-edit RPC, and a client-driven write must never
// be able to fake or clear a dispatch. ExecuteTask still transitions a task
// into StatusInProgress the way it always has, directly via
// TaskRepository.UpdateStatus, bypassing this method entirely.
func (t Task) SetStatus(status string) (Task, error) {
	if !validStatus(status) {
		return t, ErrInvalidStatus
	}
	if t.Status == StatusDone || t.Status == StatusCancelled {
		return t, ErrTerminalStatus
	}
	if status == StatusInProgress {
		return t, ErrCannotSetInProgress
	}
	t.Status = status
	return t, nil
}
