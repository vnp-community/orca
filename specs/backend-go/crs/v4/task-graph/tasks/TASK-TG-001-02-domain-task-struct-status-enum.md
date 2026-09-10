# TASK-TG-001-02: Widen `domain.Task` struct, introduce `domain.Status` type + 3 new status values

**From Solution:** BE-SOL-001
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/task.go`, `backend-go/services/task-service/internal/usecase/ports.go`, `backend-go/services/task-service/internal/adapter/postgres/repository.go`, `backend-go/services/task-service/internal/adapter/grpc/server.go`, `backend-go/proto/orca/task/v1/task.proto`
**Depends on:** TASK-TG-001-01 (migration must land first — this task's `Get`/`Create`/`Update` SQL reads/writes the new columns)
**Status:** `[ ]` TODO

---

## Context

Verified directly (`backend-go/services/task-service/internal/domain/task.go:1-137`,
read in full): the real `Task` struct is 52-75 lines, `Status` is a plain
**`string`** field (not a named type), and the only 4 status constants
(`StatusOpen`/`StatusInProgress`/`StatusDone`/`StatusCancelled`) live at
lines 13-16 as untyped `string` consts. `validStatus(s string) bool` is at
line 77, `NewTask` at line 92, `SetStatus` at line 125.

**Grounding correction — this is a bigger blast radius than BE-SOL-001's
isolated snippet shows.** BE-SOL-001's "Design — `domain.Task` struct +
`Status` enum" section introduces `type Status string` and gives `Task.Status`
that new type, but every current call site treats `Task.Status`/status
parameters as a bare `string`:

- `usecase/ports.go:17-47`'s `TaskRepository.UpdateStatus(ctx, tenantID, id,
  status string) error` — takes `string`.
- `adapter/postgres/repository.go:149`'s `UpdateStatus` implementation and
  `repository.go:69-95,101-143`'s `Create`/`Get`/`GetAncestors` all
  `Scan`/bind `t.Status` as a `string` column value.
- `adapter/grpc/server.go:294`'s `toProtoTask` and the (not yet located,
  search before editing) `fromProto`-style status assignment both pass
  `domain.Task.Status` straight into/out of `taskv1.Task.status` (a proto
  `string` field, `task.proto:52`) with no conversion today.
- `usecase/create_task.go:46`, `usecase/execute_task.go:62`,
  `usecase/add_edge.go` (TASK-TG-001-04) all call `domain.NewTask(...,
  domain.StatusOpen, ...)` / `uc.repo.UpdateStatus(ctx, tenantID, id,
  domain.StatusInProgress)` — these keep compiling unchanged as long as
  `domain.Status` is a `string`-based defined type and the constants stay
  typed `Status` (Go allows an untyped/defined-type constant to satisfy a
  `string`-typed parameter only if the parameter itself is also updated to
  `domain.Status` — a plain `string` parameter does NOT accept a
  `domain.Status` argument without an explicit conversion).

**Decide and do one of the following, not silently pick whichever compiles
first**: either (a) widen `TaskRepository.UpdateStatus`'s and every SQL
scan/bind site's status parameter from `string` to `domain.Status` throughout
`ports.go`/`repository.go`/`server.go` (recommended — keeps the type safety
`Status` is meant to add), or (b) keep `Task.Status` and all port signatures
as `string` and add `domain.Status` purely as a set of typed constants used
only where BE-SOL-001 already sketches it (`RecalculateProgress`,
`AddEdge`'s auto-block target). Option (a) is what BE-SOL-001's own struct
sketch implies (`Status Status` field) — implement that, and grep
`\.Status\b` and `status string` across `internal/` before considering this
task done, not just the files BE-SOL-001 names.

## Changes to make

**1. `internal/domain/task.go`** — add `type Status string`, the 3 new
consts, widen `Task`, `validStatus`, `NewTask`, `SetStatus`:

```go
type Status string

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

func validStatus(s Status) bool {
	switch s {
	case StatusBacklog, StatusTodo, StatusOpen, StatusInProgress, StatusBlocked, StatusReview, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}
```

(`StatusOpen` is kept, unlike BE-SOL-001's list which drops it silently in
its enum block but keeps it in the migration's CHECK constraint for backward
compat — this task keeps it as a first-class Go value too, since existing
rows/tests use it and TASK-TG-001-01's migration explicitly does not
backfill them away.)

`Task` struct gains BE-SOL-001's ~18 new fields (`Description`, `Type`,
`Priority`, `Labels []string`, `AssigneeID`/`ReporterID`/`OwnerID *string`,
`DueDate *time.Time`, `EstimatedHours`/`ActualHours *float64`,
`PromptTemplate`, `AIContext`, `AIPlanJSON json.RawMessage`, `Visibility`,
`WorktreeID`/`AgentSessionID`/`WorkflowExecID *string`,
`DoneSubtasks`/`TotalSubtasks int`) alongside the existing `ID, TenantID,
Title, Status, ParentID, ProjectID, WorkflowTemplateID` fields (the last one
added by `0003_task_workflow_template_id`, keep it — BE-SOL-001's own struct
sketch omits it, an oversight versus the real current struct, not an
intentional removal).

`NewTask`/`SetStatus` signatures change their `status`/`Status` parameters
from `string` to `Status` — update every caller in the same PR (see Context).

**2. `usecase/ports.go`** — `TaskRepository.UpdateStatus`'s `status string`
param becomes `status domain.Status` (line 30 today).

**3. `adapter/postgres/repository.go`** — `Create` (line 69), `Get` (line
80), `GetAncestors` (line 101), `Update`, `UpdateStatus` (line 149) all
widen their INSERT/SELECT column lists to the new columns and their
Scan/bind targets from `string` to `domain.Status` for the status column;
add the new columns to `Create`'s `INSERT INTO task.tasks (...)` list and
`Get`/`GetAncestors`'s `SELECT` column lists, using `COALESCE(x::text, '')`
for the new nullable pointer-backed fields the same way `parent_id`/
`project_id`/`workflow_template_id` are already handled at line 82.

**4. `adapter/grpc/server.go`** — `toProtoTask` (line 294) and the
request-to-domain mapping in `CreateTask`/`UpdateTask` handlers gain the new
fields; `Task.status` stays a wire `string` (protobuf has no enum-per-se
requirement here, matching the existing convention — BE-SOL-001 does not
propose a proto enum for status either).

**5. `task.proto`** — `message Task` currently has 7 fields
(`id=1..workflow_template_id=7`, confirmed by direct read of
`task.proto:49-58`). Append the ~18 new fields starting at `field 8`
sequentially (`description=8`, `type=9`, ... `total_subtasks=26`) — do not
reuse BE-SOL-003's arbitrarily-numbered `share_token = 20` from its own
sketch; that field must instead be the NEXT free number after this task's
last one (see TASK-TG-003-05's Context, which is corrected accordingly).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/domain/... -v
go test ./services/task-service/internal/usecase/... -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build across `task-service` (a status-type mismatch anywhere
in `internal/` fails the build immediately, which is the point of widening
the type — treat any remaining `string`/`domain.Status` mismatch as this
task's own scope, not a follow-up); `Status` enum table test covers all 8
values accepted, anything else rejected; existing `NewTask`/`SetStatus` unit
tests pass unchanged in behavior (only the parameter type changed).
