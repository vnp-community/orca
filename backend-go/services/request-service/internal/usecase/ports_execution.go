package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ListTasksQuery selects tasks of one or more Requests; the client follows page tokens to the end.
type ListTasksQuery struct {
	ProjectID  string
	RequestIDs []string
	TaskTypes  []string
	ParentID   string
}

// TaskEdge says From depends on To.
type TaskEdge struct{ From, To string }

type SubtreeView struct {
	Tasks     []domain.TaskView
	DependsOn []TaskEdge
}

type ExecutionStateView struct {
	LastEngine       string
	LastLinkStatus   string
	FailedAttempts   int
	BlockedByTaskIDs []string
	LastError        string
}

// TaskClient is everything request-service asks of task-service. One interface for execution (SOL-013),
// plan writing (SOL-012) and the backlog views (SOL-015), so there is a single connection and a single fake.
// Failures that callers branch on come back as domain.ErrTaskAlreadyRunning, ErrTaskDispatchTransient,
// ErrTaskForbidden or *domain.DispatchError; anything else is an infrastructure error.
type TaskClient interface {
	ListTasks(ctx context.Context, q ListTasksQuery) ([]domain.TaskView, error)
	GetSubtree(ctx context.Context, rootID string) (SubtreeView, error)
	// Execute dispatches one working task; executionRequestID is "req:<request>:<task>:<attempt>".
	Execute(ctx context.Context, taskID, executionRequestID string) error
	SetWorktree(ctx context.Context, taskID, worktreeID string) error
	SetStatus(ctx context.Context, taskID, status string) error
	// ListExecutionStates batches its ids itself (at most 500 per call).
	ListExecutionStates(ctx context.Context, taskIDs []string) (map[string]ExecutionStateView, error)
}

// ExecutionSettings are the REQUEST_* knobs of the execution loop.
type ExecutionSettings struct {
	MaxParallelTasks    int
	MaxTaskAttempts     int
	AutoCompleteTasks   bool
	DispatchRetryWindow time.Duration
	// ReconcileQuiet is how long a Request must be silent before the reconcile loop looks at it again.
	ReconcileQuiet time.Duration
}

func (s ExecutionSettings) normalized() ExecutionSettings {
	if s.MaxParallelTasks <= 0 {
		s.MaxParallelTasks = 1
	}
	if s.MaxTaskAttempts <= 0 {
		s.MaxTaskAttempts = 2
	}
	if s.DispatchRetryWindow <= 0 {
		s.DispatchRetryWindow = 15 * time.Minute
	}
	if s.ReconcileQuiet <= 0 {
		s.ReconcileQuiet = 5 * time.Minute
	}
	return s
}

// DefaultExecutionSettings are the documented defaults of REQUEST_MAX_PARALLEL_TASKS and friends.
func DefaultExecutionSettings() ExecutionSettings {
	return ExecutionSettings{MaxParallelTasks: 1, MaxTaskAttempts: 2, AutoCompleteTasks: true, DispatchRetryWindow: 15 * time.Minute, ReconcileQuiet: 5 * time.Minute}
}

// PhaseStartRepository is the idempotency table of StartPhase. All methods are tenant-scoped through ctx.
type PhaseStartRepository interface {
	// TryStart inserts the row; inserted=false means the Phase was already started.
	TryStart(ctx context.Context, s domain.PhaseStart) (inserted bool, err error)
	ListByRequest(ctx context.Context, requestID string) ([]domain.PhaseStart, error)
}

// TaskRunOutcomeRepository keeps what happened to each task run. Tenant comes from ctx.
type TaskRunOutcomeRepository interface {
	// Insert is idempotent on event_id and, for phase_done/plan_done, on the container: inserted=false means a row already covers it.
	Insert(ctx context.Context, o domain.TaskRunOutcome) (inserted bool, err error)
	// CountFailed counts runs that used up an attempt (dispatch errors do not).
	CountFailed(ctx context.Context, taskID string) (int, error)
	// LatestFailed returns the newest failed outcome per task, dispatch errors included, for the "last error" column.
	LatestFailed(ctx context.Context, taskIDs []string) (map[string]domain.TaskRunOutcome, error)
	// LastEventAt is the newest occurred_at of the Request, zero when it has none.
	LastEventAt(ctx context.Context, requestID string) (time.Time, error)
	Exists(ctx context.Context, taskID string, outcome domain.Outcome) (bool, error)
	// DispatchRetrySince is when the current streak of dispatch errors began (since the last started run).
	DispatchRetrySince(ctx context.Context, taskID string) (since time.Time, ok bool, err error)
	// LatestDispatchError is the newest dispatch_error outcome of the task, for the backlog reason.
	LatestDispatchError(ctx context.Context, taskID string) (domain.TaskRunOutcome, bool, error)
}

// RequestCheckRepository is append-only: there is no update or delete path.
type RequestCheckRepository interface {
	// Append stores the row and returns it with the database's created_at.
	Append(ctx context.Context, c domain.RequestCheck) (domain.RequestCheck, error)
	// ListByRequest returns the rows oldest first.
	ListByRequest(ctx context.Context, requestID string) ([]domain.RequestCheck, error)
	Latest(ctx context.Context, requestID string, kind domain.CheckKind) (domain.RequestCheck, bool, error)
}

// ExecutingRef names a Request found by the cross-tenant scan.
type ExecutingRef struct {
	TenantID  string
	RequestID string
}

// ExecutingRequestScanner finds executing Requests with no activity for quietFor. It reads across tenants,
// so callers must switch ctx to the ref's tenant before doing anything else.
type ExecutingRequestScanner interface {
	ListQuietExecuting(ctx context.Context, quietFor time.Duration, limit int) ([]ExecutingRef, error)
}

// ReconcileLeases keeps two replicas from reconciling the same Request at once. Tenant comes from ctx.
type ReconcileLeases interface {
	// Claim succeeds when no live lease exists and the Request was not reconciled within quiet.
	Claim(ctx context.Context, requestID, owner string, lease, quiet time.Duration) (bool, error)
	// Release ends the lease and stamps last_run_at.
	Release(ctx context.Context, requestID, owner string) error
}

// ApprovalOpener is *OpenApproval; a port so AdvanceExecution can be tested without the approval stack.
type ApprovalOpener interface {
	Execute(ctx context.Context, in OpenApprovalInput) (*domain.Approval, error)
}

// FollowUpExecutor creates the child Requests a type policy asked for after completion.
type FollowUpExecutor interface {
	ExecuteFollowUps(ctx context.Context, req domain.Request, fus []domain.FollowUp) error
}

// ExecutionActors says whose identity task-service sees when request-service dispatches: the user who approved.
type ExecutionActors interface {
	// ActorFor returns the Phase's starter when phaseID is set, else the approver of the Plan (or reporter as a last resort).
	ActorFor(ctx context.Context, req domain.Request, phaseID string) (string, error)
}
