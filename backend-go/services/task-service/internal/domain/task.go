// Package domain holds task-service's entities and pure domain services. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, no context.Context.
package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// Status is a task's lifecycle state. Widened from a plain string
// (BE-SOL-001) to a defined type so status parameters/fields get compile-time
// type safety throughout ports.go/repository.go/server.go, rather than only
// where this package itself uses status values.
type Status string

// Status values a Task can hold. StatusOpen is kept as a first-class value
// alongside the 3 new BE-SOL-001 states (Backlog/Todo dropped from the
// enum's "current" set, Blocked/Review added) because existing rows/tests
// use it and TASK-TG-001-01's migration explicitly does not backfill them
// away — see that migration's up.sql comment.
const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusReview     Status = "review"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
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
// specs/backend-go/services/task-service.md §4 and §5. Widened by
// TASK-TG-001-02/BE-SOL-001 with the fields the AI-decompose, access-control,
// and orchestration solutions need (description, classification, assignment,
// ownership, estimates, AI context/plan, visibility, execution tracking,
// subtask progress counters) alongside the original proto-backed field set.
type Task struct {
	ID       string
	TenantID string
	Title    string
	Status   Status
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
	// WorkflowTemplateID optionally attaches a workflow-service template to
	// this task (Engine 3, CR-FLOW-TASK-002) — set only via UpdateTask
	// (AttachWorkflowTemplateAction.tsx), never at creation. No FK: lives in
	// workflow-service's own database, same cross-service-reference
	// convention as ProjectID. See docs/backlog/BACKLOG-016.
	WorkflowTemplateID string

	// Description is free-form task detail beyond Title.
	Description string
	// Type classifies the task (e.g. "feature", "bug", "chore") — an
	// open string, not a proto enum, matching Status's own convention.
	Type string
	// Priority is an open string (e.g. "low"/"medium"/"high"/"urgent").
	Priority string
	// Labels are free-form tags.
	Labels []string
	// AssigneeID/ReporterID/OwnerID are logical FKs into tenant-service's
	// user set — task-service never validates them, same bounded-context
	// rule as ProjectID. OwnerID is nil for tasks created before
	// TASK-TG-001-01's migration (no backfill) — see TASK-TG-003-02 for how
	// a nil OwnerID is treated as "no intrinsic owner" by grant resolution.
	AssigneeID *string
	ReporterID *string
	OwnerID    *string
	// DueDate is optional.
	DueDate *time.Time
	// EstimatedHours/ActualHours are optional effort tracking fields.
	EstimatedHours *float64
	ActualHours    *float64
	// PromptTemplate overrides the default agent prompt built from the task
	// (see TASK-TG-005-03's buildExecutePrompt).
	PromptTemplate string
	// AIContext carries the 5-source context bundle TASK-TG-002-01 builds
	// (repo/tech-stack/sibling-task/parent-task/comment context) as opaque
	// JSON — this package never interprets its shape.
	AIContext json.RawMessage
	// AIPlanJSON is the structured decomposition plan AIApply persists
	// (TASK-TG-002-02) — also opaque JSON at this layer.
	AIPlanJSON json.RawMessage
	// Visibility gates whether a task is visible beyond its explicit grants
	// (e.g. "private"/"team"/"public") — defaults to "private" at the
	// database layer (migration DEFAULT), not re-defaulted here so a
	// zero-value Task loaded from a pre-migration row still round-trips
	// whatever the column actually holds.
	Visibility string
	// WorktreeID/AgentSessionID/WorkflowExecID track the execution
	// environment a dispatched task is running in/under — set by the
	// execution usecases (TASK-TG-005-*), never by CreateTask/UpdateTask.
	WorktreeID     *string
	AgentSessionID *string
	WorkflowExecID *string
	// DoneSubtasks/TotalSubtasks are progress-cascade counters maintained by
	// RecalculateProgress (TASK-TG-001-03) — never written directly by
	// CreateTask/UpdateTask.
	DoneSubtasks  int
	TotalSubtasks int
	// ShareToken is the public share-link lookup key (TASK-TG-003-05,
	// SECURITY REVIEW REQUIRED before merge) — nil until GenerateShareLink
	// mints one. A cryptographically random value (crypto/rand, never a
	// UUID derived from the task ID or any other guessable value) — see
	// usecase.GenerateShareLink's doc comment for why. Never exposed via
	// the public GetTaskByShareToken path itself (that returns the
	// dedicated, narrower TaskShareView) — only via authenticated reads of
	// the full Task (GetTask/ListTasks), so an admin can retrieve/share it.
	ShareToken *string
}

func validStatus(s Status) bool {
	switch s {
	case StatusBacklog, StatusTodo, StatusOpen, StatusInProgress, StatusBlocked, StatusReview, StatusDone, StatusCancelled:
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
func NewTask(id, tenantID, title string, status Status, parentID, projectID string) (Task, error) {
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
func (t Task) SetStatus(status Status) (Task, error) {
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
