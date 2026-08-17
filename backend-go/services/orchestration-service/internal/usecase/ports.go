// Package usecase holds orchestration-service's application services and
// the ports they need — defined here, implemented in internal/adapter/*,
// per the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
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

// OrchestrationTaskRepository is the persistence port for this service's
// own DAG-node table (orchestration_tasks) — a distinct id space from
// task-service's tasks table, see orchestration-service.md §2.1.
type OrchestrationTaskRepository interface {
	Create(ctx context.Context, task domain.OrchestrationTask) (domain.OrchestrationTask, error)
	Get(ctx context.Context, tenantID, id string) (domain.OrchestrationTask, error)
	// UpdateStatusAndPromote is the atomic promote saga
	// (orchestration-service.md §8): update the task's status and, in the
	// SAME database transaction, promote any pending siblings (same
	// coordinator_run_id) whose deps are now all completed. A torn read
	// between marking complete and re-scanning dependents can otherwise
	// double-dispatch a task or leave a ready task stuck pending — this is
	// a hard NFR, not a convention callers must remember. Returns the
	// updated task and the ids of any promoted siblings.
	UpdateStatusAndPromote(ctx context.Context, tenantID, taskID string, newStatus domain.TaskStatus) (domain.OrchestrationTask, []string, error)
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
	// handle/coordinatorRunID.
	//
	// NOTE: the generated CreateDispatchContextRequest proto message does
	// not carry an orchestration_task_id (see README "Known gaps"), so this
	// single INSERT is trivially atomic on its own — the §8 "dispatch row
	// and the task's dispatched transition must commit together" chain
	// only becomes reachable once the proto is extended with a task id.
	CreateDispatchContext(ctx context.Context, tenantID, handle, coordinatorRunID string) (domain.DispatchContext, error)
}

// GateRepository is the persistence port for decision gates.
type GateRepository interface {
	// CreateGate atomically resolves dispatchContextID to its owning
	// orchestration_task_id, inserts the gate row, and transitions that
	// task to blocked — all in one transaction (§8: "gate creation and the
	// task's blocked transition must commit together, or the task can be
	// dispatched past a checkpoint meant to stop it").
	CreateGate(ctx context.Context, tenantID, dispatchContextID, question string, options []string) (domain.DecisionGate, error)
	// ResolveGate atomically transitions the gate to resolved and unblocks
	// its owning task — all in one transaction (§8: "resolution, unblock,
	// and the promotion pass must commit together"). Returns the resolved
	// gate and the ids of any tasks whose status changed as a result (at
	// least the gate's own owning task).
	ResolveGate(ctx context.Context, tenantID, gateID, resolution string) (domain.DecisionGate, []string, error)
}
