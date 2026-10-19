package main

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// requestLifecycle holds the CR-REQ-003/006 use cases. Transition is the stable door other
// features (CR-REQ-004/005/007/009/013) use to change Request.status.
type requestLifecycle struct {
	Transition   usecase.RequestTransitioner
	GetFlow      *usecase.GetRequestFlow
	Return       *usecase.ReturnRequestToBacklog
	Reopen       *usecase.ReopenRequest
	Cancel       *usecase.CancelRequest
	SpawnChild   *usecase.SpawnChildRequest
	ListLinks    *usecase.ListRequestLinks
	ChildCreator usecase.ChildRequestCreator
	// Canceller closes pending approvals of a request; solution regeneration uses it too.
	Canceller usecase.ApprovalCanceller
}

// wireRequestLifecycle builds the lifecycle use cases over the shared stores.
// The execution guard is task-service-backed (CR-REQ-013) and fails closed; the classification-attempts resetter is nil until CR-REQ-005 adds the counter column.
func wireRequestLifecycle(stores *requestStores, registry *usecase.SubjectHandlerRegistry, guard usecase.ExecutionGuard) *requestLifecycle {
	transition := usecase.NewTransitionRequest(stores.requests, stores.txScope, stores.outboxWriter)
	canceller := &usecase.PendingApprovalCanceller{Inner: &usecase.CancelPendingApprovalsForRequest{
		Repo: stores.approvals, Tx: stores.tx, Locker: stores.locker, Registry: registry, Outbox: stores.outboxWriter,
	}}
	creator := usecase.NewIdempotentChildCreator(stores.requests, stores.idempotency, stores.outboxWriter)
	return &requestLifecycle{
		Transition:   transition,
		GetFlow:      usecase.NewGetRequestFlow(),
		Return:       usecase.NewReturnRequestToBacklog(stores.requests, transition, stores.returns, canceller, guard, stores.tx, stores.outboxWriter),
		Reopen:       usecase.NewReopenRequest(stores.requests, transition, stores.returns, nil, stores.tx),
		Cancel:       usecase.NewCancelRequest(stores.requests, transition, stores.returns, canceller, guard, stores.tx),
		SpawnChild:   usecase.NewSpawnChildRequest(stores.requests, stores.links, creator, stores.tx),
		ListLinks:    usecase.NewListRequestLinks(stores.requests, stores.links),
		ChildCreator: creator,
		Canceller:    canceller,
	}
}

// baseRefWiring is the CR-REQ-039 write path and the read-side lock flag.
type baseRefWiring struct {
	Set     *usecase.SetRequestBaseRef
	Locks   *usecase.BaseRefLockAnnotator
	Checker usecase.BaseRefChecker
}

// wireBaseRef builds SetRequestBaseRef and the lock annotator over the shared stores. A missing port is a startup error, not a
// disabled feature: a server that quietly skipped the worktree lock check could let a base ref change under a running Plan.
func wireBaseRef(log *slog.Logger, stores *requestStores, tasks usecase.TaskClient, checker usecase.BaseRefChecker) (*baseRefWiring, error) {
	switch {
	case stores.baseRefs == nil:
		return nil, errors.New("base ref wiring: the request store has no base ref writer")
	case stores.worktreeRecords == nil:
		return nil, errors.New("base ref wiring: no worktree presence store")
	case tasks == nil:
		return nil, errors.New("base ref wiring: no task client to see task worktrees")
	case checker == nil:
		return nil, errors.New("base ref wiring: no base ref checker")
	}
	presence := &usecase.TaskAwareWorktreePresence{Stored: stores.worktreeRecords, Tasks: tasks}
	set, err := usecase.NewSetRequestBaseRef(usecase.SetRequestBaseRefDeps{
		Requests: stores.requests, Locker: stores.locker, Writer: stores.baseRefs, Presence: presence, Checker: checker,
		Tx: stores.tx, Outbox: stores.outboxWriter,
	})
	if err != nil {
		return nil, fmt.Errorf("base ref wiring: %w", err)
	}
	return &baseRefWiring{Set: set, Locks: &usecase.BaseRefLockAnnotator{Presence: presence, Log: log}, Checker: checker}, nil
}
