package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Repository-level aliases so adapters keep returning the usecase-package names.
var (
	ErrPendingExists       = domain.ErrApprovalPendingExists
	ErrIdempotencyConflict = domain.ErrApprovalIdempotency
	ErrApprovalNotFound    = domain.ErrApprovalNotFound
	ErrForbidden           = domain.ErrApprovalForbidden
)

type ApprovalListFilter struct {
	RequestID   string
	SubjectType domain.SubjectType
	Status      domain.ApprovalStatus
	PageSize    int
	PageToken   string
}

// PendingForUserFilter carries the caller's resolved principals: the repository joins them with the
// approver snapshots in one query instead of loading every pending approval.
type PendingForUserFilter struct {
	UserID      string
	Role        string
	TeamIDs     []string
	SubjectType domain.SubjectType
	PageSize    int
	PageToken   string
}

// PendingApproval is an Approval plus the Request columns the inbox shows (no N+1 GetRequest).
type PendingApproval struct {
	domain.Approval
	RequestTitle  string
	RequestType   string
	RequestNumber int64
}

type ApprovalClaim struct {
	TenantID   string
	ApprovalID string
	RequestID  string
}

// ApprovalRepository: every method except ClaimDue/ClaimDueForReminder is tenant-scoped through ctx.
type ApprovalRepository interface {
	Insert(ctx context.Context, a domain.Approval) error
	// Get reads without a row lock; use it to learn request_id before taking the Request lock.
	Get(ctx context.Context, tenantID, id string) (domain.Approval, error)
	GetForUpdate(ctx context.Context, tenantID, id string) (domain.Approval, error)
	FindPendingBySubject(ctx context.Context, tenantID string, st domain.SubjectType, subjectID string) (*domain.Approval, error)
	FindByIdempotencyKey(ctx context.Context, tenantID, key string) (*domain.Approval, error)
	UpdateDecision(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error)
	// UpdateDue writes due_at and clears reminded_at, bumping version.
	UpdateDue(ctx context.Context, a domain.Approval, expectedVersion int64) (bool, error)
	// MarkReminded sets reminded_at once; it does not bump version so reminders never fail a user's decision.
	MarkReminded(ctx context.Context, tenantID, id string, at time.Time) (bool, error)
	UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error)
	CancelPendingForRequest(ctx context.Context, tenantID, requestID, why string, now time.Time) ([]domain.Approval, error)
	List(ctx context.Context, tenantID string, f ApprovalListFilter) ([]domain.Approval, string, error)
	ListPendingForUser(ctx context.Context, tenantID string, f PendingForUserFilter) ([]PendingApproval, string, error)
	// ClaimDue and ClaimDueForReminder scan across tenants (sweeper); callers must switch to the claim's tenant before InTx.
	ClaimDue(ctx context.Context, batch int) ([]ApprovalClaim, error)
	ClaimDueForReminder(ctx context.Context, batch int) ([]ApprovalClaim, error)
	NowDB(ctx context.Context) (time.Time, error)
}

type ApprovalPolicyRepository interface {
	ListEnabledCandidates(ctx context.Context, tenantID string, subjectType domain.SubjectType, projectID, requestType, size, urgency string) ([]domain.ApprovalPolicy, error)
	Get(ctx context.Context, tenantID, id string) (domain.ApprovalPolicy, error)
	List(ctx context.Context, tenantID string, f PolicyListFilter) ([]domain.ApprovalPolicy, string, error)
	// Upsert creates the policy when expectedVersion is 0 and the id is new, otherwise updates with a version check.
	Upsert(ctx context.Context, p domain.ApprovalPolicy, expectedVersion int64) (domain.ApprovalPolicy, error)
	Delete(ctx context.Context, tenantID, id string, expectedVersion int64) error
	NowDB(ctx context.Context) (time.Time, error)
}

type PolicyListFilter struct {
	ProjectID   string
	SubjectType domain.SubjectType
	PageSize    int
	PageToken   string
}

type ApprovalApproverRepository interface {
	InsertSnapshot(ctx context.Context, approvalID, tenantID string, approvers []domain.Principal) error
	ListForApproval(ctx context.Context, tenantID, approvalID string) ([]domain.Principal, error)
}

// RequestLocker takes the Request row lock (lock order: Request, then Approval) and returns the locked row.
// It must run inside a transaction.
type RequestLocker interface {
	LockRequest(ctx context.Context, requestID string) (domain.Request, error)
}

type TeamMembershipResolver interface {
	TeamsForUser(ctx context.Context, userID string) ([]string, error)
	MembersOfTeam(ctx context.Context, teamID string) ([]string, error)
}

type AdminDirectoryResolver interface {
	ListAdmins(ctx context.Context, tenantID string) ([]string, error)
}

// RequestReader is the read-only part of RequestRepository that approval needs outside a transaction.
type RequestReader interface {
	Get(ctx context.Context, id string) (domain.Request, error)
}
