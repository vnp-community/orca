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
	// ApprovedBy, ApprovedAt and ApprovalNote are set while State is approved and cleared when the review leaves that state.
	ApprovedBy   string
	ApprovedAt   *time.Time
	ApprovalNote string
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
	// Approve marks the review approved by the person, under the same version check; any other change of state clears the approval.
	Approve(ctx context.Context, id, by, note string, expectedVersion int64) (StoredReview, error)
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
	// Reason is why a person regenerated; empty for a run the system started. ClientKey makes a repeated request one run.
	Reason    string
	ClientKey string
	CreatedBy string
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
	// CountSince counts runs of kind started since t for the Request.
	CountSince(ctx context.Context, requestID string, kind ReviewRunKind, since time.Time) (int, error)
	// CountRegenerations counts the runs a person asked for (Reason set) since t, of any kind; the daily limit reads it.
	CountRegenerations(ctx context.Context, requestID string, since time.Time) (int, error)
	// CountActive counts the runs of the tenant that have not ended, for the limit on concurrent runs.
	CountActive(ctx context.Context) (int, error)
	// FindByClientKey returns the run started by an earlier request with the same key.
	FindByClientKey(ctx context.Context, requestID, clientKey string) (ReviewRun, bool, error)
}

// ReviewContextKind says what a stored context record holds.
type ReviewContextKind string

const (
	// ReviewContextDesign is the locked design: kept per Request and reused by the delivered phase and by later runs.
	ReviewContextDesign ReviewContextKind = "design"
	// ReviewContextResult is one whole checked context result, keyed by its context digest.
	ReviewContextResult ReviewContextKind = "context"
)

type ReviewContextRecord struct {
	ID            string
	TenantID      string
	RequestID     string
	Kind          ReviewContextKind
	Digest        string
	PromptVersion string
	Payload       []byte
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// ReviewContextStore keeps what a context run produced so the next review does not ask the agent again. Reads return only records
// that have not expired at the time given.
type ReviewContextStore interface {
	// Put inserts the record, or replaces the payload and expiry of the one with the same (request, kind, digest).
	Put(ctx context.Context, r ReviewContextRecord) (ReviewContextRecord, error)
	Get(ctx context.Context, requestID string, kind ReviewContextKind, digest string, now time.Time) (ReviewContextRecord, bool, error)
	// Latest returns the newest unexpired record of the kind for the Request.
	Latest(ctx context.Context, requestID string, kind ReviewContextKind, now time.Time) (ReviewContextRecord, bool, error)
}

// OutcomeReviewDecisionRepository keeps what people decided about the items of a review. Every call is scoped to the tenant of the
// context. A decision is bound to the review and the item key, which stays the same when the review is regenerated.
type OutcomeReviewDecisionRepository interface {
	// Record makes d the decision in force for its (review, item) and keeps the one it replaces as history.
	Record(ctx context.Context, d domain.ItemDecision) (domain.ItemDecision, error)
	// InForce returns the decisions in force for the review.
	InForce(ctx context.Context, reviewID string) ([]domain.ItemDecision, error)
	// History returns every decision about the item, newest first.
	History(ctx context.Context, reviewID, itemKey string) ([]domain.ItemDecision, error)
	// LinkChild sets the Request a send_back decision was handed to, for the decisions in force with the keys; it returns how many it changed.
	LinkChild(ctx context.Context, reviewID string, itemKeys []string, childRequestID string) (int, error)
}

// StoredDrawingProfile is a profile layer as it is kept: the project it belongs to (empty for the tenant) and its JSON.
type StoredDrawingProfile struct {
	ID         string
	TenantID   string
	ProjectKey string
	Spec       []byte
	SpecDigest string
	Version    int64
	UpdatedBy  string
	UpdatedAt  time.Time
}

// ReviewDrawingProfileRepository keeps the drawing profiles of a tenant and its projects.
type ReviewDrawingProfileRepository interface {
	Get(ctx context.Context, projectKey string) (StoredDrawingProfile, bool, error)
	// Save inserts the profile when expectedVersion is 0 and replaces it otherwise; a version that is not the stored one is
	// domain.ErrReviewStaleRevision.
	Save(ctx context.Context, p StoredDrawingProfile, expectedVersion int64) (StoredDrawingProfile, error)
	// Delete removes the layer under the same version check.
	Delete(ctx context.Context, projectKey string, expectedVersion int64) error
}
