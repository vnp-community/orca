// Package domain holds task-service's entities and pure domain services. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, no context.Context.
package domain

import "errors"

// Status values a Task can hold. Kept as an open-ish small set matching the
// TS source's status strings (see specs/backend-go/services/task-service.md
// §10 — faithful port, not a redesign).
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
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
)

// Task is task-service's central entity — see
// specs/backend-go/services/task-service.md §4 and §5. The proto's Task
// message (id, tenant_id, title, status, parent_id) is this scaffold's
// authoritative field set; the design doc's broader schema (description,
// complexity, assignee, active_execution_id) is intentionally deferred, see
// this service's README.
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
}

func validStatus(s string) bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}

// NewTask constructs a Task, enforcing the invariants a record must satisfy
// to be meaningful — mirrors usage-service's NewUsageSession pattern
// (invariant-enforcing constructor, not scattered validation in the gRPC
// handler).
func NewTask(id, tenantID, title, status, parentID string) (Task, error) {
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
	return Task{ID: id, TenantID: tenantID, Title: title, Status: status, ParentID: parentID}, nil
}

// SetStatus enforces the (small, currently permissive-by-design) set of
// valid status transitions. Per §4's note that "status transitions [are]
// enforced in methods" — kept simple in this scaffold (any known status to
// any other known status is allowed except leaving a terminal state), since
// the TS source's exact workflow-status graph isn't part of this build
// task's scope; extend here as more of the state machine is ported.
func (t Task) SetStatus(status string) (Task, error) {
	if !validStatus(status) {
		return t, ErrInvalidStatus
	}
	if t.Status == StatusDone || t.Status == StatusCancelled {
		return t, ErrTerminalStatus
	}
	t.Status = status
	return t, nil
}
