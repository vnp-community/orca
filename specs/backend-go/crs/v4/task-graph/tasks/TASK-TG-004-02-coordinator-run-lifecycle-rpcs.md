# TASK-TG-004-02: `CoordinatorRun` lifecycle RPCs — Start/Get/Complete/Fail/RecordHeartbeat

**From Solution:** BE-SOL-004
**Priority:** P1
**Service:** `orchestration-service`
**File:** `backend-go/services/orchestration-service/internal/usecase/start_coordinator_run.go`, `get_coordinator_run.go`, `complete_coordinator_run.go`, `fail_coordinator_run.go`, `record_heartbeat.go` (all new), `backend-go/proto/orca/orchestration/v1/orchestration.proto` (5 new RPCs), `backend-go/services/orchestration-service/internal/adapter/grpc/server.go` (handlers)
**Depends on:** TASK-TG-004-01 (`CoordinatorRunRepository` must exist)
**Status:** `[ ]` TODO

---

## Context

`orchestration.proto`'s real RPC list today is 7 methods
(`CreateDispatchContext`, `CreateGate`, `ResolveGate`,
`UpdateTaskStatusAndPromote`, `GetDispatchContextForTask`,
`ListActiveDispatchContextsForUser`, `FailDispatch` — per BE-SOL-004's own
count, not independently re-verified line-by-line here since the RPC names
themselves aren't in question, only that `CoordinatorRun`'s group is
absent) versus `orchestration-service.md` §3's 15-RPC sketch
(`orchestration-service.md:73-99`) — confirming the TDD intended exactly
this `CoordinatorRun` group. Use the TDD's own naming
(`orchestration-service.md:75-81,86`), not invented names.

`task-service`'s TASK-TG-005-02 (`ComplexExecutor`) is `StartCoordinatorRun`'s
real caller — it does not also create the root `OrchestrationTask` row
itself, per `orchestration-service.md` §2.1's "id-space-own" rule:
`task-service` never writes directly into `orchestration.*` tables.

## Changes to make

**1. `orchestration.proto`** — add the 5 RPCs:

```protobuf
rpc StartCoordinatorRun(StartCoordinatorRunRequest) returns (CoordinatorRun);
rpc GetCoordinatorRun(GetCoordinatorRunRequest) returns (CoordinatorRun);
rpc CompleteCoordinatorRun(CompleteCoordinatorRunRequest) returns (CoordinatorRun);
rpc FailCoordinatorRun(FailCoordinatorRunRequest) returns (CoordinatorRun);
rpc RecordHeartbeat(RecordHeartbeatRequest) returns (CoordinatorRun);

message CoordinatorRun {
  string id = 1;
  string tenant_id = 2;
  string origin_task_id = 3;
  string spec_json = 4;
  string status = 5; // idle|running|completed|failed
  string coordinator_handle = 6;
  int32 poll_interval_ms = 7;
}
message StartCoordinatorRunRequest {
  string origin_task_id = 1;
  string coordinator_handle = 2;
  string spec_json = 3;
  int32 poll_interval_ms = 4; // 0 = service default (domain.defaultPollIntervalMs = 2000, orchestration.go:296)
}
message GetCoordinatorRunRequest { string id = 1; }
message CompleteCoordinatorRunRequest { string id = 1; }
message FailCoordinatorRunRequest { string id = 1; string reason = 2; }
message RecordHeartbeatRequest { string id = 1; }
```

**2. `internal/usecase/start_coordinator_run.go`**:

```go
type StartCoordinatorRunInput struct {
	OriginTaskID, CoordinatorHandle string
	Spec                            json.RawMessage
	PollIntervalMs                  int32
}

type StartCoordinatorRun struct {
	runs CoordinatorRunRepository
}

func NewStartCoordinatorRun(runs CoordinatorRunRepository) *StartCoordinatorRun {
	return &StartCoordinatorRun{runs: runs}
}

func (uc *StartCoordinatorRun) Execute(ctx context.Context, in StartCoordinatorRunInput) (domain.CoordinatorRun, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err)
	}
	run, err := domain.NewCoordinatorRun(uuid.NewString(), tenantID, in.OriginTaskID, in.CoordinatorHandle, in.Spec, in.PollIntervalMs)
	if err != nil {
		return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_RUN_INVALID", err.Error(), err)
	}
	// NewCoordinatorRun already sets Status = RunStatusIdle (orchestration.go:336) —
	// Create persists as-is. TASK-TG-004-03's tick loop, not this usecase,
	// transitions idle -> running on its next pass.
	return uc.runs.Create(ctx, run)
}
```

**3. `get_coordinator_run.go`/`complete_coordinator_run.go`/
`fail_coordinator_run.go`/`record_heartbeat.go`** — each a thin wrapper
around the matching `CoordinatorRunRepository` method (TASK-TG-004-01),
following `StartCoordinatorRun`'s shape: extract `tenantID`, call the
repository, wrap errors with a dedicated `apperrors` code
(`ORCH_RUN_NOT_FOUND`, `ORCH_RUN_COMPLETE_FAILED`, etc.). `CompleteCoordinatorRun`/
`FailCoordinatorRun` should reject (return an error, not silently succeed)
when the run is not currently `RunStatusRunning` — per BE-SOL-004's test
plan: *"reject a run not currently running."* Add this check either in the
usecase (fetch-then-check-then-write, accepting a race under concurrent
calls since this is an operator/system-triggered transition, not a
high-contention user path) or push it into `UpdateStatus`'s SQL via a
`WHERE status = 'running'` guard clause + `RowsAffected() == 0` check
(preferred — avoids the read-then-write race entirely, same reasoning as
`UpdateStatus`'s existing `tag.RowsAffected() == 0` check in `task-service`'s
own repository at `repository.go:154-156`).

**4. `internal/adapter/grpc/server.go`** handlers — thin, following
`CreateDispatchContext`'s existing handler pattern in the same file (find
it, mirror its request/response marshaling exactly, don't invent a new
handler style for this one RPC group).

## Test plan

- `StartCoordinatorRun` persists a real row, retrievable via
  `GetCoordinatorRun`.
- `CompleteCoordinatorRun`/`FailCoordinatorRun` set `completed_at`, reject a
  run not currently `running`.
- `RecordHeartbeat` updates `heartbeat_at` (TASK-TG-004-01's added column)
  without changing `status`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/orchestration-service/...
go test ./services/orchestration-service/internal/usecase/... -run "TestStartCoordinatorRun|TestGetCoordinatorRun|TestCompleteCoordinatorRun|TestFailCoordinatorRun|TestRecordHeartbeat" -v
go test ./services/orchestration-service/internal/adapter/grpc/... -v
```

Expected: clean build; all 5 RPCs round-trip end-to-end against a real test
DB; `CompleteCoordinatorRun`/`FailCoordinatorRun` on an already-terminal run
return a clear precondition-failed error, not a silent no-op or a generic
internal error.
