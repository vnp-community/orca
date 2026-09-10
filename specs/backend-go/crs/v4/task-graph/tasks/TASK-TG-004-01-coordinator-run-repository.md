# TASK-TG-004-01: `CoordinatorRunRepository` port + Postgres adapter (no new domain type, no new migration)

**From Solution:** BE-SOL-004
**Priority:** P1
**Service:** `orchestration-service`
**File:** `backend-go/services/orchestration-service/internal/usecase/ports.go` (new `CoordinatorRunRepository` interface), `backend-go/services/orchestration-service/internal/adapter/postgres/repository.go` (implement it)
**Depends on:** None
**Status:** `[ ]` TODO

---

## Context

**`domain.CoordinatorRun` and the `orchestration.coordinator_runs` table
ALREADY EXIST — this task adds only the repository layer, no new domain
type, no new migration.** Verified directly:

- `internal/domain/orchestration.go:286-318` (confirmed exact lines by
  direct read): `RunStatus` enum (`idle|running|completed|failed`, lines
  287-294), `CoordinatorRun` struct (lines 308-318:
  `ID, TenantID, OriginTaskID, Spec, Status, CoordinatorHandle,
  PollIntervalMs, CreatedAt, CompletedAt`), `NewCoordinatorRun` constructor
  (lines 321-339ish) with the same invariant checks
  `OrchestrationTask`/`DispatchContext` get.
- `migrations/0001_init.up.sql:8-22` (confirmed exact lines): the
  `orchestration.coordinator_runs` table already has every column the
  domain struct needs (`id, tenant_id, origin_task_id, spec, status,
  coordinator_handle, poll_interval_ms, created_at, completed_at`), RLS
  already enabled with the standard `tenant_isolation` policy.

What's actually missing, confirmed by `grep -rn "CoordinatorRun"
internal/usecase/ports.go internal/adapter/`: `CoordinatorRunID` is used
only as a foreign-key-style field on `OrchestrationTask`/`DispatchContext`
— never a repository method on `CoordinatorRun` itself. Even
`repository_test.go`'s own tests have to seed a `coordinator_runs` row with
raw SQL (`seedCoordinatorRun` helper, line 53) because there's no code path
to create one through.

`OrchestrationTaskRepository`/`DispatchContextRepository`
(`internal/usecase/ports.go:39-57,60-110`, confirmed by direct read) are the
existing pattern to mirror exactly — same file, same
constructor/interface-per-entity style, same "method names distinct because
one concrete `Repository` implements all of them" convention explained in
`DispatchContextRepository`'s own doc comment (lines 58-59: *"Go has no
method overloading, so the port names must not collide"*).

## Changes to make

**1. `internal/usecase/ports.go`** — append:

```go
// CoordinatorRunRepository is the persistence port for
// orchestration.coordinator_runs — the table and domain.CoordinatorRun
// already exist (internal/domain/orchestration.go:286-318,
// migrations/0001_init.up.sql:8-22); this port is the only missing layer.
type CoordinatorRunRepository interface {
	Create(ctx context.Context, run domain.CoordinatorRun) (domain.CoordinatorRun, error)
	Get(ctx context.Context, tenantID, id string) (domain.CoordinatorRun, error)
	UpdateStatus(ctx context.Context, tenantID, id string, status domain.RunStatus, completedAt *time.Time) (domain.CoordinatorRun, error)
	// RecordHeartbeat updates a lightweight liveness timestamp — see
	// TASK-TG-004-02's RecordHeartbeat usecase. Requires a heartbeat_at
	// column; the existing coordinator_runs table has none — confirm this
	// requires a small additive migration (see note below) rather than
	// silently reusing an existing timestamp column for a different purpose.
	RecordHeartbeat(ctx context.Context, tenantID, id string, at time.Time) error
	// ListRunningForAdvance — FOR UPDATE SKIP LOCKED batch fetch, used by
	// TASK-TG-004-03's tick loop, not by any RPC handler directly.
	ListRunningForAdvance(ctx context.Context, limit int) ([]domain.CoordinatorRun, error)
}
```

**Correction versus BE-SOL-004's own §2 sketch**: its interface omits
`RecordHeartbeat` even though §3 lists `RecordHeartbeat` as one of the 5
RPCs this series adds — that RPC needs a repository method to actually
persist anything. Since `coordinator_runs` has no heartbeat-timestamp
column today (confirmed — `migrations/0001_init.up.sql:8-18`'s column list
has no `heartbeat_at`/`last_heartbeat_at`), this task needs a small,
genuinely additive migration BE-SOL-004's own "No new migration" claim
doesn't anticipate:

```sql
-- backend-go/services/orchestration-service/migrations/0004_coordinator_run_heartbeat.up.sql
-- (verify 0004 is still free — 0001-0003 exist today, confirmed by ls)
ALTER TABLE orchestration.coordinator_runs ADD COLUMN heartbeat_at TIMESTAMPTZ;
```

```sql
-- 0004_coordinator_run_heartbeat.down.sql
ALTER TABLE orchestration.coordinator_runs DROP COLUMN IF EXISTS heartbeat_at;
```

Flag this divergence in the PR description — BE-SOL-004's headline claim
("No new migration") holds for 4 of its 5 RPCs, not `RecordHeartbeat`.

**2. `internal/adapter/postgres/repository.go`** — implement
`CoordinatorRunRepository` following `OrchestrationTaskRepository`'s exact
pattern in the same file (`Create`/`Get` shape mirrors
`OrchestrationTaskRepository.Create`/`Get`; `UpdateStatus` mirrors
`UpdateStatusAndPromote`'s status-write half, minus the sibling-promotion
logic which is specific to `orchestration_tasks`, not `coordinator_runs`):

```go
func (r *Repository) CreateCoordinatorRun(ctx context.Context, run domain.CoordinatorRun) (domain.CoordinatorRun, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO orchestration.coordinator_runs (id, tenant_id, origin_task_id, spec, status, coordinator_handle, poll_interval_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, run.ID, run.TenantID, run.OriginTaskID, run.Spec, string(run.Status), run.CoordinatorHandle, run.PollIntervalMs)
	if err != nil {
		return domain.CoordinatorRun{}, fmt.Errorf("postgres: insert coordinator run: %w", err)
	}
	return run, nil
}
```

(Method names on `*Repository` must not collide with
`OrchestrationTaskRepository.Create`/`Get` — following the file's own
stated convention, name these `CreateCoordinatorRun`/`GetCoordinatorRun`/
etc., NOT bare `Create`/`Get`, even though the interface method name itself
can stay `Create`/`Get` per Go's per-interface method-set rules — check
whether `DispatchContextRepository.CreateDispatchContext`'s naming
precedent already disambiguates this way before deciding, and match it
exactly.)

`ListRunningForAdvance`'s `FOR UPDATE SKIP LOCKED` query is detailed in
TASK-TG-004-03 (it's used exclusively by that task's tick loop) — implement
it here as part of this port, but see that task for the exact SQL and its
concurrency-safety test.

## Test plan

- `Create` persists a real row, retrievable via `Get`.
- `UpdateStatus` transitions `idle -> running -> completed`, sets
  `completed_at` only on a terminal status.
- `RecordHeartbeat` updates `heartbeat_at` without touching `status`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/orchestration-service/...
go test ./services/orchestration-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; the existing `repository_test.go`'s
`seedCoordinatorRun` raw-SQL helper can now be replaced with a real
`Create` call in new tests (leave the existing helper in place for any test
that still wants direct control over seed data, don't force a migration of
every existing test in this same task unless trivial).
