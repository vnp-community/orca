# TASK-TG-004-03: `AdvancePendingRuns` autonomous tick loop + `FOR UPDATE SKIP LOCKED` concurrency safety

**From Solution:** BE-SOL-004
**Priority:** P1 — **needs a short design review before merge, per the CR's own risk note** (this is the one genuinely new architectural decision in BE-SOL-004; everything else in the BE-SOL-004 series reuses an existing pattern)
**Service:** `orchestration-service`
**File:** `backend-go/services/orchestration-service/internal/usecase/advance_pending_runs.go` (new), `backend-go/services/orchestration-service/internal/adapter/postgres/repository.go` (`ListRunningForAdvance`), `backend-go/services/orchestration-service/cmd/server/main.go` (start the tick loop)
**Depends on:** TASK-TG-004-01 (`CoordinatorRunRepository.ListRunningForAdvance`), TASK-TG-004-02 (`CompleteCoordinatorRun`/`FailCoordinatorRun` usecases this loop calls at the end of a run)
**Status:** `[ ]` TODO — flag for a short architecture/design review before merge (BE-SOL-004's own explicit note)

---

## Context

Entirely new — confirmed, `grep -rn "AdvancePendingRuns\|runCoordinatorScanLoop"
internal/ cmd/` finds nothing today. This is the one piece of BE-SOL-004
that isn't just "wire the missing layer on top of already-shipped domain +
schema" (per TASK-TG-004-01's Context) — it introduces a genuinely new
multi-instance-safe polling loop, hence the design-review flag above,
carried over verbatim from BE-SOL-004's own text: *"this should get a short
design review before merge, per the CR's own risk note."*

**This task's own dispatch step is deliberately a stub/hook, not a full
implementation** — per BE-SOL-004 §"Not in scope": *"The actual dispatch
call inside `AdvancePendingRuns` — owned by
[BE-SOL-005](./BE-SOL-005-task-agent-execution-permission-and-complex-executor.md)/[BE-SOL-006](./BE-SOL-006-task-execute-streaming-relay.md)."*
This task detects "ready and not yet dispatched" and calls out to whatever
dispatch primitive TASK-TG-005/-006 land — if those tasks haven't landed
yet when this one is implemented, leave the actual dispatch call as an
explicit `// TODO(TASK-TG-005/-006): dispatch here` with a no-op/log stub,
covered by a test that asserts detection works even though dispatch is not
yet wired — do not silently invent a placeholder dispatch mechanism.

`ready` tasks reuse `UpdateStatusAndPromote`'s existing "ready" status — no
new `OrchestrationTask` status is introduced by this task (confirmed:
`domain.TaskStatus`'s existing values already include a promotion target
per `UpdateStatusAndPromote`'s doc comment,
`orchestration-service/internal/usecase/ports.go:39-56`).

## Changes to make

**1. `internal/adapter/postgres/repository.go`** — `ListRunningForAdvance`:

```go
func (r *Repository) ListRunningForAdvance(ctx context.Context, limit int) ([]domain.CoordinatorRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms, created_at, completed_at
		FROM orchestration.coordinator_runs
		WHERE status = 'running'
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	// ...scan loop, same shape as every other List* method in this file...
}
```

Standard Postgres work-queue pattern — a second `orchestration-service`
instance's concurrent tick simply skips rows the first instance already
locked, no distributed lock service needed. **`FOR UPDATE SKIP LOCKED`
requires this query to run inside an explicit transaction that stays open
for the duration of this tick's processing** (the row lock is held until
`COMMIT`/`ROLLBACK`) — this is a meaningfully different transaction
lifetime than every other method in this file (which open-and-close a
transaction around one write). Confirm `r.pool.Query` alone (no explicit
`Begin`) actually holds the lock for the caller's intended window, or
whether `ListRunningForAdvance` needs to accept/return a `pgx.Tx` the
caller commits after processing each run — this is exactly the kind of
detail the design review above should settle, not something to guess at
silently.

**2. `internal/usecase/advance_pending_runs.go`** (new):

```go
type AdvancePendingRuns struct {
	runs        CoordinatorRunRepository
	otasks      OrchestrationTaskRepository
	complete    *CompleteCoordinatorRun
	fail        *FailCoordinatorRun
}

func (uc *AdvancePendingRuns) Execute(ctx context.Context) error {
	runs, err := uc.runs.ListRunningForAdvance(ctx, 50)
	if err != nil {
		return err
	}
	for _, run := range runs {
		ready, err := uc.otasks.ListReadyUndispatched(ctx, run.TenantID, run.ID) // NEW method — confirm this doesn't already exist under a different name before adding it
		if err != nil {
			continue // best-effort per run — one run's failure must not block the batch
		}
		for _, task := range ready {
			// TODO(TASK-TG-005/-006): dispatch here — see this task's Context
			// for why the actual dispatch call is deliberately deferred.
			_ = task
		}
		if allTerminal(run, ready) {
			if _, err := uc.complete.Execute(ctx, CompleteCoordinatorRunInput{ID: run.ID}); err != nil {
				continue // logged, retried next tick
			}
		}
	}
	return nil
}
```

`ListReadyUndispatched` is new surface BE-SOL-004's own sketch assumes
exists on `OrchestrationTaskRepository` — confirm via a direct grep before
adding it as a duplicate; if a differently-named method already answers
"orchestration tasks in this run that are ready but not dispatched," reuse
it instead of adding a second one.

**3. `cmd/server/main.go`** — start the loop alongside the existing gRPC
listener (confirmed: `main.go` is 147 lines, `grpcServer.Serve(lis)` runs at
line 119 inside its own goroutine already — add this loop the same way,
not blocking `main`'s own shutdown-signal handling):

```go
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
				logger.Error("coordinator scan tick failed", "err", err) // never panics — retried next tick
			}
		}
	}
}
```

Use this service's own `slog`-based logger (check `main.go`'s existing
logger variable name/type) rather than the bare `log` package the
solution's sketch uses.

## Test plan

- Two `AdvancePendingRuns.Execute` calls running concurrently (simulating 2
  service instances) never dispatch the same `OrchestrationTask` twice —
  test via two goroutines + `FOR UPDATE SKIP LOCKED` against a REAL test DB
  (testcontainers-go, not mocked — the guarantee is DB-level, a mock cannot
  prove it).
- `RecordHeartbeat`-staleness-triggers-`FailCoordinatorRun` is explicitly
  NOT covered by this task (per BE-SOL-004: "a follow-up tuning decision,
  not blocking this solution's merge") — do not silently add it.
- A single-run failure (simulated repository error) does not stop the tick
  from processing other runs in the same batch.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/orchestration-service/...
go test ./services/orchestration-service/internal/usecase/... -run TestAdvancePendingRuns -v
go test ./services/orchestration-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; the 2-goroutine concurrency test is the one that
actually matters here and must run against a real Postgres instance, not a
fake repository; a tick with zero running runs is a fast no-op (no error,
no busy-loop).
