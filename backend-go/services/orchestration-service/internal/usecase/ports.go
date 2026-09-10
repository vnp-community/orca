// Package usecase holds orchestration-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
)

// Sentinel errors a Repository implementation returns for the "not found" /
// "precondition failed" cases a usecase needs to distinguish from a generic
// internal failure when mapping to apperrors.Kind at the gRPC boundary.
var (
	ErrTaskNotFound             = errors.New("usecase: orchestration task not found")
	ErrDispatchContextNotFound  = errors.New("usecase: dispatch context not found")
	ErrDispatchContextHasNoTask = errors.New("usecase: dispatch context has no owning orchestration task yet")
	ErrGateNotFound             = errors.New("usecase: decision gate not found")
	ErrGateNotPending           = errors.New("usecase: decision gate is not pending")
	// ErrRunNotFound is CoordinatorRunRepository's not-found sentinel, mirroring
	// ErrTaskNotFound/ErrDispatchContextNotFound/ErrGateNotFound's existing shape.
	ErrRunNotFound = errors.New("usecase: coordinator run not found")
)

// HandleSerializer is the KeyedAsyncQueue port from
// specs/backend-go/services/orchestration-service.md §6: it serializes
// concurrent calls sharing the same key (typically an assignee_handle or
// coordinator_handle) while letting different keys run concurrently. The
// production implementation is *KeyedSerializer (this package,
// keyed_serializer.go) — a real worker-per-key goroutine pool, not a stub.
// Unit tests substitute a synchronous fake (Do calls fn immediately).
type HandleSerializer interface {
	Do(ctx context.Context, key string, fn func() error) error
}

// UpdateStatusAndPromoteResult adds RunFinalized to the (task, promotedIDs)
// return shape — a *RunFinalization is non-nil only when this call's status
// write made it the LAST non-terminal task in its coordinator_run_id, i.e.
// the run just finished. Lives in usecase (not internal/adapter/postgres)
// because usecase cannot import postgres (dependency-inversion direction),
// so the concrete Repository constructs this type instead of the other way
// around.
type UpdateStatusAndPromoteResult struct {
	Task         domain.OrchestrationTask
	PromotedIDs  []string
	RunFinalized *RunFinalization
}

// RunFinalization carries what the usecase layer needs to call
// TaskServiceReporter.ReportResult — deliberately NOT domain.CoordinatorRun
// itself (this tail only knows the run id, its origin_task_id, and whether
// it succeeded — it does not re-fetch the full run row inside the same
// transaction, avoiding an extra round-trip for data the reporter call
// doesn't need).
type RunFinalization struct {
	CoordinatorRunID string
	OriginTaskID     string
	Success          bool
}

// OrchestrationTaskRepository is the persistence port for this service's
// own DAG-node table (orchestration_tasks) — a distinct id space from
// task-service's tasks table, see orchestration-service.md §2.1.
type OrchestrationTaskRepository interface {
	Create(ctx context.Context, task domain.OrchestrationTask) (domain.OrchestrationTask, error)
	Get(ctx context.Context, tenantID, id string) (domain.OrchestrationTask, error)
	// UpdateStatusAndPromote is the atomic promote saga
	// (orchestration-service.md §8): update the task's status and, in the
	// SAME database transaction, promote any pending siblings (same
	// coordinator_run_id) whose deps are now all completed, AND detect
	// whether this write finalized the owning coordinator_run (every
	// sibling now terminal). A torn read between marking complete and
	// re-scanning dependents/run-completion can otherwise double-dispatch a
	// task, leave a ready task stuck pending, or leave a fully-completed
	// run stuck running forever — this is a hard NFR, not a convention
	// callers must remember.
	//
	// event (BE-SOL-003/TASK-FT-003-01) is enqueued into
	// orchestration.outbox_events in the SAME transaction as the status
	// write — a zero-value domain.OutboxEvent{} (ID == "") skips the
	// enqueue, e.g. when the caller's payload marshal failed.
	UpdateStatusAndPromote(ctx context.Context, tenantID, taskID string, newStatus domain.TaskStatus, event domain.OutboxEvent) (UpdateStatusAndPromoteResult, error)

	// ListReadyUnclaimed returns 'ready' orchestration_tasks across ALL
	// tenants/runs that have no dispatch_contexts row yet — the tick
	// loop's (TASK-TASKV1-005-10) outer scan. "Claim" (transition ready ->
	// dispatched) happens per-task inside ClaimReady below via a CAS
	// UPDATE, not here, so two concurrent tick instances listing the same
	// task is safe: only one's claim UPDATE succeeds.
	ListReadyUnclaimed(ctx context.Context) ([]domain.OrchestrationTask, error)

	// ClaimReady atomically transitions ONE task from 'ready' to
	// 'dispatched' (UPDATE ... WHERE status='ready' RETURNING, i.e.
	// compare-and-swap) — returns (task, true, nil) on success, (zero,
	// false, nil) if another tick instance already claimed it (not an
	// error: expected under concurrent ticks, same "ok to lose the race"
	// shape as sql.ErrNoRows-as-a-bool elsewhere in this codebase).
	ClaimReady(ctx context.Context, tenantID, taskID string) (domain.OrchestrationTask, bool, error)

	// CountNonTerminalByRun returns how many orchestration_tasks under
	// coordinatorRunID are NOT in a terminal status (completed/failed) —
	// UpdateTaskStatusAndPromote's extended transaction
	// (TASK-TASKV1-005-07) uses this to decide "is this run done" without
	// a second round-trip after COMMIT, which would reopen the exact
	// torn-read race §8 exists to close.
	CountNonTerminalByRun(ctx context.Context, tenantID, coordinatorRunID string) (int, error)
}

// CoordinatorRunRepository is the persistence port for the top-level
// "complex path" coordination session — new port, mirrors the existing
// repositories' naming/atomicity discipline (ports.go's DispatchContextRepository/
// GateRepository doc comments).
type CoordinatorRunRepository interface {
	// CreateWithTasks atomically inserts the coordinator_runs row (status
	// running) AND every OrchestrationTask domain.ExpandSpec(run.Spec)
	// produces — resolving each task's Deps from SpecNode TempIDs to the
	// freshly-minted real ids WITHIN the same transaction. A torn write
	// here would leave a run with unresolvable deps, permanently stuck
	// (same class of bug §8 guards UpdateStatusAndPromote against).
	CreateWithTasks(ctx context.Context, tenantID string, run domain.CoordinatorRun) (domain.CoordinatorRun, error)
	// GetRun is named distinctly from OrchestrationTaskRepository.Get (not
	// "Get") because a single concrete Repository (internal/adapter/postgres)
	// implements both interfaces — Go has no method overloading, so a
	// same-named method with a different return type on the same struct
	// does not compile. Same naming-collision discipline this file's
	// DispatchContextRepository doc comment already documents for
	// CreateDispatchContext vs Create.
	GetRun(ctx context.Context, tenantID, id string) (domain.CoordinatorRun, error)
	// Complete/Fail are single-row updates (no cross-table write), unlike
	// CreateWithTasks — simple, not wrapped in the txn helper.
	Complete(ctx context.Context, tenantID, id string, result json.RawMessage) (domain.CoordinatorRun, error)
	Fail(ctx context.Context, tenantID, id, errMsg string) (domain.CoordinatorRun, error)
	// ListRunning backs the tick loop's outer scan — every run this
	// instance should be advancing. Scoped by status only (not tenant):
	// the tick loop runs as the service's own internal identity, not a
	// per-request caller, so it must see every tenant's running runs.
	ListRunning(ctx context.Context) ([]domain.CoordinatorRun, error)
	// MarkReported sets reported_at = now() once TaskServiceReporter.ReportResult
	// succeeds for a terminal run — a single-column update, called from
	// UpdateTaskStatusAndPromote's post-commit reporting step
	// (TASK-TASKV1-005-07), never inside the same transaction that
	// finalized the run (a cross-service report must already have
	// succeeded before this is called).
	MarkReported(ctx context.Context, tenantID, id string) error
	// ListUnreportedTerminal returns every completed/failed run with
	// reported_at still NULL — backs the tick loop's retry pass
	// (TASK-TASKV1-005-10) for a ReportResult call that failed the first
	// time. Reuses idx_coordinator_runs_unreported (TASK-TASKV1-005-01),
	// scoped by status only, same "internal identity, not tenant-scoped"
	// reasoning as ListRunning.
	ListUnreportedTerminal(ctx context.Context) ([]domain.CoordinatorRun, error)
}

// WorkerDispatcher is the port the tick loop (TASK-TASKV1-005-10) calls to
// hand a ready OrchestrationTask to a terminal-hosted AI-agent worker via
// infra-fleet-service -> Dev Server Agent — orchestration-service.md §7's
// "resolve connectionId for assignee_handle... sync gRPC per dispatch."
// This does not decide WHICH handle to assign (the tick loop does) — it
// only performs the dispatch call for an already-chosen handle. Concrete
// adapter: TASK-TASKV1-005-08.
type WorkerDispatcher interface {
	Dispatch(ctx context.Context, tenantID string, task domain.OrchestrationTask, handle string) error
}

// TaskServiceReporter is the outbound port for reporting a coordinator
// run's terminal state back to task-service — the SERVER side of SOL-TG-04's
// ReportTaskExecutionResult RPC is task-service's own scope (TASK-TG-04-05,
// not yet built as of this writing — grep confirms no
// ReportTaskExecutionResult symbol exists anywhere in backend-go yet);
// this is orchestration-service's client stub calling it. Concrete
// adapter: TASK-TASKV1-005-08.
type TaskServiceReporter interface {
	ReportResult(ctx context.Context, taskID, coordinatorRunID string, success bool, errMsg string) error
}

// DispatchContextRepository is the persistence port for dispatch attempts.
//
// Its methods are named distinctly from OrchestrationTaskRepository's and
// GateRepository's (CreateDispatchContext, not Create) because a single
// concrete Repository (internal/adapter/postgres) implements all three
// interfaces — Go has no method overloading, so the port names must not
// collide even though each interface is conceptually scoped to one entity.
type DispatchContextRepository interface {
	// CreateDispatchContext inserts a dispatch_context row for
	// handle/coordinatorRunID, optionally owned by orchestrationTaskID.
	//
	// orchestrationTaskID may be empty — an ad-hoc coordinator-only dispatch
	// (no task yet) legitimately has none, and dispatch_contexts.
	// orchestration_task_id is a nullable FK for exactly that reason. Per
	// docs/execution-plan.md Epic C, the generated CreateDispatchContextRequest
	// proto message now carries orchestration_task_id, so a caller that does
	// supply one gets it persisted here — the precondition CreateGate needs
	// to resolve dispatch_context_id -> orchestration_task_id instead of
	// failing closed with ORCH_DISPATCH_CONTEXT_NO_TASK. This single INSERT
	// remains trivially atomic on its own (no separate task-status
	// transition happens here).
	//
	// userID may be empty — a dispatch context created by a system process
	// with no authenticated end-user caller legitimately has none (see
	// domain.DispatchContext.UserID's doc comment).
	//
	// worktreeID may also be empty — an ad-hoc dispatch with no worktree
	// association legitimately has none (see
	// domain.DispatchContext.WorktreeID's doc comment).
	//
	// event (BE-SOL-003/TASK-FT-003-01) is enqueued into
	// orchestration.outbox_events in the SAME transaction as the insert — a
	// zero-value domain.OutboxEvent{} (ID == "") skips the enqueue.
	CreateDispatchContext(ctx context.Context, tenantID, userID, worktreeID, handle, coordinatorRunID, orchestrationTaskID string, event domain.OutboxEvent) (domain.DispatchContext, error)

	// GetLatestForTask returns the most recently created dispatch_contexts
	// row for orchestrationTaskID, or ErrDispatchContextNotFound if none
	// exists. A task's dispatch_contexts row is not unique (retries after
	// failure create new rows, §8's circuit-breaker note) — "latest" is
	// the current dispatch, which is what dispatchShow's "which terminal
	// is this on" question actually needs, not full attempt history.
	GetLatestForTask(ctx context.Context, tenantID, orchestrationTaskID string) (domain.DispatchContext, error)

	// ListActiveDispatchContextsForUser returns every dispatch_contexts row
	// for (tenantID, userID) not in a terminal status (completed/failed/
	// circuit_broken excluded) — backs CR-STORAGE-006/007's
	// "agentSession.listActive" hydrate. See
	// docs/backlog/BACKLOG-006-dispatch-context-user-linkage-decision.md.
	ListActiveDispatchContextsForUser(ctx context.Context, tenantID, userID string) ([]domain.DispatchContext, error)

	// RecordDispatchFailure loads dispatchContextID (locked, tenant-scoped),
	// applies domain.DispatchContext.RecordFailure(reason) — incrementing
	// failure_count and tripping the circuit breaker at the threshold — and
	// persists the result, atomically. Returns ErrDispatchContextNotFound if
	// no such row exists for tenantID. See
	// docs/backlog/BACKLOG-009-orchestration-service-fail-dispatch-missing.md
	// for why this port and its caller (usecase.FailDispatch) didn't exist
	// before.
	RecordDispatchFailure(ctx context.Context, tenantID, dispatchContextID, reason string) (domain.DispatchContext, error)

	// RecordHeartbeat updates last_heartbeat_at = now() for
	// dispatchContextID — a single-column UPDATE, no transaction needed
	// (§8: "no cross-service calls on that path," p99 < 30ms budget rules
	// out wrapping this in the heavier per-usecase txn+serializer pattern
	// used elsewhere in this file).
	RecordHeartbeat(ctx context.Context, tenantID, dispatchContextID string) (domain.DispatchContext, error)

	// GetDispatchContext returns one dispatch_contexts row by id — added
	// for CreateGate (BE-SOL-003/TASK-FT-003-02) to resolve
	// dispatchContextID -> orchestration_task_id -> origin_task_id for the
	// decision_gate.opened outbox payload, ahead of (and independent from)
	// GateRepository.CreateGate's own locked read of the same row inside
	// its transaction. Named GetDispatchContext, not Get, for the same
	// naming-collision reason CreateDispatchContext isn't named Create —
	// one struct implements this port and OrchestrationTaskRepository's
	// own differently-typed Get.
	GetDispatchContext(ctx context.Context, tenantID, id string) (domain.DispatchContext, error)
}

// GateRepository is the persistence port for decision gates.
type GateRepository interface {
	// CreateGate atomically resolves dispatchContextID to its owning
	// orchestration_task_id, inserts the gate row, and transitions that
	// task to blocked — all in one transaction (§8: "gate creation and the
	// task's blocked transition must commit together, or the task can be
	// dispatched past a checkpoint meant to stop it"). Also inserts the
	// gate's first orchestration.messages row (BE-SOL-003/TASK-FT-003-02 —
	// see that task's Context for why this table's first real write lives
	// here) and enqueues event into orchestration.outbox_events, both in
	// the SAME transaction; a zero-value event skips the outbox enqueue.
	CreateGate(ctx context.Context, tenantID, dispatchContextID, question string, options []string, event domain.OutboxEvent) (domain.DecisionGate, error)
	// ResolveGate atomically transitions the gate to resolved and unblocks
	// its owning task — all in one transaction (§8: "resolution, unblock,
	// and the promotion pass must commit together"). Returns the resolved
	// gate and the ids of any tasks whose status changed as a result (at
	// least the gate's own owning task). Also inserts a minimal
	// orchestration.messages row (TASK-FT-003-02) — no outbox event: the
	// CR names no `.resolved` subject, only `.opened`.
	ResolveGate(ctx context.Context, tenantID, gateID, resolution string) (domain.DecisionGate, []string, error)

	// ListPending returns every pending decision_gates row for tenantID,
	// backing ListPendingDecisionGates and reusing the existing
	// idx_gates_pending partial index (0001_init.up.sql:91) — that index
	// was created for exactly this query and has had zero callers until now.
	ListPending(ctx context.Context, tenantID string) ([]domain.DecisionGate, error)
}
