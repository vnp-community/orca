package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestContentWriter is kept apart from RequestRepository so fakes of the latter keep compiling.
// It is the only path that writes the content columns after creation.
type RequestContentWriter interface {
	// UpdateContent writes title, body and the content columns of r, bumps version and content_revision,
	// and fails with REQUEST_VERSION_CONFLICT unless the row is still at expectedVersion and r.ContentRevision-1.
	UpdateContent(ctx context.Context, r domain.Request, expectedVersion int64) (domain.Request, error)
}

// RequestRevisionRepository is append-only. A duplicate (request, revision) is REQUEST_VERSION_CONFLICT.
type RequestRevisionRepository interface {
	Append(ctx context.Context, rev domain.RequestRevision) error
	Get(ctx context.Context, requestID string, revision int) (domain.RequestRevision, error)
	// List returns revisions with revision > afterRevision in ascending order, at most limit.
	List(ctx context.Context, requestID string, afterRevision, limit int) ([]domain.RequestRevision, error)
}

type ArtifactIndexRepository interface {
	// Insert is idempotent: an existing (tenant, display_id) is left alone and reported as inserted=false.
	Insert(ctx context.Context, e domain.IndexEntry) (inserted bool, err error)
	Resolve(ctx context.Context, displayID string) (domain.IndexEntry, error)
	// NextSolutionSeq only reads MAX(seq)+1; a clash on the unique key is retried by the caller.
	NextSolutionSeq(ctx context.Context, requestID string) (int, error)
	NextPlanSeq(ctx context.Context, requestID string) (int, error)
	// SetSolutionSeq stores the minted seq on a solution that has none yet.
	SetSolutionSeq(ctx context.Context, solutionID string, seq int) error
}

type ArtifactRelationRepository interface {
	// Insert is idempotent on the unique edge.
	Insert(ctx context.Context, r domain.ArtifactRelation) (inserted bool, err error)
	ListByRequest(ctx context.Context, requestID string) ([]domain.ArtifactRelation, error)
}

type RequestCoverageRepository interface {
	// ReplaceForPlan joins the caller's transaction: delete the plan's rows, insert the new ones.
	ReplaceForPlan(ctx context.Context, requestID, planTaskID string, rows []domain.CoverageRow) error
	ListByRequest(ctx context.Context, requestID string) ([]domain.CoverageRow, error)
}

// AnswerRecord is one draft or final answer stored on a question.
type AnswerRecord struct {
	QuestionID string
	Value      json.RawMessage
	Source     string
	By         string
	At         time.Time
}

// ClarificationRef points at a clarification across tenants for the sweeps.
type ClarificationRef struct {
	TenantID string
	ID       string
}

type PendingFilter struct {
	UserID    string
	Teams     []string
	Roles     []string
	IsAdmin   bool
	PageSize  int
	PageToken string
}

type ClarificationListFilter struct {
	RequestID string
	Status    domain.ClarificationStatus // empty means any
	AfterSeq  int
	Limit     int
}

type ClarificationRepository interface {
	// Insert stores the clarification with its questions and assignees. A second open one for the same
	// request is REQUEST_CLARIFICATION_STATE_NOT_ALLOWED; a seq clash is REQUEST_VERSION_CONFLICT.
	Insert(ctx context.Context, c domain.Clarification) error
	Get(ctx context.Context, id string) (domain.Clarification, error)
	GetOpenByRequest(ctx context.Context, requestID string) (*domain.Clarification, error)
	List(ctx context.Context, f ClarificationListFilter) ([]domain.Clarification, error)
	NextSeq(ctx context.Context, requestID string) (int, error)
	// MaxRound is the highest round among earlier clarifications of the same chain (source and source_ref), 0 when none.
	MaxRound(ctx context.Context, requestID string, source domain.ClarificationSource, sourceRef string) (int, error)
	UpdateAnswers(ctx context.Context, id string, answers []AnswerRecord, expectedVersion int64) error
	MarkAnswered(ctx context.Context, id string, answeredRevision int, at time.Time, expectedVersion int64) error
	// MarkCancelled with expectedVersion 0 skips the version check (cancel hooks do not know it).
	MarkCancelled(ctx context.Context, id, reason string, at time.Time, expectedVersion int64) error
	// ListDueRefs and ListRemindableRefs read across tenants and write nothing.
	ListDueRefs(ctx context.Context, now time.Time, batch int) ([]ClarificationRef, error)
	ListRemindableRefs(ctx context.Context, now time.Time, batch int) ([]ClarificationRef, error)
	// LockOpenDue locks one open, overdue clarification of the ctx tenant for this transaction; nil when it
	// is gone, not overdue yet, or held by another worker (SKIP LOCKED).
	LockOpenDue(ctx context.Context, id string, now time.Time) (*domain.Clarification, error)
	MarkExpired(ctx context.Context, id string, at time.Time) error
	// MarkReminded claims the single reminder: true for exactly one caller.
	MarkReminded(ctx context.Context, id string, at time.Time) (bool, error)
	ListPendingForUser(ctx context.Context, f PendingFilter) ([]domain.Clarification, string, error)
}

type DecisionListFilter struct {
	RequestID string
	Status    domain.DecisionStatus // empty means any
	AfterSeq  int
	Limit     int
}

type DecisionRepository interface {
	// Insert fails with REQUEST_DECISION_STATE_INVALID when a live decision already exists for the subject.
	Insert(ctx context.Context, d domain.Decision) error
	Get(ctx context.Context, id string) (domain.Decision, error)
	GetLiveBySubject(ctx context.Context, kind domain.DecisionSubjectKind, subjectID string) (*domain.Decision, error)
	Update(ctx context.Context, d domain.Decision, expectedVersion int64) (domain.Decision, error)
	AppendHistory(ctx context.Context, h domain.DecisionHistory) error
	ListHistory(ctx context.Context, decisionID string) ([]domain.DecisionHistory, error)
	List(ctx context.Context, f DecisionListFilter) ([]domain.Decision, error)
	NextSeq(ctx context.Context, requestID string) (int, error)
	// SupersedeLiveBySubject and SupersedeLiveByRequest return the decisions they superseded so the caller can write history.
	SupersedeLiveBySubject(ctx context.Context, kind domain.DecisionSubjectKind, subjectID string) ([]domain.Decision, error)
	SupersedeLiveByRequest(ctx context.Context, requestID string) ([]domain.Decision, error)
}

// SubtreeNode and PlanSubtree are what GetArtifactGraph needs from task-service's GetSubtree.
type SubtreeNode struct {
	ID       string
	ParentID string
	// Kind is plan, phase or task.
	Kind   string
	Labels []string
}

type PlanSubtree struct {
	Nodes     []SubtreeNode
	DependsOn []domain.DependsEdge
}

// PlanTreeReader reads a Plan's task tree. A nil reader (or an error) makes the graph partial rather than failing the call.
type PlanTreeReader interface {
	GetSubtree(ctx context.Context, planTaskID string) (PlanSubtree, error)
}
