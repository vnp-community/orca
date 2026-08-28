// Package usecase holds project-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// ProjectRepository is the persistence port for projects and project
// membership. Implemented by internal/adapter/postgres against
// project-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type ProjectRepository interface {
	Create(ctx context.Context, project domain.Project) (domain.Project, error)
	Get(ctx context.Context, tenantID, id string) (domain.Project, error)
	List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error)
	AddMember(ctx context.Context, member domain.ProjectMember) error
	// UpdateDevServerID rebinds a project to a new dev server — the only
	// write path for dev_server_id (see usecase.RebindDevServer). Returns
	// domain.ErrProjectNotFound (wrapped) if no project matches.
	UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error)
	// UpdateProject applies patch's non-empty fields only ("" = no change,
	// per project.proto's UpdateProjectRequest doc comment). Never touches
	// dev_server_id. Returns domain.ErrProjectNotFound (wrapped) if no
	// project matches.
	UpdateProject(ctx context.Context, tenantID, projectID string, patch domain.ProjectUpdatePatch) (domain.Project, error)
	// DeleteProject hard-deletes a project row. Child rows (project_members,
	// repos, worktrees) cascade via ON DELETE CASCADE FKs — see
	// migrations/0002's comment and this service's README "DeleteProject
	// cascade" note. Returns domain.ErrProjectNotFound (wrapped) if no
	// project matches.
	DeleteProject(ctx context.Context, tenantID, projectID string) error
	// GetMembership returns the caller's membership row for a project —
	// used by requireProjectAccess (authorization.go) to resolve
	// caller_project_role for the OPA policy check. Returns
	// domain.ErrMembershipNotFound (wrapped) if the user has no membership
	// row for this project; this is the normal "not a member" case, not an
	// error requireProjectAccess treats as a fetch failure.
	GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error)
	// ListMembers returns every membership row for a project.
	ListMembers(ctx context.Context, projectID string) ([]domain.ProjectMember, error)
	// RemoveMember deletes one membership row. Returns
	// domain.ErrMembershipNotFound if none exists.
	RemoveMember(ctx context.Context, projectID, userID string) error
	// UpdateMemberRole changes one membership row's role. Returns
	// domain.ErrMembershipNotFound if none exists.
	UpdateMemberRole(ctx context.Context, projectID, userID string, role domain.ProjectRole) (domain.ProjectMember, error)
	// CountOwners is the read RemoveMember/UpdateMemberRole use to enforce
	// the "≥1 owner" invariant before mutating.
	CountOwners(ctx context.Context, projectID string) (int, error)
	// ListForMember returns only projects userID is a member of, within
	// tenantID — unlike List, which returns every tenant project regardless
	// of caller membership (a pre-existing gap this closes; ListProjects's
	// visibility filter is meaningless layered over an unscoped list). NEW.
	ListForMember(ctx context.Context, tenantID, userID, pageToken string, pageSize int32) ([]domain.Project, string, error)
}

// MembershipRepository is the read-only subset of ProjectRepository that
// usecases whose primary repository dependency is RepoRepository/
// WorktreeRepository (not ProjectRepository) take as an extra port, purely
// to resolve caller_project_role for requireProjectAccess. A
// *postgres.Repository satisfies this interface structurally — no adapter
// duplication needed, see cmd/server/main.go's wiring.
type MembershipRepository interface {
	GetMembership(ctx context.Context, projectID, userID string) (domain.ProjectMember, error)
}

// OPAClient is the authorization port every OPA-gated usecase in this
// service uses for the "does this caller_project_role/caller_global_role
// authorize this action" decision — implemented by internal/adapter/
// opaclient against the shared embedded OPA evaluator (common/policy),
// consuming backend-go/policy/orca-authz/project.rego's
// data.orca.authz.project.allow rule. Mirrors auth-service/
// annotation-service/task-service's own OPAClient port shape.
type OPAClient interface {
	// Decision reports whether callerProjectRole/callerGlobalRole authorizes
	// action, per project.rego's {"caller_project_role", "caller_global_role",
	// "action"} input contract. action is one of the projectAction* constants
	// in authorization.go.
	Decision(ctx context.Context, callerProjectRole, callerGlobalRole, action string) (bool, error)
}

// WorkflowExecutionChecker is the outbound port toward workflow-service —
// RebindDevServer's and DeleteProject's guard against mutating a project
// with a running workflow. Implemented by internal/adapter/grpcclient. See
// project-service.md §3 ("closing the active-execution gap").
type WorkflowExecutionChecker interface {
	HasActiveExecutions(ctx context.Context, projectID string) (bool, error)
}

// TaskExecutionChecker is the outbound port toward task-service — the same
// guard as WorkflowExecutionChecker but for standalone task executions. Kept
// as a separate interface (not merged with WorkflowExecutionChecker) per
// project-service.md §6, so RebindDevServer's/DeleteProject's test fakes
// stay independent of whichever of the two services changes its contract
// first.
type TaskExecutionChecker interface {
	HasActiveExecutions(ctx context.Context, projectID string) (bool, error)
}

// RepoRepository is the persistence port for a project's repo catalog.
// Implemented by internal/adapter/postgres against project.repos.
type RepoRepository interface {
	// AddRepo persists repo and assigns it the next position within its
	// project (MAX(position)+1, or 0 for the first repo) — position is
	// never caller-supplied on add, only via ReorderRepos afterward.
	AddRepo(ctx context.Context, repo domain.Repo) (domain.Repo, error)
	// ListRepos returns a project's repos ordered by position.
	ListRepos(ctx context.Context, projectID string) ([]domain.Repo, error)
	// ReorderRepos rewrites every listed repo's position (0-indexed, by
	// list order) in a single transaction. Callers must have already
	// validated idsInOrder is an exact permutation of the project's
	// existing repo ids — see usecase.ReorderRepos.
	ReorderRepos(ctx context.Context, projectID string, idsInOrder []string) error
	// RemoveRepo hard-deletes a repo. Leaving a position gap in its former
	// project is fine — see domain.Repo.Position's doc comment.
	RemoveRepo(ctx context.Context, repoID string) error
	// GetRepo returns a single repo by id — used by usecase.RemoveRepo to
	// resolve its owning project_id for the OPA owner-only authorization
	// check, since RemoveRepoInput carries only a repo_id. Returns
	// domain.ErrRepoNotFound (wrapped) if no repo matches.
	GetRepo(ctx context.Context, repoID string) (domain.Repo, error)
	// Update persists repo's current url/display_name — used by
	// usecase.UpdateRepo after it applies the field-mask. Returns
	// domain.ErrRepoNotFound (wrapped) if no repo matches.
	Update(ctx context.Context, repo domain.Repo) (domain.Repo, error)
}

// WorktreeRepository is the persistence port for a project's worktree
// metadata. Implemented by internal/adapter/postgres against
// project.worktrees. See domain.Worktree's doc comment: never authoritative
// for on-disk existence.
type WorktreeRepository interface {
	// RecordWorktreeCreated inserts the worktree row and durably enqueues
	// event as an outbox row, in the SAME transaction — the
	// transactional-outbox pattern (05-data-architecture.md).
	RecordWorktreeCreated(ctx context.Context, worktree domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error)
	// RecordWorktreeRemoved hard-deletes the worktree row — a deliberate
	// choice over a soft-removed flag: this table is disposable metadata
	// with git-gateway-service as the source of truth for lineage/history,
	// so there is no reporting/audit need for a tombstone row here. See
	// this service's README for the explicit decision record.
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
	// ListWorktrees returns projectID's worktrees, optionally filtered by
	// statusIn (nil/empty = no filter) and olderThan (nil = no filter) —
	// BL-AT-04's cleanup_worktrees step candidate query.
	ListWorktrees(ctx context.Context, projectID string, statusIn []string, olderThan *time.Time) ([]domain.Worktree, error)
	// GetWorktree looks up a single worktree by id — backs the new
	// GetWorktree RPC (SOL-WT-04).
	GetWorktree(ctx context.Context, worktreeID string) (domain.Worktree, error)
	SetWorktreeActivation(ctx context.Context, worktreeID string, active bool) (domain.Worktree, error)
	RenameWorktree(ctx context.Context, worktreeID, branch string) (domain.Worktree, error)
	// FindWorktreeByIdempotencyKey backs BR-CLI-01 — see
	// GetWorktreeByIdempotencyKey's doc comment. found=false, err=nil means
	// "no match yet", not an error.
	FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error)

	// CreateWorktreeWithEvent inserts worktree and its worktree.created
	// outbox event in ONE transaction (SOL-PI-03) — see usage-service's
	// SaveSession(ctx, session, event) for the precedent this follows.
	// RecordWorktreeCreated (used by usecase.RecordWorktreeCreated as of
	// TASK-PI-03-03) now calls this instead of the bare method above.
	CreateWorktreeWithEvent(ctx context.Context, worktree domain.Worktree, event domain.OutboxEvent) (domain.Worktree, error)
	// RemoveWorktreeWithEvent deletes the row and enqueues worktree.deleted
	// in the same transaction. buildEvent is called with the just-deleted
	// row (for its linked-issue fields) so the caller can build the event
	// payload without a separate pre-delete read.
	RemoveWorktreeWithEvent(ctx context.Context, worktreeID string, buildEvent func(removed domain.Worktree) domain.OutboxEvent) error
}

// ProjectGroupRepository is the persistence port for the folder-style
// project-group tree. Implemented by internal/adapter/postgres against
// project.project_groups.
type ProjectGroupRepository interface {
	CreateProjectGroup(ctx context.Context, group domain.ProjectGroup) (domain.ProjectGroup, error)
	// GetProjectGroup is used by CreateProjectGroup to validate a supplied
	// parent_group_id actually exists (mirrors workflow-service's
	// CreateTemplate parent-existence-check convention) — not exposed on
	// the RPC surface itself.
	GetProjectGroup(ctx context.Context, tenantID, id string) (domain.ProjectGroup, error)
	// UpdateProjectGroup renames a group only — parent_group_id is never
	// rewritten through this path, see usecase.UpdateProjectGroup's doc
	// comment for why.
	UpdateProjectGroup(ctx context.Context, tenantID, id, name string) (domain.ProjectGroup, error)
	// DeleteProjectGroup deletes a group; descendants cascade (ON DELETE
	// CASCADE on parent_group_id — see migrations/0005).
	DeleteProjectGroup(ctx context.Context, tenantID, id string) error
	ListProjectGroups(ctx context.Context, tenantID string) ([]domain.ProjectGroup, error)
	// UpsertLeafGroupForProject finds-or-creates projectID's own leaf group
	// row (project_id = projectID, DB-enforced unique — migrations/0006)
	// and sets its parent_group_id. Used by usecase.MoveProject.
	UpsertLeafGroupForProject(ctx context.Context, tenantID, projectID, projectName, targetParentGroupID string) (domain.ProjectGroup, error)
	// ImportNested creates one ProjectGroup + one Project (+ its first Repo,
	// pointed at candidate.Path) per selected candidate, atomically. See
	// this task's Context for why this is one hand-rolled repository
	// method rather than composed usecase calls.
	ImportNested(ctx context.Context, tenantID, createdBy, devServerID, parentGroupID string, candidates []domain.NestedRepoCandidate) ([]domain.ProjectGroup, []domain.Project, error)
}

// DevServerRelay is the outbound port toward infra-fleet-service's
// connection+relay primitives — implemented by internal/adapter/grpcclient
// against infrafleetv1.InfraFleetServiceClient's already-generic
// CreateConnection/Relay RPCs. Deliberately separate from
// WorkflowExecutionChecker/TaskExecutionChecker (port-per-concern
// convention, 03-clean-architecture-guidelines.md).
type DevServerRelay interface {
	CreateConnection(ctx context.Context, devServerID, repoPath, worktreeID string) (connectionID string, err error)
	Relay(ctx context.Context, connectionID, method string, paramsJSON []byte) (resultJSON []byte, err error)
}

// HostSetupRepository is the persistence port for the pre-project
// dev-server-folder wizard. Implemented by internal/adapter/postgres
// against project.project_host_setups (migrations/0007).
type HostSetupRepository interface {
	Create(ctx context.Context, setup domain.HostSetup) (domain.HostSetup, error)
	Get(ctx context.Context, tenantID, id string) (domain.HostSetup, error)
	List(ctx context.Context, tenantID string) ([]domain.HostSetup, error)
	// Update applies patch's non-empty fields only. Returns
	// domain.ErrHostSetupNotFound if no row matches.
	Update(ctx context.Context, tenantID, id string, patch domain.HostSetupPatch) (domain.HostSetup, error)
	Delete(ctx context.Context, tenantID, id string) error
	// SetStatus is SetupExistingFolder's failure-path write (-> Failed).
	SetStatus(ctx context.Context, tenantID, id string, status domain.HostSetupStatus) error
	// Complete is SetupExistingFolder's success-path write: sets status to
	// Completed and stamps project_id in one statement.
	Complete(ctx context.Context, tenantID, id, projectID string) (domain.HostSetup, error)
}

// DevServerLister backs CreateHostSetup's dev_server_id validation —
// infra-fleet-service has no GetDevServer RPC (only ListDevServers), so
// validation is "does this id appear in this tenant's dev server list."
// Implemented by internal/adapter/grpcclient against the already-dialed
// infrafleetv1.InfraFleetServiceClient.
type DevServerLister interface {
	Exists(ctx context.Context, tenantID, devServerID string) (bool, error)
}

// TerminalStatusResolver is the outbound port toward infra-fleet-service's
// PTY/terminal-session surface — GetMobileWorktreeStatus's ONE cross-service
// dependency (SOL-MB-04), implemented by internal/adapter/grpcclient against
// infrafleetv1.InfraFleetServiceClient. Kept as infrafleetv1 DTOs rather than
// a project-service domain type: this data never persists here, it is
// composed fresh into MobileWorktreeStatus on every call, so an extra
// translation layer would buy nothing.
type TerminalStatusResolver interface {
	// ListSessionsForDevServer resolves devServerID to its live connection
	// and lists its terminal sessions — nil, nil (not an error) when the dev
	// server has no live connection.
	ListSessionsForDevServer(ctx context.Context, devServerID string) ([]*infrafleetv1.TerminalSession, error)
	// GetAgentStatus fetches one ptyId's AgentKind/AgentRunning/
	// ReadyForInput — TerminalSession itself doesn't carry these (see
	// GetMobileWorktreeStatus's doc comment for the extra-RPC-cost tradeoff
	// this implies).
	GetAgentStatus(ctx context.Context, ptyID string) (*infrafleetv1.GetTerminalAgentStatusResponse, error)
}

// FolderWorkspaceRepository is the persistence port for FolderWorkspace —
// see domain.FolderWorkspace's doc comment for why this is a standalone
// entity, not a ProjectGroup extension. Implemented by internal/adapter/
// postgres against project.folder_workspaces.
type FolderWorkspaceRepository interface {
	Create(ctx context.Context, fw domain.FolderWorkspace) (domain.FolderWorkspace, error)
	// Update renames a folder workspace — the only mutable field, per
	// project.proto's UpdateFolderWorkspaceRequest doc comment. Returns
	// domain.ErrFolderWorkspaceNotFound (wrapped) if no row matches.
	Update(ctx context.Context, id, name string) (domain.FolderWorkspace, error)
	// Delete returns domain.ErrFolderWorkspaceNotFound (wrapped) if no row
	// matches.
	Delete(ctx context.Context, id string) error
	ListByTenant(ctx context.Context, tenantID string) ([]domain.FolderWorkspace, error)
	// FindByPath returns nil, nil (not an error) when no row matches — used
	// by usecase.FolderWorkspaceUseCase.GetPathStatus to distinguish
	// PathStatusAlreadyFolderWorkspace from the next check.
	FindByPath(ctx context.Context, tenantID, devServerID, path string) (*domain.FolderWorkspace, error)
	// RepoPathExists cross-checks against project.worktrees (joined through
	// project.projects for its dev_server_id — project.repos itself has no
	// filesystem-path column, see the postgres adapter's doc comment) so
	// GetFolderWorkspacePathStatus can distinguish PathStatusAlreadyRepo from
	// PathStatusAvailable.
	RepoPathExists(ctx context.Context, tenantID, devServerID, path string) (bool, error)
	// Get returns nil, nil (not an error) when no row matches — used by
	// Update/Delete's ownership check (usecase.FolderWorkspaceUseCase) to
	// load the caller's added_by before mutating.
	Get(ctx context.Context, id string) (*domain.FolderWorkspace, error)
}

// DevServerHealthChecker is the outbound port toward infra-fleet-service's
// GetFleetHealth RPC — CreateProject/RebindDevServer's online/health guard.
// Genuinely new: infra-fleet-service.md §1 already documents fleet health
// monitoring, but no caller in this service used it before this task.
type DevServerHealthChecker interface {
	// IsReachable fails closed on error — a health-check outage must never
	// silently bind/rebind to a server that might be down.
	IsReachable(ctx context.Context, tenantID, devServerID string) (bool, error)
}

// AuditPublisher is the outbound port RebindDevServer calls after a
// successful rebind to emit a security-relevant audit event — outbox
// pattern (05-data-architecture.md), not a synchronous call to another
// service. A nil AuditPublisher is valid — callers must nil-check, same
// convention as tenant-service's CacheInvalidationPublisher.
type AuditPublisher interface {
	PublishAuditEvent(ctx context.Context, tenantID, actorID, action, target string) error
}

// MemberNotifier is the outbound port RebindDevServer calls after a
// successful rebind to notify every project member — best-effort, same
// outbox posture as AuditPublisher. A nil MemberNotifier is valid.
type MemberNotifier interface {
	NotifyDevServerChanged(ctx context.Context, tenantID string, userIDs []string, projectID, oldDevServerID, newDevServerID string) error
}

// ProfileResolver is the outbound port ListProjects uses to resolve the
// caller's ResolvedProfile for the fleet.allowedServerTags visibility
// filter — a NEW outbound edge from project-service to tenant-service
// (tenant-service.md §3/§7 already documents GetResolvedProfile as callable
// by any service, just not exercised by project-service before this task).
// DevServerTags resolves a dev server's tags via infra-fleet-service.ListDevServers.
type ProfileResolver interface {
	GetResolvedProfile(ctx context.Context, tenantID, userID string) (ResolvedProfileView, error)
	DevServerTags(ctx context.Context, tenantID, devServerID string) ([]string, error)
}

// ResolvedProfileView is the subset of tenant-service's ResolvedProfile this
// service actually reads — decoded from GetResolvedProfileResponse's
// resolved_settings_json by the adapter, not the raw JSON map, so usecase/
// code never touches encoding/json directly.
type ResolvedProfileView struct {
	allowedServerTags []string
	hasRestriction    bool
}

// NewResolvedProfileView constructs a ResolvedProfileView — called by
// internal/adapter/infrafleetclient.ProfileResolver's implementation (or
// wherever the JSON is decoded) after reading fleet.allowedServerTags out of
// the decoded resolved_settings_json. hasRestriction distinguishes "key
// absent" (false, unrestricted) from "key present, possibly empty" (true) —
// see domain/profile_resolution.go's mergeAllowedServerTags doc comment
// (tenant-service, SOL-PRF-02) for why this distinction must survive.
func NewResolvedProfileView(tags []string, hasRestriction bool) ResolvedProfileView {
	return ResolvedProfileView{allowedServerTags: tags, hasRestriction: hasRestriction}
}

// AllowedServerTags returns the tag allowlist and whether one is defined at
// all (false = unrestricted, filter nothing).
func (v ResolvedProfileView) AllowedServerTags() ([]string, bool) {
	return v.allowedServerTags, v.hasRestriction
}

// DevServerHostnameResolver resolves a dev server id to its host string via
// infra-fleet-service.ListDevServers — best-effort, used only by
// GetProjectContext's display-only dev_server_hostname field. A lookup
// failure never fails the whole GetProjectContext read.
type DevServerHostnameResolver interface {
	Hostname(ctx context.Context, tenantID, devServerID string) (string, error)
}
