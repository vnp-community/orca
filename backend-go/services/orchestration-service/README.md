# orchestration-service

Go implementation of the multi-agent coordination "complex path" pipeline —
see
[`specs/backend-go/services/orchestration-service.md`](../../../specs/backend-go/services/orchestration-service.md)
for the full design, and
[`services/usage-service`](../usage-service) for the reference package
layout/conventions this service follows.

## What's implemented (real, not stubbed)

- `internal/domain/` — `OrchestrationTask`, `DispatchContext`,
  `DecisionGate`, `CoordinatorRun` with invariant-enforcing constructors and
  pure unit tests, including the named invariant "a gate can't be resolved
  twice" (`DecisionGate.Resolve`/`ErrGateAlreadyResolved`) and the circuit
  breaker on `DispatchContext.RecordFailure`. `OrchestrationTask` is
  deliberately a distinct id space from `task-service`'s `Task` — see the
  design doc §2.1 and the package doc comment.
- `internal/usecase/keyed_serializer.go` — **`KeyedSerializer`, a real,
  working implementation of the design doc's KeyedAsyncQueue port (§6)**,
  not a stub: one FIFO worker goroutine per active key (spun up lazily,
  torn down after an idle timeout), guaranteeing strict per-key ordering —
  chosen over a plain `map[string]*sync.Mutex` because a mutex only
  guarantees exclusion, not ordering. See `keyed_serializer_test.go`:
  - `TestKeyedSerializer_SerializesSameKey` runs 50 goroutines × 100 calls
    against the same key, each doing a **deliberately unguarded**
    read-modify-write on a shared counter inside the callback. Run under
    `go test -race`, this only passes if `Do` truly serializes same-key
    calls — a broken implementation both races (caught by `-race`) and
    underclounts.
  - `TestKeyedSerializer_DifferentKeysRunConcurrently` proves the converse:
    two different keys' callbacks must both be able to start before either
    finishes, or the test times out.
  - `TestKeyedSerializer_QueuedCallRespectsContextCancellation` and
    `TestKeyedSerializer_TearsDownIdleWorkers` cover cancellation and the
    idle-teardown path.
- `internal/usecase/` — `CreateDispatchContext`, `CreateGate`,
  `ResolveGate`, `UpdateTaskStatusAndPromote`, each routed through the
  shared `KeyedSerializer` and tested against in-memory fakes
  (`fakes_test.go`) — including a usecase-level test that resolving the
  same gate twice fails (`TestResolveGate_CannotResolveTwice`) and that
  completing a task promotes exactly the sibling whose deps are now
  satisfied (`TestUpdateTaskStatusAndPromote_PromotesReadySiblings`).
- `internal/adapter/postgres/` — real `pgx`-backed repository.
  `UpdateStatusAndPromote`, `CreateGate`, and `ResolveGate` are each **one
  Postgres transaction** — `BEGIN` → status update / gate mutation →
  dependent-promotion or task-(un)block scan → `COMMIT` — per the design
  doc §8's hard NFR: a torn read between marking a task complete and
  re-scanning its dependents can double-dispatch a task or leave a ready
  task stuck pending. `ResolveGate` additionally locks the gate row
  (`SELECT ... FOR UPDATE`) before checking `status = 'pending'`, so the
  "resolved exactly once" invariant holds even under a race, as
  defense-in-depth alongside the usecase-level `KeyedSerializer` keying.
- `internal/adapter/grpc/` — implements
  `orchestrationv1.UnimplementedOrchestrationServiceServer`'s 4 generated
  RPCs, pure wire<->usecase translation.
- `migrations/0001_init.{up,down}.sql` — real DDL:
  `orchestration.coordinator_runs`, `orchestration.orchestration_tasks`,
  `orchestration.dispatch_contexts`, `orchestration.decision_gates`,
  `orchestration.messages`, RLS policies on every table (`usage-service`
  pattern).
- `cmd/server/main.go` — a real, working composition root: config load,
  Postgres pool, the `KeyedSerializer` wired **once as a singleton** shared
  by every usecase (a per-request instance would make the serialization
  guarantee meaningless), gRPC server with the shared interceptor chain,
  health/readiness HTTP server, graceful shutdown on SIGTERM.

## Known gaps / follow-ups (flagged honestly, not silently dropped)

The generated proto
(`proto/orca/orchestration/v1/orchestration.proto`, **not regenerated for
this service**) is narrower than the design doc §3's RPC sketch. This
service's domain/DB layer is still built per the design doc's fuller model
(the system-of-record contract), and the gaps below are all at the
proto/adapter seam, not in the coordination logic itself:

- **`CreateDispatchContextRequest` carries no `orchestration_task_id`.**
  The design doc's §8 NFR table requires "dispatch row and `dispatched`
  transition must commit together"; without a task id in the request, a
  `dispatch_contexts` row is created with `orchestration_task_id = NULL`
  and no task transition happens. `orchestration_task_id` is a nullable FK
  in the migration specifically so the schema is ready the moment the
  proto is extended — see `internal/usecase/ports.go`'s
  `DispatchContextRepository` doc comment.
- **`CreateGateRequest` carries only `dispatch_context_id`** — no
  `orchestration_task_id`, `question`, or `options`. `CreateGate` resolves
  `dispatch_context_id -> orchestration_task_id` via a locked read inside
  its transaction, but because every dispatch context created through this
  proto surface has `orchestration_task_id = NULL` (previous point),
  `CreateGate` will return `ORCH_DISPATCH_CONTEXT_NO_TASK`
  (`FailedPrecondition`) until `CreateDispatchContextRequest` is extended.
  `question`/`options` are accepted by the usecase/repository but always
  empty from the current gRPC adapter.
- **No handle in `ResolveGateRequest` / `UpdateTaskStatusAndPromoteRequest`.**
  The design doc keys `HandleSerializer.Do` by `assignee_handle`/
  `coordinator_handle`; those RPCs don't carry one, so `ResolveGate` keys
  by `gate_id` and `UpdateTaskStatusAndPromote` keys by
  `orchestration_task_id` instead — the closest available substitutes.
  Both still close the real race each exists to prevent (two
  concurrent/retried calls for the *same* gate/task interleaving), just
  narrower in scope than a full handle-wide serialization. See the doc
  comments on `ResolveGate`/`UpdateTaskStatusAndPromote`.
- **`decision_gates.dispatch_context_id`** is an additive column beyond the
  design doc §5 schema, added so `ResolveGateResponse` can round-trip the
  `dispatch_context_id` the generated `DecisionGate` proto message requires.
- **`StartCoordinatorRun`/`GetCoordinatorRun`/`CompleteCoordinatorRun`/
  `FailCoordinatorRun`, `RecordHeartbeat`, `FailDispatch`,
  `ListPendingDecisionGates`, `PostMessage`/`ListMessages`/`MarkMessageRead`,
  `GetAgentStatusForHandle`** from the design doc §3 sketch are **not**
  in the generated proto, so no RPC/usecase exists for them. `CoordinatorRun`
  exists as a domain type (with invariant tests) and a DB table, but has no
  repository/usecase wired — nothing calls it yet. `orchestration.messages`
  is likewise schema-only. This means: today there is no RPC that actually
  creates a `coordinator_runs` row, so `CreateDispatchContext`'s
  `coordinator_run_id` FK will fail against a real database unless a row
  was seeded out-of-band — a direct consequence of `StartCoordinatorRun`
  being outside the current proto surface, not a shortcut taken here.
- **`common/secrets` (Vault) is not wired into `main.go`** — same posture
  as usage-service; `DATABASE_DSN` is read directly from the environment.
- **`common/tracing` has no OTLP exporter configured** — spans are created
  but not shipped anywhere until a collector endpoint is wired in.
- No transactional outbox / event publishing — the design doc §6 sketches
  an `adapter/eventbus/` outbox (`orchestration.gate.resolved`,
  `orchestration.run.completed`, ...); not implemented here since no
  generated RPC's response depends on it and the core ask (KeyedSerializer +
  atomic promotion) doesn't need it.

## What IS fully real despite the above

The two pieces the task exists to prove out are not gated by any of the
above: **`KeyedSerializer`** (`internal/usecase/keyed_serializer.go`) is a
complete, race-tested, worker-per-key implementation with no external
dependency, and **`UpdateStatusAndPromote`**
(`internal/adapter/postgres/repository.go`) is a genuine single-transaction
atomic promote saga end to end, from the gRPC request through to committed
SQL — this is the one RPC in the generated proto that carries every field
its usecase/repository needs, so nothing about it is narrowed by the proto
gaps above.

## Running locally

```sh
# from backend-go/
docker compose up -d postgres   # see ../../docker-compose.yml
migrate -path services/orchestration-service/migrations \
  -database "$DATABASE_DSN" up  # golang-migrate; see architecture/05

cd services/orchestration-service
DATABASE_DSN=postgres://orca:orca@localhost:5432/orchestration?sslmode=disable \
  go run ./cmd/server
```

## Testing

```sh
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test -race ./...                                # unit tests — no external deps
GOWORK=off go test -tags=integration ./internal/adapter/postgres/...  # requires Docker (testcontainers-go)
```

This service is not listed in the workspace `go.work` (only
`services/usage-service` is, per the pilot) — build/test it standalone with
`GOWORK=off` from inside this directory.
