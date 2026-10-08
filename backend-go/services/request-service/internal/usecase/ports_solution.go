package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ErrProbeUnavailable means the repository state cannot be read (no worktree id); the run records repo_check=skipped.
var ErrProbeUnavailable = errors.New("repository state probe unavailable")

// ErrAgentReadonlyUnsupported means the dev server's agent refused accessMode=readonly (it would have run in write mode).
var ErrAgentReadonlyUnsupported = errors.New("agent does not support readonly mode")

type SolutionListFilter struct {
	RequestID string
	Kind      domain.SolutionKind
	Status    domain.SolutionStatus
	Limit     int
}

// SolutionStore extends the foundation SolutionCoreRepository with the queries the solution flow needs.
// All methods take the tenant from ctx and join the ctx transaction.
type SolutionStore interface {
	SolutionCoreRepository
	ListByRequest(ctx context.Context, f SolutionListFilter) ([]domain.Solution, error)
	// Choose is a compare-and-set on version; false means the solution moved on (not proposed, or edited).
	Choose(ctx context.Context, id string, idx int, expectedVersion int64) (bool, error)
	// SupersedeOpen retires proposed and rejected solutions of the same kind, keeping exceptID.
	SupersedeOpen(ctx context.Context, requestID string, kind domain.SolutionKind, exceptID string) (int, error)
	DeleteDraft(ctx context.Context, id string) error
}

type StartRunOptions struct {
	LeaseTTL time.Duration
	// MaxAgentRuns caps running agent_readonly runs per project; 0 disables the cap.
	MaxAgentRuns int
}

type StartRunResult struct {
	Run domain.AnalysisRun
	// Created is false when an existing run (same idempotency key, or already running for the request and kind) is returned instead.
	Created bool
}

// AnalysisRunStore keeps analysis runs with a DB-clock lease. Time is the database's, never the app's,
// so skewed instances cannot steal each other's runs.
type AnalysisRunStore interface {
	// EnsureProjectGate creates the per-project lock row StartRun serialises agent runs on. Call it outside any
	// transaction: concurrent creators inside transactions can deadlock on InnoDB gap locks.
	EnsureProjectGate(ctx context.Context, projectID string) error
	// StartRun inserts the run and its draft Solution in the ctx transaction, or returns the blocking run
	// without writing. A full project returns REQUEST_ANALYSIS_BUSY and writes nothing.
	StartRun(ctx context.Context, run domain.AnalysisRun, draft domain.Solution, opts StartRunOptions) (StartRunResult, error)
	Get(ctx context.Context, runID string) (domain.AnalysisRun, error)
	// RenewLease reports false when another owner holds the run or it already finished.
	RenewLease(ctx context.Context, runID, owner string, ttl time.Duration) (bool, error)
	// FinishOwned records the final state; false means the lease was lost and nothing was written.
	FinishOwned(ctx context.Context, run domain.AnalysisRun, owner string) (bool, error)
	// ClaimExpired takes runs whose lease lapsed, across tenants; FOR UPDATE SKIP LOCKED keeps sweepers apart.
	ClaimExpired(ctx context.Context, owner string, ttl time.Duration, batch int) ([]domain.AnalysisRun, error)
	ListRecent(ctx context.Context, requestID string, limit int) ([]domain.AnalysisRun, error)
	CountRunning(ctx context.Context, projectID string, mode domain.AnalysisMode) (int, error)
}

// AnalysisConnection says where AI work runs. RepoPath and WorktreeID are only known for a resolved infra connection.
type AnalysisConnection struct {
	ConnectionID string
	DevServerID  string
	RepoPath     string
	WorktreeID   string
}

type AnalysisConnectionResolver interface {
	// ResolveForProject returns ErrNoDevServer when nothing can run the call.
	ResolveForProject(ctx context.Context, projectID string) (AnalysisConnection, error)
}

// ProjectAICompleter runs ai.complete for a project on its dev server.
type ProjectAICompleter interface {
	Complete(ctx context.Context, projectID, prompt string) (string, error)
}

type ProjectContext struct {
	Name    string
	RepoURL string
}

// ProjectContextReader is best effort: callers continue with an empty context on error.
type ProjectContextReader interface {
	Read(ctx context.Context, projectID string) (ProjectContext, error)
}

// AgentPromptInput deliberately has no trust preset or free-form env: the adapter fixes both, so no caller can request full trust.
type AgentPromptInput struct {
	StepID    string
	Prompt    string
	RepoPath  string
	RequestID string
	ProjectID string
	TimeoutMS int
	// ReadOnlyEnforced asks the agent to enforce read-only (accessMode=readonly); only set after the capability check.
	ReadOnlyEnforced bool
}

type AgentPromptResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
	// Warnings come from the agent, e.g. READONLY_VIOLATION.
	Warnings []string
	// AppliedAccessMode echoes what the agent really applied; empty when it did not echo.
	AppliedAccessMode string
	ChangesAvailable  bool
	HeadMoved         bool
	ChangedFiles      int
}

type AgentPromptRunner interface {
	ExecPrompt(ctx context.Context, conn AnalysisConnection, in AgentPromptInput) (AgentPromptResult, error)
}

type RepoSnapshot struct {
	Branch string
	Files  []RepoFileState
}

type RepoFileState struct {
	Path  string
	State string
}

type RepoStateProbe interface {
	// Snapshot returns ErrProbeUnavailable when worktreeID is empty.
	Snapshot(ctx context.Context, worktreeID string) (RepoSnapshot, error)
}

// AnalysisSpawner starts the background worker for a run that was just committed.
type AnalysisSpawner interface {
	Spawn(run domain.AnalysisRun)
}

// SolutionActorAuthorizer decides who may steer analysis of a request (D6 is provisional: reporter or admin).
type SolutionActorAuthorizer interface {
	AuthorizeGenerate(ctx context.Context, req domain.Request) error
	AuthorizeChoose(ctx context.Context, req domain.Request) error
}

type OpenSolutionApprovalInput struct {
	Request     domain.Request
	SubjectType domain.SubjectType
	SubjectID   string
	// Digest binds the approval to the exact content; Approve must send it back.
	Digest string
}

// SolutionApprovalOpener opens the pending Approval inside the ctx transaction. The registry-backed implementation
// arrives with CR-REQ-009; until then wiring uses a logging no-op.
type SolutionApprovalOpener interface {
	Open(ctx context.Context, in OpenSolutionApprovalInput) error
}

// PendingDigestUpdater refreshes the digest of a pending Approval; ApprovalRepository satisfies it.
type PendingDigestUpdater interface {
	UpdatePendingDigest(ctx context.Context, tenantID string, st domain.SubjectType, subjectID, digest string) (bool, error)
}

// BacklogReturner sends a request back to the backlog; *ReturnRequestToBacklog satisfies it.
type BacklogReturner interface {
	Execute(ctx context.Context, in ReturnInput) (domain.Request, error)
}
