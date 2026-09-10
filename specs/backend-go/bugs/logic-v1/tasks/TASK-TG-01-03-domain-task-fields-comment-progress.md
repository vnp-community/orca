# TASK-TG-01-03: Domain — widen `Task`, add `StatusBlocked`/`StatusReview`, `TaskComment`, `CalculateProgress`

**From Solution:** SOL-TG-01
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/task.go`
**Depends on:** none (pure domain, no proto/DB dependency to compile)
**Status:** `[x]` DONE — domain.Task widened, StatusBlocked/StatusReview added, TaskComment + CalculateProgress created; go test ./internal/domain/... passes (new task_test.go/task_comment_test.go/progress_test.go cases).

---

## Context

`domain.Task` only has the scaffold's original 6 fields; `NewTask`'s
invariants must stay backward compatible (every existing caller/test sets
only `Title`/`ParentID`/`ProjectID`). This task adds the new fields as
optional (zero-value-valid), the two new status constants, a new
`TaskComment` entity mirroring `task_edge.go`'s minimal-invariant shape, and
the pure `CalculateProgress` function `RecalculateProgress` will call.

## Changes to make

In `backend-go/services/task-service/internal/domain/task.go`:

```go
const (
	StatusOpen       = "open"
	StatusBlocked    = "blocked"     // new — see TASK-TG-01-07's auto-block design
	StatusInProgress = "in_progress"
	StatusReview     = "review"      // new — SOL-TG-04's completion target
	StatusDone       = "done"
	StatusCancelled  = "cancelled"
)
```

Update `validStatus` to include the two new values:

```go
func validStatus(s string) bool {
	switch s {
	case StatusOpen, StatusBlocked, StatusInProgress, StatusReview, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}
```

Widen the `Task` struct (append after the existing 6 fields, all optional —
`NewTask`'s signature and invariants are unchanged):

```go
type Task struct {
	ID       string
	TenantID string
	Title    string
	Status   string
	ParentID string
	ProjectID string

	Description     string
	Type            string // task|bug|feature|epic
	Priority        string
	AssigneeID      string
	OwnerID         string // see SOL-TG-03 — intrinsic-owner short-circuit
	DueDate         *time.Time
	EstimatedHours  *float64
	ActualHours     *float64 // see SOL-TG-04
	PromptTemplate  string   // see SOL-TG-02
	AIContext       string
	AIPlanJSON      string   // see SOL-TG-02
	Visibility      string
	WorktreeID      string   // see SOL-TG-04
	AgentSessionID  string   // see SOL-TG-04
	ProgressPercent int
}
```

Add `"time"` to the file's imports.

Extend `SetStatus`'s transition matrix — `StatusBlocked`/`StatusReview` are
allowed like any other non-terminal, non-`in_progress` status (the doc
comment's existing "any known status to any other known status is allowed
except leaving a terminal state or entering in_progress" rule already covers
them; no code change needed beyond `validStatus` above accepting them).

Create `backend-go/services/task-service/internal/domain/task_comment.go`:

```go
package domain

import (
	"errors"
	"time"
)

// TaskComment is one task.task_comments row — mirrors task_edge.go's
// minimal-invariant-constructor shape. See SOL-TG-01's AddComment/
// ListComments design.
type TaskComment struct {
	ID        string
	TaskID    string
	AuthorID  string
	Content   string
	CreatedAt time.Time
}

// ErrEmptyCommentBody guards against a content-less comment.
var ErrEmptyCommentBody = errors.New("domain: comment content is required")

// NewTaskComment constructs a TaskComment, enforcing the one invariant that
// matters for a comment: non-empty content.
func NewTaskComment(id, taskID, authorID, content string) (TaskComment, error) {
	if content == "" {
		return TaskComment{}, ErrEmptyCommentBody
	}
	return TaskComment{ID: id, TaskID: taskID, AuthorID: authorID, Content: content}, nil
}
```

Create `backend-go/services/task-service/internal/domain/progress.go`:

```go
package domain

// CalculateProgress computes, for one task, the percentage of its direct
// children marked Done — 100 for a leaf task whose OWN status is Done, 0
// for a leaf that isn't, and the average of children's own (already
// recursively computed) percentages for a task with children. The caller
// (usecase.RecalculateProgress) walks bottom-up over a subtree fetched via
// GetSubtree, calling this once per task in post-order. Pure function: no
// DB, no context.Context — same discipline as DetectCycle/ResolveGrant.
func CalculateProgress(task Task, childPercents []int) int {
	if len(childPercents) == 0 {
		if task.Status == StatusDone {
			return 100
		}
		return 0
	}
	sum := 0
	for _, p := range childPercents {
		sum += p
	}
	return sum / len(childPercents)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/internal/domain/...
go test ./services/task-service/internal/domain/... -run 'TestNewTask|TestSetStatus|TestNewTaskComment|TestCalculateProgress' -v
```

Expected: package builds; add/extend `task_test.go` with
`StatusBlocked`/`StatusReview` cases in the transition matrix, a new
`task_comment_test.go` (empty-content rejection), and a new
`progress_test.go` (leaf Done→100, leaf not-Done→0, uniform children
average, mixed-depth cascade) alongside this change — all pass.
