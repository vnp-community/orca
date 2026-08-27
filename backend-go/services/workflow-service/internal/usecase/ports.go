// Package usecase holds workflow-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// TemplateRepository is the persistence port for workflow templates.
// Implemented by internal/adapter/postgres against workflow-service's own
// database — see architecture/05-data-architecture.md's
// database-per-service rule.
type TemplateRepository interface {
	CreateTemplate(ctx context.Context, tmpl domain.WorkflowTemplate) error
	// GetTemplate returns domain.ErrTemplateNotFound (wrapped) if no
	// matching row exists for tenantID/id.
	GetTemplate(ctx context.Context, tenantID, id string) (domain.WorkflowTemplate, error)
	// ListTemplates keyset-paginates tenantID's templates, optionally
	// filtered by scope (empty = all scopes) — same page_token/next-token
	// convention as annotation-service's ListAnnotations (opaque token =
	// last-seen id, ORDER BY id).
	ListTemplates(ctx context.Context, tenantID, scope, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error)
	// ResolveChain returns templateID's parent_template_id chain,
	// root-first (index 0 = topmost ancestor, last = templateID itself),
	// depth-capped at maxDepth (workflow-service.md §6: 5) — see
	// usecase.ResolveTemplate's doc comment for the resolution policy this
	// feeds. Returns domain.ErrTemplateNotFound (wrapped) if templateID
	// itself doesn't exist for tenantID.
	ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error)
	// Update performs the version-bump-on-write conditional UPDATE.
	// Returns domain.ErrTemplateVersionConflict (wrapped) when
	// expectedVersion doesn't match the current row's version.
	Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32) (domain.WorkflowTemplate, error)
}

// ExecutionRepository is the persistence port for workflow executions.
type ExecutionRepository interface {
	CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error
	// GetExecution returns domain.ErrExecutionNotFound (wrapped) if no
	// matching row exists for tenantID/id.
	GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error)
	// UpdateExecution persists an execution's mutable fields (status,
	// paused_at) — called after Pause/Resume transitions.
	UpdateExecution(ctx context.Context, exec domain.WorkflowExecution) error
	// HasActiveExecutions reports whether tenantID/projectID has any
	// execution in a non-terminal status — see usecase.HasActiveExecutions.
	HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error)
	// ListRunning returns every execution across every tenant currently in
	// status=running (NOT paused — see usecase.RecoverExecutions' doc
	// comment for why paused rows are deliberately left alone). Backs
	// RecoverExecutions' boot-time recovery scan (workflow-service.md §8).
	//
	// This is the one method on this port — and the one usecase in this
	// codebase — that is NOT tenant-scoped: every other usecase pulls a
	// single tenant id from the inbound request's context via
	// tenant.RequireTenantID and every other repository method takes that
	// tenantID explicitly. A boot-time recovery scan has no inbound
	// request and no single tenant — it runs once per process, and must
	// re-attach to every tenant's in-flight executions this instance's
	// database holds, not just one.
	ListRunning(ctx context.Context) ([]domain.WorkflowExecution, error)
}

// StepExecutionRepository is the persistence port for individual step runs
// within a WorkflowExecution's wave-dispatch — see domain.StepExecution.
// Implemented by internal/adapter/postgres against the same database as
// TemplateRepository/ExecutionRepository (workflow.step_executions, RLS'd
// via its execution_id join per architecture/05-data-architecture.md).
type StepExecutionRepository interface {
	CreateStepExecution(ctx context.Context, se domain.StepExecution) error
	// UpdateStepExecution persists a step execution's mutable fields
	// (status, output, error) — called as a step transitions
	// pending->running->completed/failed.
	UpdateStepExecution(ctx context.Context, se domain.StepExecution) error
	// ListStepExecutions returns every step execution row for
	// tenantID/executionID, ordered by wave then id — used by integration
	// tests and any future observability surface over a run's step-level
	// history.
	ListStepExecutions(ctx context.Context, tenantID, executionID string) ([]domain.StepExecution, error)
}

// ErrStepExecutorNotRegistered is returned by StepExecutorRegistry.Resolve
// when no StepExecutor is wired for the requested StepType — a composition
// root bug (cmd/server/main.go didn't register all five types), not a
// normal runtime condition.
var ErrStepExecutorNotRegistered = errors.New("usecase: no step executor registered for this step type")

// StepExecutorRegistry resolves a StepType to the concrete StepExecutor
// that runs it. Implemented by internal/adapter/stepexecutors and wired in
// cmd/server/main.go with all five step types (Condition/Webhook real,
// Agent/Shell/Notification stubbed pending infra-fleet-service — see
// workflow-service.md §4 and this service's README).
type StepExecutorRegistry interface {
	Resolve(stepType domain.StepType) (domain.StepExecutor, error)
}

// WorktreeInfo is the minimal worktree shape
// CleanupWorktreesStepExecutor needs from project-service's ListWorktrees —
// see ProjectClient.
type WorktreeInfo struct {
	ID     string
	Branch string
}

// ProjectClient wraps project-service's filtered ListWorktrees RPC — a new
// outbound dependency edge (BL-AT-04's cleanup_worktrees step candidate
// query, TASK-AT-04-02).
type ProjectClient interface {
	ListWorktrees(ctx context.Context, projectID string, statusIn []string, olderThan time.Time) ([]WorktreeInfo, error)
}

// GitGatewayClient wraps git-gateway-service's RemoveWorktree RPC — the ONE
// call site CleanupWorktreesStepExecutor uses for the actual delete
// (TASK-AT-04-03's BR-AT-11/BR-AT-12 safety checks live server-side, in
// git-gateway-service, so every caller — manual worktree.rm AND this
// automated bulk path — gets them for free).
type GitGatewayClient interface {
	RemoveWorktree(ctx context.Context, worktreeID string, force, allowOpenPR bool) error
}

// ErrWorktreeRemovalBlocked is returned (wrapped) by GitGatewayClient.
// RemoveWorktree when git-gateway-service rejected the removal on a safety
// check (BR-AT-11 uncommitted changes / BR-AT-12 open PR) — the adapter
// translates the gRPC FailedPrecondition status into this transport-agnostic
// sentinel so CleanupWorktreesStepExecutor can distinguish "expected skip"
// from a genuine removal failure without importing grpc codes into usecase/.
var ErrWorktreeRemovalBlocked = errors.New("usecase: worktree removal blocked by a safety check")

// CleanupEntry is one worktree's outcome in a cleanup run — mirrors
// automation-service's domain.CleanupLogEntry (kept as a separate type here
// per architecture/03's rule that domain/ packages don't share types
// across service boundaries).
type CleanupEntry struct {
	WorktreeID string
	Action     string // "deleted" | "skipped" | "would_delete"
	Reason     string
}

// CleanupAuditWriter wraps automation-service's WriteCleanupReport RPC —
// BR-AT-14's per-worktree audit trail (TASK-AT-04-04). runID identifies the
// automation_runs row this cleanup dispatch belongs to; a caller with no
// such ID (e.g. a manually-triggered cleanup outside RunNow's action loop)
// should pass "" — WriteCleanupReport's own FK requires a real row, so the
// implementation treats a failed/empty-runID write as best-effort, never
// failing the step itself.
type CleanupAuditWriter interface {
	WriteCleanupReport(ctx context.Context, runID string, entries []CleanupEntry) error
}

// ProfileResolver is the outbound port toward tenant-service.GetResolvedProfile
// — a NEW dependency edge (tenant-service.md §7 already documents this as
// intended for task-service/workflow-service; workflow-service never
// exercised it before this task). Returns the already-JSON-decoded
// resolved_settings_json as a generic map, matching the shape
// domain.BuildAgentEnv reads.
type ProfileResolver interface {
	GetResolvedProfile(ctx context.Context, userID string) (map[string]any, error)
}

// ProjectContextResolver is the outbound port toward
// project-service.GetProjectContext (TASK-PRF-04-01/02).
type ProjectContextResolver interface {
	GetProjectContext(ctx context.Context, projectID string) (ProjectContext, error)
}

// ProjectContext is the subset of project-service's ProjectContext this
// service's agent-spawn preamble needs — decoded from the gRPC response by
// internal/adapter/infrafleetclient's ProjectContextResolver implementation.
type ProjectContext struct {
	ProjectID, ProjectName, Description     string
	RepoURL, DevServerID, DevServerHostname string
}
