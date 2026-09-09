# TASK-TG-005-02: Real `ComplexExecutor` (calls `StartCoordinatorRun`) + `ReportTaskExecutionResult` callback RPC

**From Solution:** BE-SOL-005
**Priority:** P1
**Service:** `task-service` (+ calls into `orchestration-service`'s new RPC)
**File:** `backend-go/services/task-service/internal/adapter/grpcclient/complex_executor.go`, `backend-go/proto/orca/task/v1/task.proto` (`ReportTaskExecutionResult` RPC), `backend-go/services/task-service/internal/adapter/grpc/server.go` (handler), `backend-go/services/task-service/internal/usecase/report_task_execution_result.go` (new)
**Depends on:** TASK-TG-004-02 (`StartCoordinatorRun` must exist before `ComplexExecutor` can call it)
**Status:** `[ ]` TODO

---

## Context

Verified directly (`internal/adapter/grpcclient/complex_executor.go:1-26`,
read in full): `StubComplexExecutor.Execute` (lines 24-26) returns
`fmt.Sprintf("stub-orchestration-exec:%s:%s", taskID, requestID)` without
calling anything — a fixed placeholder. Its own doc comment (lines 8-17)
already documents the fix needed: *"Real wiring needs: an
`orchestrationv1.OrchestrationServiceClient` (gRPC) dialed to
`orchestration-service`, handing off to its coordinator... which sequences
subtask/dependency execution... orchestration-service calls back into
task-service to read/update task state as it progresses — that inbound path
is this service's gRPC server (`internal/adapter/grpc`), not this client."*
This task builds exactly that: the outbound client call AND the inbound
callback RPC it needs to actually report back.

**`ReportTaskExecutionResult` does not exist anywhere yet** — confirmed,
`grep -rln "ReportTaskExecutionResult" backend-go/` (excluding this task's
own spec files) returns nothing in either proto or Go source. This is the
same RPC [`docs/crs/v3/flow-task/CR-FLOW-TASK-002`](../../../../../../docs/crs/v3/flow-task/CR-FLOW-TASK-002-workflow-as-task-execution-engine.md)
(§4) independently designs for Engine 3 (Workflow) —
[`specs/backend-go/crs/v3/flow-task/tasks/TASK-FT-002-04`](../../../crs/v3/flow-task/tasks/TASK-FT-002-04-task-service-report-execution-result-usecase.md)
covers that side. **As of this writing, neither series has landed it.**
Whichever of TASK-TG-005-02 or TASK-FT-002-04 is implemented first should
add the RPC with an `engine` field distinguishing the caller
(`"orchestration"` for this task, `"workflow"` for that one) — per
CR-FLOW-TASK-002's own note (quoted in BE-SOL-005): *"if SOL-TG-04 has
added this RPC for Engine 2, Engine 3 TÁI SỬ DỤNG cùng RPC — request thêm
field `engine`... KHÔNG tạo RPC riêng."* **Before writing this task's proto
change, re-check whether TASK-FT-002-04 has landed the RPC already** — if
so, adopt its message shape as-is and only add the `engine="orchestration"`
call site here, do not create a second, colliding
`ReportTaskExecutionResult` definition.

## Changes to make

**1. `internal/adapter/grpcclient/complex_executor.go`** — replace the
stub:

```go
package grpcclient

import (
	"context"
	"fmt"

	orchestrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/orchestration/v1"
)

// ComplexExecutor implements usecase.ComplexExecutor for real — replaces
// StubComplexExecutor (TASK-TG-005-02). Dispatches Execute's complex path
// by calling orchestration-service's StartCoordinatorRun (TASK-TG-004-02).
type ComplexExecutor struct {
	orchestration orchestrationv1.OrchestrationServiceClient
}

func NewComplexExecutor(orchestration orchestrationv1.OrchestrationServiceClient) *ComplexExecutor {
	return &ComplexExecutor{orchestration: orchestration}
}

func (e *ComplexExecutor) Execute(ctx context.Context, tenantID, taskID, requestID, prompt string) (string, error) {
	run, err := e.orchestration.StartCoordinatorRun(ctx, &orchestrationv1.StartCoordinatorRunRequest{
		OriginTaskId: taskID, CoordinatorHandle: fmt.Sprintf("task-%s", taskID),
	})
	if err != nil {
		return "", fmt.Errorf("complex_executor: start_coordinator_run: %w", err)
	}
	return run.GetId(), nil
}
```

`prompt`/`requestID` aren't forwarded to `StartCoordinatorRunRequest` in
this sketch — `StartCoordinatorRunRequest` (TASK-TG-004-02) has no `prompt`
field. Decide whether the prompt override needs to flow into the
`spec_json` field (`StartCoordinatorRunRequest.spec_json`,
TASK-TG-004-02) so the coordinator's eventual dispatch step
(TASK-TG-004-03's deferred dispatch call) has access to it — if
`orchestration-service`'s coordinator needs the prompt to actually run
anything useful, this is a real gap to close here, not a follow-up; check
`orchestration-service.md`'s `spec` field semantics before deciding.

**2. `cmd/server/main.go`** — dial `orchestration-service` (mirror
whatever dial pattern TASK-TG-003-01 used for `tenant-service`, or an
existing precedent if `orchestration-service` is already dialed for
another purpose — check first), replace `NewStubComplexExecutor()`'s wiring
with `NewComplexExecutor(orchestrationClient)`.

**3. `task.proto`** — service-to-service RPC (add to `TaskService`, since
`orchestration-service` calls back INTO `task-service`, per the doc
comment's "that inbound path is this service's gRPC server" framing):

```protobuf
rpc ReportTaskExecutionResult(ReportTaskExecutionResultRequest) returns (google.protobuf.Empty);

message ReportTaskExecutionResultRequest {
  string task_id = 1;
  string engine = 2; // "orchestration" (this task) | "workflow" (CR-FLOW-TASK-002/TASK-FT-002-04)
  bool success = 3;
  string error_message = 4; // empty if success
}
```

**4. `internal/usecase/report_task_execution_result.go`** (new):

```go
type ReportTaskExecutionResultInput struct {
	TaskID       string
	Engine       string
	Success      bool
	ErrorMessage string
}

func (uc *ReportTaskExecutionResult) Execute(ctx context.Context, in ReportTaskExecutionResultInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	newStatus := domain.StatusDone
	if !in.Success {
		newStatus = domain.StatusBlocked // matches TASK-TG-005-01's dispatch-failure revert target
	}
	if err := uc.repo.UpdateStatus(ctx, tenantID, in.TaskID, newStatus); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_REPORT_EXECUTION_RESULT_FAILED", "failed to persist execution result", err)
	}
	// If the completed task has a non-empty ParentID, cascade via
	// RecalculateProgress (TASK-TG-001-03) — this is the actual completion
	// callback TASK-TG-001-03's Context flags as a near-term follow-up once
	// one exists. Wire it here.
	return nil
}
```

**5. `orchestration-service`'s side** (TASK-TG-004-03's scope, not this
task's — cross-reference only): once `AdvancePendingRuns` detects a
`CoordinatorRun`'s root task reaching a terminal state, it should call this
RPC back into `task-service` with `engine="orchestration"`. Note this
dependency explicitly in TASK-TG-004-03's own tracking if that task lands
after this one.

## Test plan

- `ComplexExecutor.Execute` calls `StartCoordinatorRun` with the right
  `OriginTaskId`; no more `"stub-orchestration-exec:"` string anywhere in
  the codebase (grep assertion in CI, per BE-SOL-005's own test plan item).
- `ReportTaskExecutionResult` with `success=true` transitions the task to
  `done`; `success=false` transitions it to `blocked`.
- `ReportTaskExecutionResult` with an unrecognized `engine` value — decide
  whether to reject or accept-and-log; BE-SOL-005 doesn't specify, default
  to accept-and-log (forward-compatible with a future Engine 4) unless
  product wants strict validation.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/... ./services/orchestration-service/...
go test ./services/task-service/internal/adapter/grpcclient/... -run TestComplexExecutor -v
go test ./services/task-service/internal/usecase/... -run "TestReportTaskExecutionResult|TestExecuteTask" -v
grep -rn "stub-orchestration-exec" backend-go/ && exit 1 || echo "no stub references remain"
```

Expected: clean build across both services; the `grep` check for the stub
string returns nothing; `ReportTaskExecutionResult` test asserts the
correct status transition for both success and failure.
