package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// StoredReview is a review row. Model is the JSON of domain.OutcomeReview; the other columns are what the service queries by.
type StoredReview struct {
	ID            string
	TenantID      string
	RequestID     string
	Phase         domain.ReviewPhase
	HeadKey       string
	HeadCommit    string
	BaseCommit    string
	InputsDigest  string
	State         domain.ReviewState
	Model         []byte
	KGSnapshot    string
	DesignDigest  string
	ContextDigest string
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int64
}

// OutcomeReviewRepository stores reviews. Every call is scoped to the tenant of the context.
type OutcomeReviewRepository interface {
	// Create inserts the row; an existing row for (request, phase, head key) is domain.ErrReviewAlreadyExists.
	Create(ctx context.Context, r StoredReview) (StoredReview, error)
	Get(ctx context.Context, id string) (StoredReview, error)
	GetByKey(ctx context.Context, requestID string, phase domain.ReviewPhase, headKey string) (StoredReview, bool, error)
	// Latest returns the newest review of the Request that is not superseded; ok is false when there is none.
	Latest(ctx context.Context, requestID string) (StoredReview, bool, error)
	// List returns newest first. After is the id of the last row of the previous page.
	List(ctx context.Context, requestID string, pageSize int, afterID string) ([]StoredReview, error)
	// Update replaces model, digests and state when the stored version equals expectedVersion and bumps the version; a mismatch is
	// domain.ErrReviewStaleRevision.
	Update(ctx context.Context, r StoredReview, expectedVersion int64) (StoredReview, error)
	// SetState changes only the state under the same version check.
	SetState(ctx context.Context, id string, state domain.ReviewState, expectedVersion int64) (StoredReview, error)
	// SupersedeOthers marks every other review of the Request superseded.
	SupersedeOthers(ctx context.Context, requestID, exceptID string) error
}

type ReviewRunKind string

const (
	ReviewRunContext ReviewRunKind = "context"
	ReviewRunDraw    ReviewRunKind = "draw"
)

type ReviewRunStatus string

const (
	ReviewRunQueued    ReviewRunStatus = "queued"
	ReviewRunRunning   ReviewRunStatus = "running"
	ReviewRunSucceeded ReviewRunStatus = "succeeded"
	ReviewRunFailed    ReviewRunStatus = "failed"
	ReviewRunCancelled ReviewRunStatus = "cancelled"
)

func (s ReviewRunStatus) Active() bool { return s == ReviewRunQueued || s == ReviewRunRunning }

type ReviewRun struct {
	ID           string
	TenantID     string
	ReviewID     string
	RequestID    string
	Kind         ReviewRunKind
	Status       ReviewRunStatus
	Scope        string
	AgentRunRef  string
	CostEstimate string
	CostActual   string
	ErrorCode    string
	StartedAt    time.Time
	FinishedAt   *time.Time
}

// OutcomeReviewRunRepository records agent runs. At most one run of a kind is active per Request.
type OutcomeReviewRunRepository interface {
	// Start inserts an active run; when one is already active it returns domain.ErrReviewRunInProgress carrying that run's id.
	Start(ctx context.Context, r ReviewRun) (ReviewRun, error)
	// Finish ends an active run and frees the slot.
	Finish(ctx context.Context, id string, status ReviewRunStatus, errorCode, costActual string) error
	Get(ctx context.Context, id string) (ReviewRun, error)
	// ActiveForRequest returns the active run of kind, if any.
	ActiveForRequest(ctx context.Context, requestID string, kind ReviewRunKind) (ReviewRun, bool, error)
	ListByReview(ctx context.Context, reviewID string) ([]ReviewRun, error)
	// CountSince counts runs of kind started since t for the Request; the regeneration limit reads it.
	CountSince(ctx context.Context, requestID string, kind ReviewRunKind, since time.Time) (int, error)
}
