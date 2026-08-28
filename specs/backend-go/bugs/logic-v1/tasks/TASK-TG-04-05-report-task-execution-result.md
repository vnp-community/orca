# TASK-TG-04-05: `ReportTaskExecutionResult` — inbound completion callback for the complex path

**From Solution:** SOL-TG-04
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`, `backend-go/services/task-service/internal/usecase/report_execution_result.go` (new)
**Depends on:** TASK-TG-04-04 (`coordinator_run_id`/`ActiveExecutionID` produced by `ComplexExecutor`)
**Status:** `[ ]` TODO

---

## Context

`orchestration-service.md §2.2`: "`task-service` calls in to *start* a run
and *read* its terminal result; it never touches this service's tables
directly," matched by `orch --> task` in the dependency graph. This RPC is
that callback — `orchestration-service` calls it when a `coordinator_run`
reaches a terminal state. `Task` needs an `ActiveExecutionID` field to make
a staleness check possible (a retried/duplicate callback, or a callback for
a run this task was re-dispatched away from, must be ignored, not an
error — at-least-once consumer idempotence per
`05-data-architecture.md`'s outbox-consumer note).

**Security note**: this RPC is called by `orchestration-service`, never a
browser/mobile client — `api-gateway` never routes to it. The handler must
validate the calling service's mesh identity is `orchestration-service`
specifically, via whatever `common/grpcmw` interceptor already extracts
service identity from mTLS for other internal-only RPCs in this codebase —
confirm the exact interceptor at implementation time rather than guessing.

## Changes to make

Add `ActiveExecutionID` to `Task` — a small additive migration
(`0005`, after `TASK-TG-03-05`'s `0004`):

`backend-go/services/task-service/migrations/0005_active_execution_id.up.sql`:

```sql
ALTER TABLE task.tasks ADD COLUMN active_execution_id TEXT;
```

`backend-go/services/task-service/migrations/0005_active_execution_id.down.sql`:

```sql
ALTER TABLE task.tasks DROP COLUMN active_execution_id;
```

Add `ActiveExecutionID string` to `domain.Task` (alongside `TASK-TG-01-03`'s
other new fields), and widen `taskColumns`/`scanTask`/`Update` in
`repository.go` (from `TASK-TG-01-04`) to include it. Set it in
`ComplexExecutor.Execute` (`TASK-TG-04-04`) right after `StartCoordinatorRun`
succeeds, via a new `UpdateActiveExecutionID` repo method (same one-column
setter shape as `UpdateWorktreeID`).

Add to `task.proto`'s `TaskService` service block:

```protobuf
  // ReportTaskExecutionResult is called BY orchestration-service only — see
  // this RPC's usecase doc comment for the service-identity check this
  // handler must perform. api-gateway never routes to it.
  rpc ReportTaskExecutionResult(ReportTaskExecutionResultRequest) returns (google.protobuf.Empty);
```

```protobuf
message ReportTaskExecutionResultRequest {
  string task_id = 1;
  string coordinator_run_id = 2; // must match the task's current active_execution_id
  bool success = 3;
  double actual_hours = 4;
  string error_message = 5; // set iff !success
}
```

Create `backend-go/services/task-service/internal/usecase/report_execution_result.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ReportTaskExecutionResultInput struct {
	TaskID           string
	CoordinatorRunID string
	Success          bool
	ActualHours      float64
}

type ReportTaskExecutionResult struct {
	tasks TaskRepository
}

func NewReportTaskExecutionResult(tasks TaskRepository) *ReportTaskExecutionResult {
	return &ReportTaskExecutionResult{tasks: tasks}
}

func (uc *ReportTaskExecutionResult) Execute(ctx context.Context, in ReportTaskExecutionResultInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	task, err := uc.tasks.Get(ctx, tenantID, in.TaskID)
	if err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found", err)
	}
	if task.ActiveExecutionID != in.CoordinatorRunID {
		// Stale/duplicate callback (retried delivery, or a callback for a
		// run this task was re-dispatched away from) — ignored, not an
		// error, per at-least-once consumer idempotence.
		return nil
	}
	if in.Success {
		return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusReview, in.ActualHours)
	}
	// Failed complex execution -> Blocked, not silently left in_progress or
	// auto-Done.
	return uc.tasks.CompleteExecution(ctx, tenantID, in.TaskID, domain.StatusBlocked, in.ActualHours)
}
```

Add the gRPC handler in
`backend-go/services/task-service/internal/adapter/grpc/server.go`:

```go
func (s *Server) ReportTaskExecutionResult(ctx context.Context, req *taskv1.ReportTaskExecutionResultRequest) (*emptypb.Empty, error) {
	// Service-identity check: this RPC must only be callable BY
	// orchestration-service (mTLS mesh identity), never api-gateway/a user
	// session — confirm the exact common/grpcmw interceptor call at
	// implementation time; a placeholder guard is intentionally NOT
	// substituted here since asserting the wrong thing would be worse than
	// flagging it as an open wiring detail.
	if err := s.reportExecutionResult.Execute(ctx, usecase.ReportTaskExecutionResultInput{
		TaskID: req.GetTaskId(), CoordinatorRunID: req.GetCoordinatorRunId(),
		Success: req.GetSuccess(), ActualHours: req.GetActualHours(),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./...
go test ./services/task-service/internal/usecase/... -run TestReportTaskExecutionResult -v
```

Expected: a callback whose `coordinator_run_id` doesn't match the task's
current `ActiveExecutionID` is a silent no-op, not an error; success →
`StatusReview`; failure → `StatusBlocked`, never a status revert to the
pre-dispatch value (that's the simple path's failure semantics from
`TASK-TG-04-01`, not the complex path's).
