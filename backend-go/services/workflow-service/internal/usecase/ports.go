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
	// ListTemplates keyset-paginates tenantID's templates — optionally
	// filtered by scope (empty = all scopes), full-text query (against
	// name/description, idx_templates_fts) and tags (AND-filter: every
	// listed tag must be present, GIN-indexed) — and sorted by sort
	// ("" = default id order, matching annotation-service's ListAnnotations
	// convention; "trending" = usage_count DESC, rating_sum DESC;
	// "recent" = updated_at DESC). page_token is opaque regardless of
	// sort, but its ENCODING differs per sort (a bare last-seen id doesn't
	// carry enough information to resume a non-id ORDER BY) — see
	// adapter/postgres's encodeListCursor/decodeListCursor doc comment for
	// why and how; usecase/grpc layers never decode it themselves.
	ListTemplates(ctx context.Context, tenantID, scope, query string, tags []string, sort, pageToken string, pageSize int32) ([]domain.WorkflowTemplate, string, error)
	// ResolveChain returns templateID's parent_template_id chain,
	// root-first (index 0 = topmost ancestor, last = templateID itself),
	// depth-capped at maxDepth (workflow-service.md §6: 5) — see
	// usecase.ResolveTemplate's doc comment for the resolution policy this
	// feeds. Returns domain.ErrTemplateNotFound (wrapped) if templateID
	// itself doesn't exist for tenantID.
	ResolveChain(ctx context.Context, tenantID, templateID string, maxDepth int) ([]domain.WorkflowTemplate, error)
	// Update performs the conditional UPDATE, gated by expectedVersion —
	// still an optimistic-concurrency check on every write. templates.version
	// itself is only incremented when bumpVersion is true (SOL-030's
	// breaking-change + active-usage gate — see
	// usecase.UpdateTemplate.Execute's isBreakingChange call), not on every
	// write unconditionally. Returns domain.ErrTemplateVersionConflict
	// (wrapped) when expectedVersion doesn't match the current row's version.
	Update(ctx context.Context, tmpl domain.WorkflowTemplate, expectedVersion int32, bumpVersion bool) (domain.WorkflowTemplate, error)
	// WithTx runs fn inside a single Postgres transaction, committing on
	// nil and rolling back on any error fn returns — the transaction
	// boundary BUG-WF-03's visibility/approval work needs (a template's
	// Visibility transition and its Approval row, or its Visibility
	// transition and an execution's usage-count increment, must commit
	// together; see usecase.PublishTemplate/ResolveApproval and
	// TemplateRepositoryTx's doc comment). First multi-statement-atomic
	// requirement in this service — every existing usecase before
	// TASK-WF-03-04 is single-statement.
	WithTx(ctx context.Context, fn func(tx TemplateRepositoryTx) error) error
	// GetByShareToken looks up the (at most one, per the DB's unique
	// partial index) template whose share_token matches — backs
	// PreviewSharedTemplate/ImportSharedTemplate. Returns
	// domain.ErrTemplateNotFound (wrapped) if no template carries this
	// token (also covers "the empty token", which no row can match since
	// share_token is NULL until a template is published — see
	// migrations/0008's partial unique index).
	GetByShareToken(ctx context.Context, shareToken string) (domain.WorkflowTemplate, error)
	// SetShareToken mints (or overwrites) templateID's share_token —
	// backs usecase.GenerateShareLink.
	SetShareToken(ctx context.Context, templateID, token string) error
}

// TemplateRepositoryTx is the tx-scoped subset of template writes
// available inside TemplateRepository.WithTx's fn — every method here
// participates in the SAME transaction that fn was called with, so a
// caller composing several of these (e.g. UpdateVisibility + a sibling
// ApprovalRepositoryTx.CreateTx, via ApprovalRepositoryTx.Templates())
// gets a single atomic commit/rollback across all of them.
type TemplateRepositoryTx interface {
	// UpdateVisibility persists tmpl's Visibility (and returns the
	// updated row) — the direct-apply path (no approval gate): owner/admin
	// escalating outside the company tier, an admin escalating to
	// company, or any unpublish.
	UpdateVisibility(ctx context.Context, tmpl domain.WorkflowTemplate) (domain.WorkflowTemplate, error)
	// SetVisibility is UpdateVisibility's narrower sibling — sets ONLY
	// templateID's visibility column, used when the caller already has
	// nothing else about the row to change (e.g.
	// ResolveApproval.Execute's VisibilityCompany apply on approve, which
	// only ever touches this one column, not a full template write).
	SetVisibility(ctx context.Context, templateID string, v domain.Visibility) error
	// CreateExecution mirrors ExecutionRepository.CreateExecution but
	// inside THIS transaction — for an atomic
	// create-execution-and-increment-usage-count write (see
	// IncrementUsageCount's doc comment); not used by
	// PublishTemplate/ResolveApproval themselves.
	CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error
	// IncrementUsageCount bumps templateID's usage_count by 1 — the
	// signal TASK-WF-01-06's version-bump-on-breaking-change gate reads
	// (UsageCount > 0). Paired with CreateExecution in one transaction so
	// "this template was actually executed" and "an execution row exists"
	// can never disagree.
	IncrementUsageCount(ctx context.Context, templateID string) error
	// UpsertRating records userID's stars rating for templateID — one
	// rating per (user, template), enforced by workflow.ratings'
	// (template_id, user_id) UNIQUE constraint (a second call from the
	// SAME user UPDATES their prior rating, never duplicates it) — and
	// atomically recomputes templates.rating_sum/rating_count as a delta
	// against whatever the prior value was (0 for a first-time rating,
	// the old star count for an update), in the SAME transaction as the
	// ratings-table write, per TASK-WF-03-07.
	UpsertRating(ctx context.Context, templateID, userID string, stars int32) (RateTemplateResult, error)
}

// RateTemplateResult is UpsertRating's result — the template's aggregate
// rating AFTER this call's write, for RateTemplate's response.
type RateTemplateResult struct {
	RatingSum   int32
	RatingCount int32
}

// ApprovalRepositoryTx is the tx-scoped subset of approval writes
// available inside ApprovalRepository.WithTx's fn.
type ApprovalRepositoryTx interface {
	// Get returns domain.ErrApprovalNotFound (wrapped) if no matching row
	// exists for approvalID.
	Get(ctx context.Context, approvalID string) (domain.Approval, error)
	// Update persists approval's mutable fields (status, resolved_by,
	// resolved_at) — called after Approve/Reject.
	Update(ctx context.Context, approval domain.Approval) error
	// Templates returns a TemplateRepositoryTx scoped to the SAME
	// transaction as this ApprovalRepositoryTx — the mechanism
	// ResolveApproval.Execute uses to apply VisibilityCompany atomically
	// with the approval's status flip (see that usecase's doc comment).
	Templates() TemplateRepositoryTx
	// CreateTx inserts a new pending approval row — the tx-scoped sibling
	// of a plain Create, named distinctly since this interface has no
	// non-tx counterpart to collide with.
	CreateTx(ctx context.Context, approval domain.Approval) error
}

// ApprovalRepository is the persistence port for template publish-approval
// gate rows (workflow.approvals) — see domain.Approval's doc comment.
type ApprovalRepository interface {
	// WithTx runs fn inside a single Postgres transaction — see
	// TemplateRepository.WithTx's doc comment for the same contract.
	WithTx(ctx context.Context, fn func(tx ApprovalRepositoryTx) error) error
	// ListPending keyset-paginates tenantID's pending approvals — same
	// page_token/next-token convention as ListTemplates (opaque token =
	// last-seen id, ORDER BY id).
	ListPending(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Approval, string, error)
}

// OPAChecker mirrors orchestration-service's in-process OPA policy check
// for ResolveDecisionGate — "requester is a lead"/"approver is an admin"
// are auth-service/tenant-service facts, not something workflow-service
// determines from its own tables, so this port exists to keep that
// dependency explicit and swappable (a real OPA/auth-service-backed
// implementation vs. a test fake) rather than workflow-service reaching
// into auth-service's tables directly.
type OPAChecker interface {
	IsAdmin(ctx context.Context, userID string) bool
}

// ExecutionRepository is the persistence port for workflow executions.
type ExecutionRepository interface {
	CreateExecution(ctx context.Context, exec domain.WorkflowExecution) error
	// GetExecution returns domain.ErrExecutionNotFound (wrapped) if no
	// matching row exists for tenantID/id.
	GetExecution(ctx context.Context, tenantID, id string) (domain.WorkflowExecution, error)
	// UpdateExecution persists an execution's mutable fields (status,
	// paused_at) and, when event is non-nil, an outbox row — in the SAME
	// transaction, matching task-service.TaskRepository.Update's identical
	// extension (TASK-PW-04-02/03/05). Called after Pause/Resume
	// transitions and terminal-status writes; event is nil except at the
	// terminal-status call site (Execute.runToCompletion / recover
	// completion).
	UpdateExecution(ctx context.Context, exec domain.WorkflowExecution, event *domain.OutboxEvent) error
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

// ServerResolver turns a step's Target string into a connectionId ready
// for infra-fleet-service.Relay — see domain.AgentStepConfig.Target's doc
// comment for the four accepted shapes. An empty connectionId result
// means "execute locally," unchanged from today.
type ServerResolver interface {
	Resolve(ctx context.Context, tenantID, target string) (connectionID string, err error)
}

// ProviderResolver resolves which ai-provider-service account an agent
// step should use — workflow-service.md §7's priority note.
type ProviderResolver interface {
	Resolve(ctx context.Context, tenantID, userID, projectID string, pin *domain.ProviderPin) (accountID string, err error)
}

// EventPublisher fans a step/execution lifecycle event out to live
// StreamExecutionEvents subscribers.
type EventPublisher interface {
	Publish(ctx context.Context, event domain.ExecutionEvent) error
}

// ErrStepExecutorNotRegistered is returned by StepExecutorRegistry.Resolve
// when no StepExecutor is wired for the requested StepType — a composition
// root bug (cmd/server/main.go didn't register all five types), not a
// normal runtime condition.
var ErrStepExecutorNotRegistered = errors.New("usecase: no step executor registered for this step type")

// ErrNoActionHandlerRegistered is returned by an Action step
// (stepexecutors.ActionExecutor) when domain.ActionStepConfig.ActionName
// names no registered handler — TASK-WF-02-07 wires the `action` StepType
// itself but registers no concrete handlers, so this is the expected,
// typed outcome for every ActionName today (a clear error, not a silent
// no-op or a panic), until a future pass registers real handlers.
var ErrNoActionHandlerRegistered = errors.New("usecase: no action handler registered for this action name")

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
