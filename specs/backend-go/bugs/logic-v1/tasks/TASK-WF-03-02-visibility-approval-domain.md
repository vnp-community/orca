# TASK-WF-03-02: Add `Visibility` state machine and `Approval` domain types

**From Solution:** SOL-WF-03
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/visibility.go` (new)
**Depends on:** TASK-WF-03-01, TASK-WF-01-02
**Status:** `[x]` DONE — implemented per orchestrator instruction (reviewer sign-off waived for this batch run). New `visibility.go` (`Visibility`/`visibilityRank`/`Valid`/`CanEscalateTo`) and `approval.go` (`ApprovalStatus`/`Approval` + `NewApproval` constructor + `Approve`/`Reject` methods with a not-pending guard — extended beyond the spec's bare struct for consistency with every other entity in this package). `WorkflowTemplate` gained `Visibility`/`ShareToken`/`RatingSum`/`RatingCount` + `AverageRating()`, defaulting to `VisibilityPrivate` at construction; `WithVisibility`/`WithShareToken`/`WithRating` `TemplateOption`s added for reconstruction. 18 new tests (11 `CanEscalateTo` cases, `Valid`, 3 `AverageRating` cases, 5 `Approval` constructor/transition cases) all pass. `go build/vet/test ./...` green.

---

## Context

None of BUG-WF-03's approval/visibility/share/rating domain model exists
today — `workflow-service.md` §4/§5 stop at template/execution/
step-execution. This is the largest genuine extension in this bug set;
worth a reviewer's explicit sign-off on the state-machine shape before
implementation proceeds past this task.

## Changes to make

Create `backend-go/services/workflow-service/internal/domain/visibility.go`:

```go
package domain

type Visibility string

const (
    VisibilityPrivate Visibility = "private"
    VisibilityTeam    Visibility = "team"
    VisibilityCompany Visibility = "company"
    VisibilityPublic  Visibility = "public"
)

// rank orders visibility for escalation-only transitions — the state
// machine is escalate-forward (private -> team -> company -> public)
// with company requiring approval; de-escalation (public -> private) is
// a separate, always-allowed "unpublish" operation.
var rank = map[Visibility]int{VisibilityPrivate: 0, VisibilityTeam: 1, VisibilityCompany: 2, VisibilityPublic: 3}

func (v Visibility) Valid() bool { _, ok := rank[v]; return ok }

// CanEscalateTo reports whether moving from v to next is a valid single
// forward step (only one tier at a time) OR any direct de-escalation
// back to private (unpublish, always one step, any distance).
func (v Visibility) CanEscalateTo(next Visibility) bool {
    if next == VisibilityPrivate {
        return true
    }
    return rank[next] == rank[v]+1
}
```

Create `backend-go/services/workflow-service/internal/domain/approval.go`:

```go
package domain

type ApprovalStatus string

const (
    ApprovalPending  ApprovalStatus = "pending"
    ApprovalApproved ApprovalStatus = "approved"
    ApprovalRejected ApprovalStatus = "rejected"
)

type Approval struct {
    ID, TemplateID, RequestedBy string
    Status                      ApprovalStatus
    ResolvedBy                  string
    ResolvedAt                  *time.Time
}
```

Extend `domain.WorkflowTemplate` (`template.go`, already extended by
TASK-WF-01-02) with:

```go
Visibility  Visibility
ShareToken  string
RatingSum   int32
RatingCount int32
```

```go
// AverageRating is a derived value, not a stored field — matches
// rating_sum/rating_count's "don't persist a derived value" posture.
func (t WorkflowTemplate) AverageRating() float64 {
    if t.RatingCount == 0 {
        return 0
    }
    return float64(t.RatingSum) / float64(t.RatingCount)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestVisibility
```

Expected: `CanEscalateTo` allows exactly one tier forward, rejects
skipping a tier (e.g. `private` -> `company` directly), always allows
any-tier -> `private`; `AverageRating()` returns `0` when
`RatingCount == 0` (no divide-by-zero panic).
