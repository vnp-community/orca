# TASK-110: Add `GetDispatchContextForTask` read RPC to `orchestration-service`

**From Solution:** SOL-018
**Priority:** P1
**Service:** `orchestration-service`
**File:** `backend-go/proto/orca/orchestration/v1/orchestration.proto`, `services/orchestration-service/internal/{usecase,adapter/postgres,adapter/grpc}/*.go`
**Depends on:** none
**Status:** `[x]` DONE (verified) — `GetDispatchContextForTask` RPC + messages added to orchestration.proto (buf breaking clean), `DispatchContextRepository.GetLatestForTask` port + postgres implementation (ORDER BY created_at DESC LIMIT 1), usecase, grpc/server.go wiring, main.go wiring. `go build`/`go vet`/`go test` clean.

---

## Context

BUG-018 reports two things; SOL-018 finds the first is not actually a bug
(`DispatchContext.handle` already IS the assignee handle — TASK-111
resolves the naming gap at the `wscompat` translation boundary, not here)
and the second is real: `orchestration.proto`'s 4 RPCs are all writes,
there is no read RPC at all, not even in `orchestration-service.md` §3's
own drafted surface. This task adds `GetDispatchContextForTask` — flagged
by SOL-018 as a scope addition beyond the TDD, the same category as
SOL-015's `ListPriorities`/`ListTransitions`/`GetProjectStatusOrder`.

`dispatch_contexts.orchestration_task_id` is not unique (retries after
`FailDispatch` create new rows, §8's circuit-breaker note) — this RPC
returns the **most recent** row for the task (`ORDER BY created_at DESC
LIMIT 1`), matching the frontend's actual use ("which terminal is this
task *currently* running on"), not a full attempt history.

## Changes to make

### 1. `backend-go/proto/orca/orchestration/v1/orchestration.proto`

Add to the `OrchestrationService` service block:

```protobuf
service OrchestrationService {
  rpc CreateDispatchContext(CreateDispatchContextRequest) returns (CreateDispatchContextResponse);
  rpc CreateGate(CreateGateRequest) returns (CreateGateResponse);
  rpc ResolveGate(ResolveGateRequest) returns (ResolveGateResponse);
  rpc UpdateTaskStatusAndPromote(UpdateTaskStatusAndPromoteRequest) returns (UpdateTaskStatusAndPromoteResponse);

  // GetDispatchContextForTask is orchestration-service's first read RPC —
  // a scope addition beyond both the shipped proto and
  // orchestration-service.md §3's own drafted surface (neither has a
  // dispatch-context read). Backs orchestration.dispatchShow: "which
  // terminal was this task dispatched to." See SOL-018 for the "not a
  // missing assignee_handle field, a missing read RPC" distinction.
  rpc GetDispatchContextForTask(GetDispatchContextForTaskRequest) returns (GetDispatchContextForTaskResponse);
}
```

Add messages (append to the bottom of the file):

```protobuf
message GetDispatchContextForTaskRequest {
  string orchestration_task_id = 1;
}

message GetDispatchContextForTaskResponse {
  // Unset when no dispatch context exists yet for this task — a legitimate
  // state (the task hasn't been dispatched), not an error. Reuses the
  // existing DispatchContext message; no new message type needed.
  DispatchContext dispatch = 1;
}
```

Additive only (new RPC, new messages, no existing field touched) —
`buf breaking` stays clean.

### 2. `internal/usecase/ports.go` — extend `DispatchContextRepository`

```go
type DispatchContextRepository interface {
	CreateDispatchContext(ctx context.Context, tenantID, handle, coordinatorRunID, orchestrationTaskID string) (domain.DispatchContext, error)

	// GetLatestForTask returns the most recently created dispatch_contexts
	// row for orchestrationTaskID, or ErrDispatchContextNotFound if none
	// exists. A task's dispatch_contexts row is not unique (retries after
	// failure create new rows, §8's circuit-breaker note) — "latest" is
	// the current dispatch, which is what dispatchShow's "which terminal
	// is this on" question actually needs, not full attempt history.
	GetLatestForTask(ctx context.Context, tenantID, orchestrationTaskID string) (domain.DispatchContext, error)
}
```

### 3. `internal/usecase/get_dispatch_context_for_task.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

type GetDispatchContextForTask struct {
	repo DispatchContextRepository
}

func NewGetDispatchContextForTask(repo DispatchContextRepository) *GetDispatchContextForTask {
	return &GetDispatchContextForTask{repo: repo}
}

// Execute returns (context, found, err). found=false with err=nil means
// "no dispatch context exists yet for this task" — a legitimate read
// result the gRPC adapter maps to an unset response field, not a gRPC
// error, since ErrDispatchContextNotFound already exists as a sentinel
// (ports.go) for exactly this case.
func (uc *GetDispatchContextForTask) Execute(ctx context.Context, taskID string) (domain.DispatchContext, bool, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DispatchContext{}, false, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	if taskID == "" {
		return domain.DispatchContext{}, false, apperrors.New(apperrors.KindInvalidArgument, "ORCH_EMPTY_TASK_ID", "orchestration_task_id is required", nil)
	}
	dc, err := uc.repo.GetLatestForTask(ctx, tenantID, taskID)
	if errors.Is(err, ErrDispatchContextNotFound) {
		return domain.DispatchContext{}, false, nil
	}
	if err != nil {
		return domain.DispatchContext{}, false, apperrors.New(apperrors.KindInternal, "ORCH_GET_DISPATCH_FAILED", "failed to look up dispatch context", err)
	}
	return dc, true, nil
}
```

Read-only — no `HandleSerializer` routing needed; none of §8's atomicity
table rows apply to a pure read.

### 4. `internal/adapter/postgres/repository.go` — implement `GetLatestForTask`

Find `CreateDispatchContext`'s implementation in this file and add:

```go
func (r *Repository) GetLatestForTask(ctx context.Context, tenantID, orchestrationTaskID string) (domain.DispatchContext, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, orchestration_task_id, handle, coordinator_run_id, status,
		       failure_count, last_failure, dispatched_at, completed_at, last_heartbeat_at, created_at
		FROM dispatch_contexts
		WHERE tenant_id = $1 AND orchestration_task_id = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, orchestrationTaskID)

	var dc domain.DispatchContext
	var lastFailure *string
	var dispatchedAt, completedAt, lastHeartbeatAt *time.Time
	err := row.Scan(&dc.ID, &dc.OrchestrationTaskID, &dc.Handle, &dc.CoordinatorRunID, &dc.Status,
		&dc.FailureCount, &lastFailure, &dispatchedAt, &completedAt, &lastHeartbeatAt, &dc.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DispatchContext{}, usecase.ErrDispatchContextNotFound
	}
	if err != nil {
		return domain.DispatchContext{}, fmt.Errorf("postgres: get latest dispatch context for task: %w", err)
	}
	dc.TenantID = tenantID
	if lastFailure != nil {
		dc.LastFailure = *lastFailure
	}
	if dispatchedAt != nil {
		dc.DispatchedAt = *dispatchedAt
	}
	if completedAt != nil {
		dc.CompletedAt = *completedAt
	}
	if lastHeartbeatAt != nil {
		dc.LastHeartbeatAt = *lastHeartbeatAt
	}
	return dc, nil
}
```

Match this file's existing column-nullability handling exactly — check
how `CreateDispatchContext`'s own `Scan`/insert already handles
`last_failure`/`dispatched_at`/`completed_at`/`last_heartbeat_at` (nullable
columns per `domain.DispatchContext`'s doc comment) before writing this,
and mirror that convention rather than the sketch above if it differs
(e.g. if the existing code uses `pgtype.Timestamptz` instead of `*time.Time`).
Add `"errors"` and `"github.com/jackc/pgx/v5"` to this file's imports if
not already present.

### 5. `internal/adapter/grpc/server.go` — wire the RPC

```go
type Server struct {
	orchestrationv1.UnimplementedOrchestrationServiceServer

	createDispatchContext      *usecase.CreateDispatchContext
	createGate                 *usecase.CreateGate
	resolveGate                *usecase.ResolveGate
	updateTaskStatusAndPromote *usecase.UpdateTaskStatusAndPromote
	getDispatchContextForTask  *usecase.GetDispatchContextForTask // NEW
}

func New(
	createDispatchContext *usecase.CreateDispatchContext,
	createGate *usecase.CreateGate,
	resolveGate *usecase.ResolveGate,
	updateTaskStatusAndPromote *usecase.UpdateTaskStatusAndPromote,
	getDispatchContextForTask *usecase.GetDispatchContextForTask, // NEW
) *Server {
	return &Server{
		createDispatchContext:      createDispatchContext,
		createGate:                 createGate,
		resolveGate:                resolveGate,
		updateTaskStatusAndPromote: updateTaskStatusAndPromote,
		getDispatchContextForTask:  getDispatchContextForTask,
	}
}

func (s *Server) GetDispatchContextForTask(ctx context.Context, req *orchestrationv1.GetDispatchContextForTaskRequest) (*orchestrationv1.GetDispatchContextForTaskResponse, error) {
	dc, found, err := s.getDispatchContextForTask.Execute(ctx, req.GetOrchestrationTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	if !found {
		return &orchestrationv1.GetDispatchContextForTaskResponse{}, nil
	}
	return &orchestrationv1.GetDispatchContextForTaskResponse{
		Dispatch: &orchestrationv1.DispatchContext{
			Id:                  dc.ID,
			Handle:              dc.Handle,
			CoordinatorRunId:    dc.CoordinatorRunID,
			OrchestrationTaskId: dc.OrchestrationTaskID,
		},
	}, nil
}
```

### 6. `cmd/server/main.go` — wire the new usecase and constructor arg

```go
getDispatchContextForTaskUC := usecase.NewGetDispatchContextForTask(repo)

server := orchestrationgrpc.New(createDispatchContextUC, createGateUC, resolveGateUC, updateTaskStatusAndPromoteUC, getDispatchContextForTaskUC)
```

(Match the existing local variable names for the other 4 usecases in
`main.go` — this shows the shape, not necessarily the exact identifiers.)

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/orchestration-service/... && go vet ./services/orchestration-service/...
go test ./services/orchestration-service/... -count=1
```
