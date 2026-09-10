// Package domain holds task-service's entities and pure domain services. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, no context.Context.
package domain

import (
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
// Explicitly typed as Status (not left as untyped string constants) so
// `status := domain.StatusDone`-style short variable declarations infer
// domain.Status, not string — load-bearing for every call site that takes
// its address (e.g. UpdateTaskInput.Status *domain.Status).
const (
	StatusOpen       Status = "open"
	StatusBlocked    Status = "blocked" // new — see TASK-TG-01-07's auto-block design
	StatusInProgress Status = "in_progress"
	// StatusReview is the simple execution path's terminal status (SOL-TG-04)
	// — ExecuteTask.CompleteExecution lands a successful simple-path run here
	// directly, inline, rather than leaving it stuck in_progress. See
	// usecase.ExecuteTask's doc comment.
	StatusReview    Status = "review"
	StatusDone      Status = "done"
	StatusCancelled Status = "cancelled"
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
	// WorktreeID is the git-gateway-service/project-service worktree this
	// task executes against — empty until ExecuteTask's WorktreeProvisioner
	// first provisions or reuses one (SOL-TG-04); also mirrors
	// project-service's Worktree.TaskID (SOL-PW-04). Once set, later Execute
	// calls reuse the same worktree rather than creating a new one each time.
	WorktreeID     string
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
	// TaskNumber is a per-project sequential number (immutable, assigned
	// once at Create) so a commit message can reference "#TG-42" without
	// embedding a UUID — added SOL-PW-04.
	TaskNumber int64
	// PRURL is set by the PR-creation write-back saga — empty until a PR
	// referencing this task's #TG-N is created. Added SOL-PW-04.
	PRURL string
	// WorkflowTemplateID, when set, attaches a workflow-service template to
	// this task and routes ExecuteTask's dispatch to Engine 3 (EngineWorkflow,
	// CR-FLOW-TASK-002) ahead of the subtask/dependency check — see
	// selectEngine's priority rule. Empty means "no workflow engine
	// selected", the same default every existing task effectively has today.
	// Set only via UpdateTask (AttachWorkflowTemplateAction.tsx), never at
	// creation. No FK: lives in workflow-service's own database, same
	// cross-service-reference convention as ProjectID. Backed by
	// task.tasks.workflow_template_id (0004_execution_links migration,
	// BE-SOL-001). See docs/backlog/BACKLOG-016.
	WorkflowTemplateID string
	// ActiveExecutionLinkID points at the execution_links row (TASK-FT-001-01)
	// created by the Execute call currently (or most recently) dispatched for
	// this task — empty if the task has never been dispatched. Backed by
	// task.tasks.active_execution_link_id (0004_execution_links migration).
	// ReportTaskExecutionResult (TASK-FT-002-04) compares an inbound
	// callback's execution_ref/engine against this link before accepting it,
	// so a stale/duplicate callback (e.g. for a task re-dispatched since) is
	// a no-op rather than corrupting a newer run's state.
	ActiveExecutionLinkID string

	// Labels are free-form tags (TASK-TG-001-02's widening) — added to the
	// proto at the next-free field number (25), since the task's own
	// self-assigned numbering collided with fields main had already claimed.
	Labels []string
	// ReporterID is a logical FK into tenant-service's user set, same
	// bounded-context rule as AssigneeID/OwnerID — empty means unset.
	ReporterID string
	// WorkflowExecID is the execution-tracking counterpart to
	// WorkflowTemplateID above — set once Engine 3 dispatch starts.
	WorkflowExecID string
	// DoneSubtasks/TotalSubtasks are progress-cascade counters maintained by
	// RecalculateProgress — never written directly by CreateTask/UpdateTask.
	DoneSubtasks  int
	TotalSubtasks int
	// ShareToken is the public share-link lookup key (TASK-TG-003-05,
	// SECURITY REVIEW REQUIRED before merge) — empty until GenerateShareLink
	// mints one. A cryptographically random value (crypto/rand, never a UUID
	// derived from the task ID or any other guessable value) — see
	// usecase.GenerateShareLink's doc comment for why. Never exposed via the
	// public GetTaskByShareToken path itself (that returns the dedicated,
	// narrower TaskShareView) — only via authenticated reads of the full
	// Task (GetTask/ListTasks), so an admin can retrieve/share it.
	ShareToken string
}

func validStatus(s Status) bool {
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
