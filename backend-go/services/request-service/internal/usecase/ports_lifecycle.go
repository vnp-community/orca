package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestTransitioner is the one door through which any feature changes Request.status
// (CR-REQ-004/005/007/009/013 call it); implemented by *TransitionRequest.
// Idempotent redelivery: set ExpectedFrom to the status the caller saw, and a repeat of the
// same (ExpectedFrom, Trigger) returns Applied=false without writing or emitting anything.
type RequestTransitioner interface {
	Execute(ctx context.Context, in TransitionInput) (TransitionResult, error)
}

var _ RequestTransitioner = (*TransitionRequest)(nil)

// TxScope is a TxRunner that can tell whether ctx already carries a transaction, so the
// outermost caller (and only it) may retry a lost CAS in a fresh transaction.
type TxScope interface {
	TxRunner
	InTransaction(ctx context.Context) bool
}

// ReturnHistoryRepository is append-only: requests keeps the latest return, this keeps every one.
type ReturnHistoryRepository interface {
	Append(ctx context.Context, e domain.ReturnHistoryEntry) error
	List(ctx context.Context, requestID string) ([]domain.ReturnHistoryEntry, error)
}

// ApprovalCanceller closes pending approvals when a Request leaves the flow.
type ApprovalCanceller interface {
	CancelPending(ctx context.Context, requestID, why string) error
}

// ExecutionGuard reports whether a Request still has running Tasks; return and cancel are
// refused while it does. The real implementation needs task-service (CR-REQ-011/013).
type ExecutionGuard interface {
	HasActiveExecution(ctx context.Context, requestID string) (bool, error)
}

// ClassificationAttemptsResetter zeroes the AI classification attempt counter when a request is
// reopened. The counter column belongs to CR-REQ-005; ReopenRequest skips the call while it is nil.
type ClassificationAttemptsResetter interface {
	ResetClassificationAttempts(ctx context.Context, requestID string) error
}

// ChildRequestInput is what SpawnChildRequest hands to the creator inside its transaction.
type ChildRequestInput struct {
	ProjectID       string
	Title           string
	Body            string
	Provider        domain.SourceProvider
	ReporterID      string
	IdempotencyKey  string
	TypeHint        domain.RequestType
	ParentRequestID string
	LinkReason      domain.LinkReason
}

type ChildRequestResult struct {
	Request domain.Request
	Created bool
}

// ChildRequestCreator creates a Request inside the caller's transaction and is idempotent on
// IdempotencyKey (Created=false returns the earlier child). CR-REQ-004's CreateWithinTx can replace the default.
type ChildRequestCreator interface {
	CreateChild(ctx context.Context, in ChildRequestInput) (ChildRequestResult, error)
}

// NoActiveExecutionGuard is the default until task-service exposes running-task state:
// it never blocks. Wiring it explicitly keeps that gap visible instead of hiding a nil check.
type NoActiveExecutionGuard struct{}

func (NoActiveExecutionGuard) HasActiveExecution(context.Context, string) (bool, error) {
	return false, nil
}
