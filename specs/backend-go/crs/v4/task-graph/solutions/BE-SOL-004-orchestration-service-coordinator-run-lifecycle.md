# BE-SOL-004: `CoordinatorRun` lifecycle RPCs + autonomous advance loop

**Resolves:** [CR-TG-004](../../../../../../docs/crs/v4/task-graph/CR-TG-004-orchestration-service-coordinator-run-lifecycle.md)
**Service:** `orchestration-service` only
**Affected files (proposed):**
- `backend-go/services/orchestration-service/internal/usecase/ports.go` (new `CoordinatorRunRepository` interface)
- `backend-go/services/orchestration-service/internal/usecase/start_coordinator_run.go`, `get_coordinator_run.go`, `complete_coordinator_run.go`, `fail_coordinator_run.go`, `record_heartbeat.go` (new)
- `backend-go/services/orchestration-service/internal/adapter/postgres/repository.go` (implement `CoordinatorRunRepository` against the already-existing `orchestration.coordinator_runs` table)
- `backend-go/services/orchestration-service/internal/usecase/advance_pending_runs.go` (new — the scan-loop's per-tick logic)
- `backend-go/services/orchestration-service/cmd/server/main.go` (start the tick loop alongside the existing gRPC listener)
- `backend-go/proto/orca/orchestration/v1/orchestration.proto` (5 new RPCs)
- `backend-go/services/orchestration-service/internal/adapter/grpc/server.go` (handlers)
- **No new migration** — `orchestration.coordinator_runs` already exists with every column this solution needs (see §1)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** dòng "No new migration"
> ở trên **sai một phần** — đúng cho `StartCoordinatorRun`/`GetCoordinatorRun`/
> `CompleteCoordinatorRun`/`FailCoordinatorRun`, nhưng `RecordHeartbeat`
> cần 1 cột `heartbeat_at` chưa tồn tại trên `orchestration.coordinator_runs`
> — vẫn cần 1 migration additive nhỏ. Xem
> [TASK-TG-004-01](../tasks/TASK-TG-004-01-coordinator-run-repository.md)
> cho chi tiết migration đã bổ sung.

---

## 1. Current state — narrower gap than it first appears

`domain.CoordinatorRun` **already exists, fully modeled and validated**
(`internal/domain/orchestration.go:308-338` — `RunStatus` enum
`idle|running|completed|failed`, `NewCoordinatorRun` constructor with the
same invariant checks `OrchestrationTask`/`DispatchContext` get). The
`orchestration.coordinator_runs` table **already exists** with every
column the domain struct needs
(`migrations/0001_init.up.sql:8-20` — `id, tenant_id, origin_task_id, spec,
status, coordinator_handle, poll_interval_ms, created_at, completed_at`,
RLS already enabled).

**What's actually missing is one layer only: no `CoordinatorRunRepository`
port, no usecase, no proto RPC, and no background loop.** Confirmed
directly: `grep -rn "CoordinatorRun" internal/usecase/ports.go
internal/adapter/` finds `CoordinatorRunID` used only as a foreign-key-style
field on `OrchestrationTask`/`DispatchContext` — never a repository method
on `CoordinatorRun` itself. Even the repository's own tests have to seed a
`coordinator_runs` row with raw SQL (`repository_test.go:53`'s
`seedCoordinatorRun` helper) because there's no code path to create one
through. This means the CR's "6 missing RPC" framing corresponds to
building the usecase/adapter/proto layer on top of domain modeling and
schema that are both **already done and don't need to change** — the
smallest-footprint version of this gap possible.

`orchestration.proto`'s real RPC list (7: `CreateDispatchContext`,
`CreateGate`, `ResolveGate`, `UpdateTaskStatusAndPromote`,
`GetDispatchContextForTask`, `ListActiveDispatchContextsForUser`,
`FailDispatch`) vs. `orchestration-service.md` §3's sketch (15 RPCs,
`orchestration-service.md:73-99`) confirms the TDD intended a
`CoordinatorRun` RPC group (`StartCoordinatorRun`, `GetCoordinatorRun`,
`CompleteCoordinatorRun`, `FailCoordinatorRun`) plus `RecordHeartbeat` and
`ListPendingDecisionGates` that were never added to the real proto — this
solution builds exactly that group, using the TDD's own naming
(`orchestration-service.md:75-81,86`), not inventing new names.

**Explicitly not touched by this solution** (also sketched in the TDD but
outside CR-TG-004's scope, flagged so it isn't silently assumed done): the
`Message`/mailbox RPC group (`PostMessage`/`ListMessages`/
`MarkMessageRead`/`GetAgentStatusForHandle`, `orchestration-service.md:92-97`)
— no `domain.Message` type exists yet despite the `orchestration.messages`
table existing (`migrations/0001_init.up.sql:101-115`). This is a real,
separate gap; it needs its own CR if the product wants it, not a scope
expansion here.

## 2. Design — `CoordinatorRunRepository` port + Postgres adapter

```go
// usecase/ports.go
type CoordinatorRunRepository interface {
    Create(ctx context.Context, run domain.CoordinatorRun) (domain.CoordinatorRun, error)
    Get(ctx context.Context, tenantID, id string) (domain.CoordinatorRun, error)
    UpdateStatus(ctx context.Context, tenantID, id string, status domain.RunStatus, completedAt *time.Time) (domain.CoordinatorRun, error)
    // ListRunning + FOR UPDATE SKIP LOCKED batch fetch — used by the tick loop (§4), not by any RPC handler directly.
    ListRunningForAdvance(ctx context.Context, limit int) ([]domain.CoordinatorRun, error)
}
```

Implementation in `internal/adapter/postgres/repository.go` follows the
exact same pattern already used for `OrchestrationTaskRepository`/
`DispatchContextRepository` in the same file — no new pattern introduced.

## 3. Design — 5 RPCs (proto, per TDD §3's sketch)

```protobuf
rpc StartCoordinatorRun(StartCoordinatorRunRequest) returns (CoordinatorRun);
rpc GetCoordinatorRun(GetCoordinatorRunRequest) returns (CoordinatorRun);
rpc CompleteCoordinatorRun(CompleteCoordinatorRunRequest) returns (CoordinatorRun);
rpc FailCoordinatorRun(FailCoordinatorRunRequest) returns (CoordinatorRun);
rpc RecordHeartbeat(RecordHeartbeatRequest) returns (CoordinatorRun);
```

(`ListPendingDecisionGates` is deferred — `decision_gates`/`CreateGate`/
`ResolveGate` are already real; only the list-view read RPC is missing.
Small enough to fold into this same proto change, but the usecase is
trivial — a filtered `SELECT` on the existing `decision_gates` table — so
it's listed here for completeness but not detailed further.)

```go
// usecase/start_coordinator_run.go
func (uc *StartCoordinatorRun) Execute(ctx context.Context, in StartCoordinatorRunInput) (domain.CoordinatorRun, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    if err != nil { return domain.CoordinatorRun{}, apperrors.New(apperrors.KindUnauthenticated, "ORCH_NO_TENANT", "no tenant in request context", err) }
    run, err := domain.NewCoordinatorRun(uuid.NewString(), tenantID, in.OriginTaskID, in.CoordinatorHandle, in.Spec, in.PollIntervalMs)
    if err != nil { return domain.CoordinatorRun{}, apperrors.New(apperrors.KindInvalidArgument, "ORCH_RUN_INVALID", err.Error(), err) }
    // NewCoordinatorRun already sets Status = RunStatusIdle — Create persists as-is.
    // The tick loop (§4), not this usecase, transitions idle -> running on its next pass.
    return uc.runs.Create(ctx, run)
}
```

`task-service`'s [BE-SOL-005](./BE-SOL-005-task-agent-execution-permission-and-complex-executor.md)'s
`ComplexExecutor` is this RPC's caller — it does not also create the root
`OrchestrationTask` row itself (that's `StartCoordinatorRun`'s job,
matching `orchestration-service.md` §2.1's "id-space-own" rule: task-service
never writes directly into `orchestration.*` tables).

## 4. Design — autonomous advance loop

```go
// cmd/server/main.go — added alongside the existing gRPC listener, not replacing it
go runCoordinatorScanLoop(ctx, advanceUsecase, 5*time.Second) // interval: needs load-test before production default is final

func runCoordinatorScanLoop(ctx context.Context, uc *AdvancePendingRuns, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := uc.Execute(ctx); err != nil {
                log.Error("coordinator scan tick failed", "err", err) // never panics — retried next tick
            }
        }
    }
}
```

```go
// usecase/advance_pending_runs.go
func (uc *AdvancePendingRuns) Execute(ctx context.Context) error {
    runs, err := uc.runs.ListRunningForAdvance(ctx, 50) // FOR UPDATE SKIP LOCKED inside — see §5
    if err != nil { return err }
    for _, run := range runs {
        ready, err := uc.otasks.ListReadyUndispatched(ctx, run.TenantID, run.ID) // reuses UpdateStatusAndPromote's existing "ready" status, no new status introduced
        if err != nil { continue }
        for _, task := range ready {
            // dispatch: relay to infra-fleet-service, mark dispatched — see
            // CR-TG-005/CR-TG-006 for the actual dispatch mechanism this
            // loop invokes; this solution only owns detecting "ready and
            // not yet dispatched," not re-designing dispatch itself.
        }
        if uc.allTerminal(run) {
            _ = uc.completeOrFail(ctx, run)
        }
    }
    return nil
}
```

## 5. Design — concurrency safety across multiple `orchestration-service` instances

```sql
-- ListRunningForAdvance, inside the repository
SELECT * FROM orchestration.coordinator_runs
WHERE status = 'running'
ORDER BY created_at
LIMIT $1
FOR UPDATE SKIP LOCKED;
```

Standard Postgres work-queue pattern — a second instance's concurrent tick
simply skips rows the first instance already locked, no distributed lock
service needed. This is the one genuinely new architectural decision in
this solution (everything else reuses an existing pattern); it should get
a short design review before merge, per the CR's own risk note.

## Test plan

- `StartCoordinatorRun` persists a real row, retrievable via `GetCoordinatorRun`.
- Two `AdvancePendingRuns.Execute` calls running concurrently (simulating 2
  service instances) never dispatch the same `OrchestrationTask` twice —
  test via two goroutines + `FOR UPDATE SKIP LOCKED` against a real test DB
  (not mocked, since the guarantee is DB-level).
- `RecordHeartbeat` updates a timestamp the tick loop can later check for
  staleness (staleness-triggered `FailCoordinatorRun` is worth a follow-up
  test once that policy is implemented — see CR-TG-004 §2.3, not detailed
  further in this solution beyond the RPC existing).
- `CompleteCoordinatorRun`/`FailCoordinatorRun` set `completed_at`, reject
  a run not currently `running`.

## Not in scope (per the CR, plus §1's narrowing)

- `Message`/mailbox RPCs (`PostMessage`/`ListMessages`/`MarkMessageRead`/
  `GetAgentStatusForHandle`) — real, TDD-sketched gap, but not part of
  CR-TG-004; needs its own CR.
- Heartbeat-timeout-triggers-`FailCoordinatorRun` policy — RPC exists per
  this solution, the policy deciding *when* to call it (staleness
  threshold) is a follow-up tuning decision, not blocking this solution's
  merge.
- The actual dispatch call inside `AdvancePendingRuns` — owned by
  [BE-SOL-005](./BE-SOL-005-task-agent-execution-permission-and-complex-executor.md)/[BE-SOL-006](./BE-SOL-006-task-execute-streaming-relay.md).
- Production polling interval — 5s is a starting point for load-testing,
  not a final decision.

## References

- [CR-TG-004](../../../../../../docs/crs/v4/task-graph/CR-TG-004-orchestration-service-coordinator-run-lifecycle.md)
- `specs/backend-go/tdd/services/orchestration-service.md` §3, §4, §5
- `backend-go/services/orchestration-service/internal/domain/orchestration.go:287-338`
