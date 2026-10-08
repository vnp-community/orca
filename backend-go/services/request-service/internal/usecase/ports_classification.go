package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ErrNoDevServer means no reachable dev server could run the AI call.
var ErrNoDevServer = errors.New("no dev server available for AI classification")

// ErrClassifierTimeout is returned when the AI call exceeded its deadline.
var ErrClassifierTimeout = errors.New("classifier timed out")

type ClassificationInput struct {
	RequestID string
	ProjectID string
	Title     string
	Body      string
	IssueType string
	Labels    []string
}

// RequestClassifier asks the AI for a proposal. Implementations return
// domain.ErrProposalInvalid for unusable output, ErrNoDevServer or ErrClassifierTimeout.
type RequestClassifier interface {
	Classify(ctx context.Context, in ClassificationInput) (domain.ClassificationProposal, error)
}

type ProcessedEventRepository interface {
	// MarkProcessed joins the ctx transaction; alreadyProcessed means a previous delivery won.
	MarkProcessed(ctx context.Context, eventID, subject string) (alreadyProcessed bool, err error)
	Prune(ctx context.Context, olderThan time.Time) (int64, error)
}

// ClassificationRunRepository keeps the durable record of background classifications.
type ClassificationRunRepository interface {
	// Start inserts the run and returns it (started=true). started=false returns the run that
	// blocked it instead: same source event, or another run already live for the request.
	Start(ctx context.Context, run domain.ClassificationRun) (owner domain.ClassificationRun, started bool, err error)
	Get(ctx context.Context, runID string) (domain.ClassificationRun, error)
	Renew(ctx context.Context, runID, owner string, until time.Time) (bool, error)
	// Finish ends a running run; it joins the ctx transaction so it commits with the result.
	Finish(ctx context.Context, runID string, status domain.ClassificationRunStatus, errCode string) error
	// ClaimExpired takes over runs whose lease expired, across tenants. Runs already claimed
	// MaxClassificationRunClaims times are failed instead of returned.
	ClaimExpired(ctx context.Context, owner string, until time.Time, batch int) ([]domain.ClassificationRun, error)
}

// ApprovalRecorder opens and settles the request_type approval. No-op until CR-REQ-009.
type ApprovalRecorder interface {
	RequestTypeApproval(ctx context.Context, requestID string) error
	Approve(ctx context.Context, requestID, actorID string) error
}

type NoopApprovalRecorder struct{}

func (NoopApprovalRecorder) RequestTypeApproval(context.Context, string) error { return nil }
func (NoopApprovalRecorder) Approve(context.Context, string, string) error     { return nil }

type NoopApprovalCanceller struct{}

func (NoopApprovalCanceller) CancelPending(context.Context, string, string) error { return nil }

type NoopExecutionGuard struct{}

func (NoopExecutionGuard) HasActiveExecution(context.Context, string) (bool, error) {
	return false, nil
}
