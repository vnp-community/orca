// Package usecase holds project-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
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
	RecordWorktreeCreated(ctx context.Context, worktree domain.Worktree) (domain.Worktree, error)
	// RecordWorktreeRemoved hard-deletes the worktree row — a deliberate
	// choice over a soft-removed flag: this table is disposable metadata
	// with git-gateway-service as the source of truth for lineage/history,
	// so there is no reporting/audit need for a tombstone row here. See
	// this service's README for the explicit decision record.
	RecordWorktreeRemoved(ctx context.Context, worktreeID string) error
	ListWorktrees(ctx context.Context, projectID string) ([]domain.Worktree, error)
	SetWorktreeActivation(ctx context.Context, worktreeID string, active bool) (domain.Worktree, error)
	RenameWorktree(ctx context.Context, worktreeID, branch string) (domain.Worktree, error)
	// FindWorktreeByIdempotencyKey backs BR-CLI-01 — see
	// GetWorktreeByIdempotencyKey's doc comment. found=false, err=nil means
	// "no match yet", not an error.
	FindWorktreeByIdempotencyKey(ctx context.Context, projectID, idempotencyKey string) (domain.Worktree, bool, error)
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
