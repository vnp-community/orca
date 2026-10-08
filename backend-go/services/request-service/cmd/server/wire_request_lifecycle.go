package main

import "github.com/stablyai/orca-go/services/request-service/internal/usecase"

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
