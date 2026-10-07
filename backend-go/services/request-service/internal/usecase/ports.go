package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var (
	ErrPendingExists       = errors.New("pending approval already exists for this subject")
	ErrIdempotencyConflict = errors.New("approval idempotency key conflict")
	ErrApprovalNotFound    = errors.New("approval not found")
	ErrForbidden           = errors.New("forbidden")
)

type ApprovalListFilter struct {
	RequestID   string
	SubjectType domain.SubjectType
	Status      domain.ApprovalStatus
	PageSize    int
	PageToken   string
}

type Cursor struct {
	UpdatedAt time.Time
	ID        string
}

type BacklogRequestFilter struct {
	ProjectID          string
	Types              []string
	Categories         []string
	Cursor             *Cursor
	Limit              int
}

type BacklogRequestReader interface {
	ListReturnedRequests(ctx context.Context, tenantID string, f BacklogRequestFilter) ([]domain.Request, error)
	ParentRequestIDs(ctx context.Context, tenantID string, childIDs []string) (map[string][]string, error)
	LatestReturns(ctx context.Context, tenantID string, requestIDs []string) (map[string]domain.ReturnEvent, error)
}

type ApprovalGateReader interface {
	ListGateApprovals(ctx context.Context, tenantID string, requestIDs []string) ([]domain.Approval, error)
}

type ApprovalPolicyRepository interface {
	ListEnabledCandidates(ctx context.Context, tenantID string, subjectType domain.SubjectType, projectID, requestType, size, urgency string) ([]domain.ApprovalPolicy, error)
	Upsert(ctx context.Context, p domain.ApprovalPolicy) error
	Delete(ctx context.Context, tenantID, id string) error
	NowDB(ctx context.Context) (time.Time, error)
}

type ApprovalApproverRepository interface {
	InsertSnapshot(ctx context.Context, approvalID, tenantID string, approvers []domain.Principal) error
	ListForApproval(ctx context.Context, tenantID, approvalID string) ([]domain.Principal, error)
}

type ApprovalClaim struct {
	TenantID   string
	ApprovalID string
	RequestID  string
}

type ApprovalRepository interface {
	Insert(ctx context.Context, a domain.Approval) error
	GetForUpdate(ctx context.Context, tenantID, id string) (domain.Approval, error)
	FindPendingBySubject(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error)
	UpdateDecision(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error)
	UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error)
	CancelPendingForRequest(ctx context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error)
	List(ctx context.Context, tenantID string, f ApprovalListFilter) ([]domain.Approval, string, error)
	ClaimDue(ctx context.Context, batch int) ([]ApprovalClaim, error)
	ClaimDueForReminder(ctx context.Context, batch int) ([]ApprovalClaim, error)
	NowDB(ctx context.Context) (time.Time, error)
}

type TxRunner interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}

type OutboxWriter interface {
	InsertOutboxEvent(ctx context.Context, ev domain.OutboxEvent) error
}

func NewOutboxEvent(ctx context.Context, subject string, payload any) (domain.OutboxEvent, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.OutboxEvent{}, err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return domain.OutboxEvent{}, fmt.Errorf("marshal outbox payload: %w", err)
	}
	return domain.OutboxEvent{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		Subject:    subject,
		OccurredAt: time.Now().UTC(),
		Version:    1,
		Payload:    b,
	}, nil
}

type ContextSourceRepository interface {
    List(ctx context.Context) ([]domain.ContextSource, error)
    Get(ctx context.Context, key string) (domain.ContextSource, error)
    Upsert(ctx context.Context, s domain.ContextSource, expectedVersion int64) (domain.ContextSource, error)
    SetStatus(ctx context.Context, key, status string, expectedVersion int64) (domain.ContextSource, error)
}

type ContextPackRepository interface {
    Insert(ctx context.Context, p domain.ContextPack) error
    FindByInputDigest(ctx context.Context, requestID string, stage domain.Stage, digest string, notBefore time.Time) (domain.ContextPack, bool, error)
    Latest(ctx context.Context, requestID string, stage domain.Stage) (domain.ContextPack, bool, error)
}

type EvidenceRepository interface {
    NextSeq(ctx context.Context, requestID string) (int, error)
    InsertBatch(ctx context.Context, items []domain.Evidence) error
    Get(ctx context.Context, id string) (domain.Evidence, error)
    GetBySeq(ctx context.Context, requestID string, seq int) (domain.Evidence, error)
    MarkUsedBy(ctx context.Context, id string, use domain.EvidenceUse) error
}

type ListFilter struct {
	ProjectID      string
	ReporterID     string
	SourceProvider string
	SourceSite     string
	SourceRef      string
	Statuses       []domain.RequestStatus
	Types          []domain.RequestType
	PageSize       int
	PageToken      string
}

func (f ListFilter) Normalize() (ListFilter, error) {
	if f.PageSize < 0 {
		return f, apperrors.New(apperrors.KindInvalidArgument, "INVALID_PAGE_SIZE", "page size cannot be negative", nil)
	}
	if f.PageSize == 0 {
		f.PageSize = 50
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
	return f, nil
}

type ListResult struct {
	Requests      []domain.Request
	NextPageToken string
}

// RequestRepository manages domain.Request persistence.
// All methods extract tenant from context via tenant.RequireTenantID.
type RequestRepository interface {
	Create(ctx context.Context, r domain.Request) error
	Get(ctx context.Context, id string) (domain.Request, error)
	GetByNumber(ctx context.Context, number int64) (domain.Request, error)
	List(ctx context.Context, f ListFilter) (ListResult, error)
	Update(ctx context.Context, r domain.Request, expectedVersion int64) (domain.Request, error)
	UpdateSolutionEngine(ctx context.Context, id string, name domain.EngineName, expectedVersion int64) error
	NextNumber(ctx context.Context) (int64, error)
}

type RequestTypeHistoryRepository interface {
	Insert(ctx context.Context, h domain.RequestTypeChange) error
}

type SolutionListFilter struct {
	RequestID string
	Kind      domain.SolutionKind
	Status    domain.SolutionStatus
	Limit     int
}

type SolutionRepository interface {
	Insert(ctx context.Context, s domain.Solution) error
	Get(ctx context.Context, id string) (domain.Solution, error)
	GetByRequest(ctx context.Context, requestID string) (domain.Solution, error)
	Update(ctx context.Context, s domain.Solution, expectedVersion int64) (domain.Solution, error)
	ListByRequest(ctx context.Context, tenantID string, filter SolutionListFilter) ([]domain.Solution, error)
	UpdateOptions(ctx context.Context, tenantID, id string, options []byte, status domain.SolutionStatus) error
	Choose(ctx context.Context, tenantID, id string, idx int, expectedVersion int64) (bool, error)
	SupersedeOpen(ctx context.Context, tenantID, requestID string, kind domain.SolutionKind, exceptID string) error
	DeleteDraft(ctx context.Context, tenantID, id string) error
}

type RequestLinkRepository interface {
	Insert(ctx context.Context, link domain.RequestLink) error
	ListParents(ctx context.Context, childID string) ([]domain.RequestLink, error)
	ListChildren(ctx context.Context, parentID string) ([]domain.RequestLink, error)
	Delete(ctx context.Context, parentID, childID string) error
}

type RequestIdempotencyRepository interface {
	Claim(ctx context.Context, sourceProvider, sourceSite, sourceRef, requestID string) (existingRequestID string, claimed bool, err error)
	Find(ctx context.Context, sourceProvider, sourceSite, sourceRef string) (requestID string, err error)
}

type AnalysisRunRepository interface {
	InsertRunWithSolution(ctx context.Context, run domain.AnalysisRun, sol domain.Solution) (*domain.AnalysisRun, error)
	RenewLease(ctx context.Context, runID, owner string, ttl time.Duration) (bool, error)
	Complete(ctx context.Context, run domain.AnalysisRun) error
	Fail(ctx context.Context, run domain.AnalysisRun) error
	ClaimExpired(ctx context.Context, owner string, batch int) ([]domain.AnalysisRun, error)
	ListRecent(ctx context.Context, tenantID, requestID string, limit int) ([]domain.AnalysisRun, error)
	CountRunning(ctx context.Context, tenantID, projectID string, mode domain.AnalysisMode) (int, error)
}

type EngineSettingsRepository interface {
	Get(ctx context.Context, projectID string) (domain.ProjectEngineSettings, bool, error)
	Upsert(ctx context.Context, s domain.ProjectEngineSettings, expectedVersion int64) (domain.ProjectEngineSettings, error)
}

type OpenSpecChangeRepository interface {
	GetByRequest(ctx context.Context, requestID string) (domain.OpenSpecChange, bool, error)
	Upsert(ctx context.Context, c domain.OpenSpecChange) (domain.OpenSpecChange, error)
	UpdateSync(ctx context.Context, id string, state domain.SyncState, digest string, at *time.Time, expectedVersion int64) error
	ListPendingSync(ctx context.Context, limit int) ([]domain.OpenSpecChange, error)
}
