# TASK-TG-001-04: Auto-block on unmet `depends_on` + atomic cycle-check-then-write `AddEdge` (corrected to the real `TxRunner` shape)

**From Solution:** BE-SOL-001
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/add_edge.go`, `backend-go/services/task-service/internal/adapter/grpc/server.go` (wire the new `TxRunner`-wrapped call)
**Depends on:** TASK-TG-001-02 (`domain.StatusBlocked`, `domain.Status` type)
**Status:** `[x]` DONE

---

## Context

**Grounding correction versus BE-SOL-001's own sketch — read before writing
this task.** BE-SOL-001's "Design — auto-block on unmet dependency + atomic
`AddEdge`" section sketches:

```go
return u.tx.RunInTx(ctx, func(tx ports.Tx) error {
    wouldCycle, err := u.validator.WouldCreateCycleTx(ctx, tx, fromID, toID)
    ...
    u.repo.InsertEdgeTx(ctx, tx, fromID, toID, kind)
    fromTask, err := u.repo.GetTx(ctx, tx, fromID)
    ...
    return u.repo.UpdateStatusTx(ctx, tx, toID, domain.StatusBlocked)
})
```

This `ports.Tx` / `*Tx`-suffixed-method shape **does not exist and would be
new port surface** — but a `TxRunner` port that already solves this exact
problem is ALREADY DEFINED AND IMPLEMENTED in this codebase, confirmed by
direct read:

- `internal/usecase/ports.go:167-184`: `TxRunner` interface —
  `RunInTx(ctx context.Context, fn func(ctx context.Context, tasks
  TaskRepository, edges EdgeRepository) error) error`. Its doc comment: *"RunInTx
  hands fn a TaskRepository/EdgeRepository pair already scoped to the open
  transaction, reusing the exact port shapes AIApply's CreateTask/AddEdge
  sub-usecases already know, rather than inventing transaction-specific
  interfaces."*
- `internal/adapter/postgres/repository.go:62-67`: `Repository.RunInTx`
  implements it via `pgx.BeginFunc`, handing `fn` a single `*Repository`
  scoped to the open `pgx.Tx` for BOTH the `tasks` and `edges` parameters
  (one struct implements both `usecase.TaskRepository` and
  `usecase.EdgeRepository`).
- `internal/usecase/ai_apply.go:53-73` is the real, ALREADY-SHIPPED consumer
  of this exact pattern: it calls `uc.txRunner.RunInTx(ctx, func(ctx,
  tasks, edges) error { createTask := NewCreateTask(tasks); addEdge :=
  NewAddEdge(edges); ... })` — constructing fresh `CreateTask`/`AddEdge`
  usecase instances scoped to the transaction's repos, not calling
  `*Tx`-suffixed methods on anything.

**This task follows the real, working `TxRunner` pattern, not BE-SOL-001's
`ports.Tx` sketch.** Concretely, this means:

1. `AddEdge` gains a `tasks TaskRepository` field (needed for the auto-block
   check: reading `fromTask.Status` and writing `toID`'s status) alongside
   its existing `edges EdgeRepository` field — `NewAddEdge(tasks
   TaskRepository, edges EdgeRepository)`. This is a breaking constructor
   signature change; update `ai_apply.go:61`'s `NewAddEdge(edges)` call to
   `NewAddEdge(tasks, edges)` (it already has both `tasks`/`edges` in scope
   from its own `RunInTx` closure) and `cmd/server/main.go`'s wiring.
2. `AddEdge.Execute` itself does **not** open a transaction — it trusts
   whatever `tasks`/`edges` pair it was constructed with are already
   correctly scoped (either the pool-backed repos for a standalone call, or
   tx-scoped repos when constructed inside `AIApply`'s `RunInTx` closure, as
   today). This mirrors `AIApply`'s existing relationship with `CreateTask`.
3. Atomicity for the **standalone** `AddEdge` RPC path (the actual race
   BE-SOL-001 is fixing — closing `add_edge.go`'s own admitted gap at lines
   41-46: *"fetching the existing edge set and then writing the new one is
   two separate calls, not one transaction... Not wired in this scaffold"*)
   is achieved by wrapping the call at the point that previously called
   `NewAddEdge(edges).Execute(...)` directly: the gRPC handler (or a thin
   new usecase-level wrapper) calls
   `txRunner.RunInTx(ctx, func(ctx, tasks, edges) error { return
   NewAddEdge(tasks, edges).Execute(ctx, in) })` instead of constructing
   `AddEdge` once at server-startup wiring time. This requires `server.go`
   to hold a `usecase.TxRunner` (the same `*postgres.Repository` already
   passed in for `AIApply`'s wiring) instead of (or in addition to) a
   pre-built `*usecase.AddEdge`.

## Changes to make

**1. `internal/usecase/add_edge.go`** — widen the struct and `Execute`:

```go
type AddEdge struct {
	tasks TaskRepository
	edges EdgeRepository
}

func NewAddEdge(tasks TaskRepository, edges EdgeRepository) *AddEdge {
	return &AddEdge{tasks: tasks, edges: edges}
}

func (uc *AddEdge) Execute(ctx context.Context, in AddEdgeInput) (domain.TaskEdge, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	edge, err := domain.NewTaskEdge(in.FromTaskID, in.ToTaskID, in.Kind)
	if err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_EDGE_INVALID", err.Error(), err)
	}

	if edge.Kind == domain.EdgeKindDependsOn {
		existing, err := uc.edges.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn)
		if err != nil {
			return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_LIST_FAILED", "failed to list existing edges for cycle check", err)
		}
		if domain.DetectCycle(existing, edge) {
			return domain.TaskEdge{}, apperrors.New(apperrors.KindFailedPrecondition, "TASK_CYCLIC_DEPENDENCY", domain.ErrCyclicDependency.Error(), domain.ErrCyclicDependency)
		}
	}

	if err := uc.edges.Add(ctx, tenantID, edge); err != nil {
		return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_ADD_FAILED", "failed to persist edge", err)
	}

	// NEW — auto-block: a fresh depends_on edge onto a not-yet-done
	// dependency blocks the dependent task immediately.
	if edge.Kind == domain.EdgeKindDependsOn {
		fromTask, err := uc.tasks.Get(ctx, tenantID, in.FromTaskID)
		if err != nil {
			return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_AUTOBLOCK_LOOKUP_FAILED", "failed to load dependency task for auto-block check", err)
		}
		if fromTask.Status != domain.StatusDone {
			if err := uc.tasks.UpdateStatus(ctx, tenantID, in.ToTaskID, domain.StatusBlocked); err != nil {
				return domain.TaskEdge{}, apperrors.New(apperrors.KindInternal, "TASK_EDGE_AUTOBLOCK_FAILED", "failed to auto-block dependent task", err)
			}
		}
	}
	return edge, nil
}
```

Everything from the cycle-check read (`ListByKind`) through the auto-block
write (`UpdateStatus`) now happens against whichever `tasks`/`edges` pair
`AddEdge` was constructed with — atomic when that pair is transaction-scoped
(see point 3 above), exactly as `AIApply` already relies on for its own
`CreateTask`/`AddEdge` calls.

**2. `internal/adapter/grpc/server.go`** — replace the pre-built
`*usecase.AddEdge` field with a `usecase.TxRunner` (or add one alongside),
and in the `AddEdge` handler:

```go
func (s *Server) AddEdge(ctx context.Context, req *taskv1.AddEdgeRequest) (*taskv1.AddEdgeResponse, error) {
	err := s.txRunner.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		_, err := usecase.NewAddEdge(tasks, edges).Execute(ctx, usecase.AddEdgeInput{
			FromTaskID: req.GetFromTaskId(), ToTaskID: req.GetToTaskId(), Kind: toDomainEdgeKind(req.GetType()),
		})
		return err
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AddEdgeResponse{}, nil
}
```

**3. `cmd/server/main.go`** — update `NewAddEdge` call sites for the new
2-arg constructor; `AIApply`'s own wiring (`ai_apply.go:61`) already has
both `tasks`/`edges` in scope, just change the call from `NewAddEdge(edges)`
to `NewAddEdge(tasks, edges)`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestAddEdge|TestAIApply" -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build (constructor signature change compiles everywhere);
two goroutines calling the `AddEdge` RPC concurrently with edges that
together would form a cycle — exactly one succeeds, the other gets
`ErrCyclicDependency` (test against a real Postgres transaction, not a
mock, since the guarantee is DB-level `RunInTx` serialization); adding a
`depends_on` edge to a not-done task flips the target to `StatusBlocked` in
the same call; `TestAIApply_MidLoopFailure_RollsBackEntireSubtree`
(`ai_apply_test.go`, already exists) still passes unchanged, proving the
`NewAddEdge(tasks, edges)` signature change didn't disturb `AIApply`'s
existing transactional behavior.

## Execution notes (2026-09-09)

Implemented exactly per the corrected `TxRunner` shape (not BE-SOL-001's
`ports.Tx` sketch): `AddEdge` gained a `tasks TaskRepository` field
(`NewAddEdge(tasks, edges)`), does its own cycle-check + write + auto-block
sequence without opening a transaction itself, and trusts the
tasks/edges pair it was constructed with. `server.go`'s `AddEdge` field was
replaced with `txRunner usecase.TxRunner`; the `AddEdge` RPC handler now
constructs a fresh `usecase.NewAddEdge(tasks, edges)` inside
`txRunner.RunInTx` per call, matching the task's own handler sketch
verbatim. `ai_apply.go:61`'s `NewAddEdge(edges)` → `NewAddEdge(tasks,
edges)`; `main.go` no longer builds a standalone `addEdgeUC` — `repo`
(already a `usecase.TxRunner`) is passed straight into `taskgrpc.New`.

Fixed the resulting fallout beyond the task's own file list: `add_edge_test.go`
needed a `TaskRepository` fake at every `NewAddEdge(...)` call site (seeded
with a real `domain.Task` for the two tests that now reach the auto-block
lookup) and gained 2 new tests (`TestAddEdge_AutoBlocksDependentOnUnmetDependency`,
`TestAddEdge_DoesNotAutoBlockWhenDependencyAlreadyDone`) since the auto-block
behavior itself had no unit coverage yet; `server_test.go`'s `newTestServer`
already had a `fakeTxRunner` (built for `AIApply`'s wiring) — reused it for
`AddEdge` too rather than introducing a second fake.

Added a new integration-tagged file,
`internal/adapter/postgres/add_edge_integration_test.go`, with the exact
test the Verify section asks for:
`TestAddEdge_ConcurrentCycleRace_ExactlyOneSucceeds` (two goroutines racing
`b->c` and `c->a` against a seeded `a->b`, each wrapped in a real
`repo.RunInTx`+`usecase.AddEdge` call — asserts exactly one of the two
succeeds) plus `TestAddEdge_AutoBlock_PersistsAgainstRealDB` (auto-block
write survives a real commit, not just a fake's in-memory map).

Verify: `go build`/`go vet ./services/task-service/...` both clean; `go test
./services/task-service/...` (unit) all pass, including the 2 new
`add_edge_test.go` cases and the unchanged `TestAIApply_MidLoopFailure_RollsBackEntireSubtree`.
`go test -tags=integration .../postgres/... -run TestAddEdge` — both new
integration tests pass; `TestAddEdge_ConcurrentCycleRace_ExactlyOneSucceeds`
hit the same pre-existing testcontainers `pq: the database system is
starting up` flake (documented in TASK-TG-001-02/-03's execution notes) on
one run and passed cleanly (4.87s) on immediate re-run in isolation — the
concurrency guarantee itself is confirmed working, the flake is
infra-level.
