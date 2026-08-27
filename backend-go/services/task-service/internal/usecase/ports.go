// Package usecase holds task-service's application services and the ports
// they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// Clock abstracts time.Now so expiry logic (ResolveGrant's grant-expiry
// filter, SOL-TG-04's execute_task.go) is deterministically testable
// against fakes, per specs/backend-go/standards/testing-strategy.md —
// mirrors auth-service/internal/usecase/ports.go's identical Clock/
// SystemClock pattern.
type Clock interface {
	Now() time.Time
}

// SystemClock is the real Clock, wired in cmd/server/main.go.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// TaskRepository is the persistence port for tasks. Implemented by
// internal/adapter/postgres against task-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type TaskRepository interface {
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
	Get(ctx context.Context, tenantID, id string) (domain.Task, error)
	// GetAncestors returns the chain from the given task up to its root,
	// walking tasks.parent_id (task-service.md §6's WITH RECURSIVE note).
	// The first element is the task itself; the last is the root. maxDepth
	// bounds the walk (task-service.md §8's max-depth guard) — 0 means the
	// repository's own default cap.
	GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error)
	// UpdateStatus persists a task's status transition. Currently only ever
	// called to set StatusInProgress on dispatch (see ExecuteTask) — there is
	// no RPC surface yet to transition a task back out of in_progress. See
	// this service's README "Known gaps".
	UpdateStatus(ctx context.Context, tenantID, id, status string) error
	// HasActiveExecutions reports whether tenantID/projectID has any task
	// currently in_progress — see usecase.HasActiveExecutions's doc comment
	// for the one-way-transition caveat this answer is subject to today.
	HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error)
	// List returns tasks for tenantID, optionally filtered by projectID
	// (empty = no filter), cursor-paginated by page_size/page_token.
	List(ctx context.Context, tenantID, projectID, pageToken string, pageSize int32) ([]domain.Task, string, error)
	// Update persists a partial field update (title/status). Status
	// transitions into StatusInProgress are rejected at the domain layer
	// (domain.Task.SetStatus) before this is ever called — see TASK-223's
	// Context note.
	Update(ctx context.Context, tenantID string, task domain.Task) error
	// Delete removes a task. task_edges/task_grants reference tasks(id)
	// with ON DELETE CASCADE (migrations/0001_init.up.sql) — no explicit
	// edge/grant cleanup needed.
	Delete(ctx context.Context, tenantID, id string) error
	// UpdateWorktreeID persists the provisioned worktree a task's execution
	// is running in — see SOL-TG-04.
	UpdateWorktreeID(ctx context.Context, tenantID, id, worktreeID string) error
	// UpdateActiveExecutionID persists the complex path's coordinator_run
	// id (TASK-TG-04-04/05) — read back by ReportTaskExecutionResult to
	// reject a stale/duplicate completion callback.
	UpdateActiveExecutionID(ctx context.Context, tenantID, id, activeExecutionID string) error
	// UpdatePromptTemplate persists the "Generate Agent Prompt" output — see
	// SOL-TG-02.
	UpdatePromptTemplate(ctx context.Context, tenantID, id, promptTemplate string) error
	// UpdateAIPlanJSON persists the raw AIDecompose response for later
	// inspection/replay — see SOL-TG-02.
	UpdateAIPlanJSON(ctx context.Context, tenantID, id, aiPlanJSON string) error
	// GetSubtree walks tasks.parent_id DOWNWARD from rootID — the mirror
	// image of GetAncestors — returning every descendant task plus every
	// depends_on edge originating from one of those tasks. See SOL-TG-01.
	GetSubtree(ctx context.Context, tenantID, rootID string, maxDepth int) ([]domain.Task, []domain.TaskEdge, error)
	// GetSubtreeWithChildPercents mirrors GetSubtree but orders
	// deepest-first and folds in each node's direct children's current
	// progress_percent — the shape usecase.RecalculateProgress reduces
	// bottom-up through domain.CalculateProgress.
	GetSubtreeWithChildPercents(ctx context.Context, tenantID, rootID string) ([]SubtreeProgressNode, error)
	// BatchUpdateProgress persists every (taskID -> progress_percent) pair
	// in one call — task-service.md §8's N+1 guard.
	BatchUpdateProgress(ctx context.Context, tenantID string, updates map[string]int) error
	// CompleteExecution is the simple path's (TASK-TG-04-03) and, via
	// TASK-TG-04-05's ReportTaskExecutionResult, the complex path's terminal
	// write: sets status, actual_hours, and clears agent_session_id in one
	// statement.
	CompleteExecution(ctx context.Context, tenantID, id, status string, actualHours float64) error
}

// SubtreeProgressNode is one GetSubtreeWithChildPercents result row: the
// task itself, its depth (root=0), and its direct children's CURRENT
// progress_percent values as persisted.
type SubtreeProgressNode struct {
	Task          domain.Task
	Depth         int
	ChildPercents []int
}

// EdgeRepository is the persistence port for task_edges rows. Cycle
// detection is NOT this port's job — the AddEdge usecase calls
// domain.DetectCycle itself, using edges this port fetched, per Clean
// Architecture's dependency rule (domain logic never lives in an adapter).
type EdgeRepository interface {
	// Add persists a single edge. Callers (the AddEdge usecase) are
	// responsible for running cycle detection before calling this for a
	// depends_on edge — see task-service.md §8's "Edge-mutation
	// consistency" note and this service's README for the known gap this
	// scaffold has around doing the check-then-write atomically.
	Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error
	// ListByKind returns every edge of the given kind for the tenant — the
	// graph AddEdge's cycle check walks. Fetching the whole kind-scoped
	// edge set (rather than only the reachable subgraph) keeps this port's
	// contract simple; see this service's README for the scale follow-up.
	ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
	// ListFrom returns the edges of the given kind originating at
	// fromTaskID — used by the Execute usecase's complexity branch (§3.1)
	// and reused as-is by GetDependencies (TASK-223) for the identical
	// depends_on edge kind.
	ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
	// ListByKindForUpdate is ListByKind's transaction-scoped, row-locked
	// variant — SELECT ... FOR UPDATE over the kind-scoped edge set, closing
	// the check-then-write race AddEdge's prior two-call shape allowed. Only
	// meaningful when called through TxRunner.RunInTx's fn (r.db is a
	// pgx.Tx there); called outside a transaction it behaves like ListByKind.
	ListByKindForUpdate(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
	// ListTo returns the edges of the given kind terminating AT toTaskID —
	// the symmetric counterpart to ListFrom, used by UpdateTask's un-block
	// step to find a task's dependents.
	ListTo(ctx context.Context, tenantID, toTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
}

// GrantRepository is the persistence port for task_grants rows.
type GrantRepository interface {
	// Grant returns the persisted grant's id — needed by RevokeGrant
	// callers (TASK-TG-03-04's GrantResponse.id).
	Grant(ctx context.Context, tenantID string, grant domain.Grant) (string, error)
	// ListGrantsForAncestors returns every grant recorded against any of
	// taskIDs, grouped by task ID — the input ResolveGrant's BFS walk
	// (domain/grant_resolution.go) consumes. Excludes expired rows.
	ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error)
	// Revoke deletes a grant by id — a nonexistent grant_id is a real
	// error, never a silent no-op.
	Revoke(ctx context.Context, tenantID, grantID string) error
	// ListGrantsForTask returns only the grants recorded directly against
	// taskID — NOT the ancestor chain, per usecase.ListGrants's doc
	// comment (avoids leaking an ancestor's grant details).
	ListGrantsForTask(ctx context.Context, tenantID, taskID string) ([]domain.Grant, error)
}

// ShareLinkRepository is the persistence port for task.task_share_links
// rows — the public/anonymous share-link flow (TASK-TG-03-08). Never
// stores or returns a plaintext token, only its SHA-256 hash.
type ShareLinkRepository interface {
	Create(ctx context.Context, tenantID, taskID, tokenHash, createdBy string) (id string, err error)
	ResolveActive(ctx context.Context, tenantID, tokenHash string) (taskID string, err error)
	Revoke(ctx context.Context, tenantID, linkID string) error
	TaskIDFor(ctx context.Context, tenantID, linkID string) (taskID string, err error)
}

// EventPublisher writes a best-effort outbox row for async consumption
// (notification-service) — see internal/adapter/eventbus's doc comment for
// the outbox-write + common/outbox.Relay polling-publish implementation,
// mirroring usage-service's transactional-outbox pattern. No error return:
// a missed audit event must never fail the grant mutation it describes.
type EventPublisher interface {
	Publish(ctx context.Context, tenantID, eventType string, payload map[string]any)
}

// CommentRepository is the persistence port for task.task_comments rows —
// see SOL-TG-01's AddComment/ListComments design.
type CommentRepository interface {
	AddComment(ctx context.Context, tenantID string, c domain.TaskComment) (domain.TaskComment, error)
	ListComments(ctx context.Context, tenantID, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error)
}

// TeamScopeResolver resolves a user's team memberships by calling
// tenant-service — task-service never reads tenant-service's team_members
// table directly, per task-service.md §2/§9's bounded-context rule.
//
// STUB in this scaffold: internal/adapter/grpcclient's implementation
// always returns an empty team list rather than actually calling
// tenant-service. Team-scoped grants will silently never match until this
// is wired to a real tenant-service client — see this service's README.
type TeamScopeResolver interface {
	ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error)
}

// OPAClient is the authorization port ResolvePermission uses for the
// "does this resolved level authorize this action" decision (§9's
// domain-computes/OPA-decides split) — implemented by
// internal/adapter/opaclient against the shared embedded OPA evaluator
// (common/policy), consuming
// backend-go/policy/orca-authz/task_grant.rego's
// data.orca.authz.task.allow rule.
type OPAClient interface {
	// Decision reports whether level authorizes action for tenantID, per
	// task_grant.rego's level_actions table. Never called with
	// domain.GrantLevelUnspecified — ResolvePermission.Execute returns its
	// own PermissionDenied before reaching this call when domain.ResolveGrant
	// finds no match.
	Decision(ctx context.Context, level domain.GrantLevel, action, tenantID string) (bool, error)
}

// SimpleExecutor relays Execute's simple-path dispatch to
// infra-fleet-service (-> Dev Server Agent agent.execPrompt), per
// task-service.md §3.1.
//
// internal/adapter/grpcclient's implementation is real (TASK-224): resolves
// the task's project_id to a connectionId + worktreePath via
// ProjectExecutionResolver and relays through infra-fleet-service's Relay
// RPC, method "agent.execPrompt" (not "agent.exec" — see that file's doc
// comment for the full agent-rpc-catalog citation trail behind that
// choice).
type SimpleExecutor interface {
	Execute(ctx context.Context, tenantID, taskID, requestID string) (executionRef string, err error)
}

// ComplexExecutor relays Execute's complex-path dispatch to
// orchestration-service's coordinator, per task-service.md §3.1.
//
// STUB in this scaffold: internal/adapter/grpcclient's implementation
// returns a fixed placeholder execution ref without calling
// orchestration-service — see this service's README.
type ComplexExecutor interface {
	// worktreeID is resolved by ExecuteTask's own worktree reuse-or-create
	// step (TASK-TG-04-02/03) before dispatch — threaded through so
	// orchestration-service's coordinator_run knows which worktree its
	// dispatched work runs in.
	Execute(ctx context.Context, tenantID, taskID, requestID, worktreeID string) (executionRef string, err error)
}

// ProjectExecutionResolver resolves a project's execution target
// (connectionId, or none for host-local) via infra-fleet-service —
// task-service never calls project-service or infra-fleet-service directly
// from this port's consumers (SimpleExecutor, AIDecompose); the resolution
// itself lives in internal/adapter/grpcclient, mirroring
// git-gateway-service's ConnectionResolver split.
//
// worktreePath is infra-fleet-service's ResolveConnectionResponse.repo_path
// echoed straight through — the same field git-gateway-service's
// ConnectionResolver folds into ResolvedConnection.RepoPath
// (git-gateway-service/internal/usecase/ports.go:26-37) for its GitExecutor
// to operate against. SimpleExecutor (TASK-224 Gap 1) needs this same value
// as agent.execPrompt's required worktreePath field — see
// simple_executor.go's doc comment for the full citation trail.
// worktreeID (the 3rd return value) is infra-fleet-service's
// ResolveConnectionResponse.worktree_id echoed straight through — added
// alongside worktreePath so callers that need git-gateway-service's
// worktree-ID-keyed RPCs (TechStackDetector's ReadFile, TASK-TG-02-03) have
// it without a second resolution call. worktreePath and worktreeID name
// different things (a filesystem path vs. git-gateway-service's own ID) —
// keep both rather than conflating them.
// TechStackDetector probes a project's worktree for common ecosystem
// marker files (package.json, go.mod, ...) to enrich AIDecompose's prompt
// — best-effort by design: a detection failure must never fail Execute
// outright, since this is an enrichment, not a precondition for producing
// a plan at all.
type TechStackDetector interface {
	Detect(ctx context.Context, tenantID, projectID string) (domain.TechStack, error)
}

type ProjectExecutionResolver interface {
	ResolveConnection(ctx context.Context, tenantID, projectID string) (connectionID, worktreePath, worktreeID string, connected bool, err error)
}

// WorktreeProvisioner implements Execute's "reuse or create" worktree step
// (SOL-TG-04) — a task with an existing WorktreeID reuses it; otherwise a
// new one is created via git-gateway-service's existing CreateWorktree
// saga.
type WorktreeProvisioner interface {
	EnsureWorktree(ctx context.Context, tenantID string, task domain.Task) (worktreeID, path string, err error)
}

// ProjectContextResolver resolves a project's name/repo URL via
// project-service — task-service never reads project-service's tables
// directly.
type ProjectContextResolver interface {
	Resolve(ctx context.Context, tenantID, projectID string) (name, repoURL string, err error)
}

// VelocityResolver returns the last n Done tasks in a project (title +
// actual hours), used to give the AI a sense of this team's real pace.
type VelocityResolver interface {
	RecentCompletedTasks(ctx context.Context, tenantID, projectID string, n int) ([]domain.Task, error)
}

// AIProviderContextResolver resolves AI provider/account context for a
// tenant+user by calling ai-provider-service — distinct from
// ProjectExecutionResolver (execution target) and from AICompleter below
// (the actual completion call).
type AIProviderContextResolver interface {
	ResolveContext(ctx context.Context, tenantID, userID string) (providerContext string, err error)
}

// AICompleter relays a prompt to the Dev Server Agent's ai.complete method
// over a resolved connectionID — same port shape as
// git-gateway-service.AICompleter, implemented here against
// infra-fleet-service's Relay RPC rather than duplicated per-service.
type AICompleter interface {
	Complete(ctx context.Context, connectionID, prompt string) (string, error)
}

// TxRunner wraps a set of subtask creates + parent-link edges in one
// Postgres transaction — closes TASK-224 Gap 2 (AIApply's create-subtask +
// add-edge loop was not atomic: a mid-loop failure could leave a partial
// subtree). A real precedent for this shape now exists in this repo (it did
// not when ai_apply.go was first written) —
// credential-broker-service/internal/usecase/ports.go's TxRunner
// (RunInTx(ctx, fn func(ctx, metadataRepo, auditRepo) error) error),
// implemented via pgx.BeginFunc in that service's postgres.Repository. This
// port mirrors that shape exactly: RunInTx hands fn a TaskRepository/
// EdgeRepository pair already scoped to the open transaction, reusing the
// exact port shapes AIApply's CreateTask/AddEdge sub-usecases already know,
// rather than inventing transaction-specific interfaces or a generic
// UnitOfWork abstraction. If fn returns a non-nil error, every write fn made
// through either repo rolls back together. Implemented by
// internal/adapter/postgres.Repository.
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context, tasks TaskRepository, edges EdgeRepository) error) error
}
