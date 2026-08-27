# TASK-138: Implement `MoveProject`/`ScanNested`/`ImportNested` usecases, `DevServerRelay` outbound client, migration

**From Solution:** SOL-021
**Priority:** P1
**Service:** `project-service`
**File:** `migrations/0006_project_groups_project_id.up.sql`/`.down.sql` (new), `internal/domain/project_group.go`, `internal/usecase/ports.go`, `internal/usecase/move_project.go` (new), `internal/usecase/scan_nested.go` (new), `internal/usecase/import_nested.go` (new), `internal/adapter/postgres/project_group_repository.go`, `internal/adapter/grpcclient/dev_server_relay.go` (new), `internal/adapter/grpc/server.go`, `internal/config/config.go`, `cmd/server/main.go`
**Depends on:** TASK-137
**Status:** `[x]` DONE — implemented, incl. `ImportNested` gaining a `DevServerID` param the task sketch omitted (the real generated proto has `dev_server_id`). Worktree `agent-a9271c5b2d89347e7`, uncommitted.

---

## Context — design decisions this task locks in

**Relay target: infra-fleet-service, not project-service's own host.**
`project-service` calls `infra-fleet-service`'s existing
`CreateConnection`/`Relay` RPCs directly — no new RPC needed there, since
`Relay` is already a generic `connectionId+method+params` passthrough
(`infrafleet.proto:31`'s own doc comment). This is the same dependency
`project-service.md` §7 already lists (`infra-fleet-service`, currently
unused by any shipped code path), one more call pattern against it.

**`devServerId` validation: no separate validation call.** The real
`project-service.CreateProject` does **not** actually validate
`dev_server_id` today (unlike what `project-service.md` §7 and SOL-021's
original sketch assumed — checked directly against the shipped
`create_project.go`, which takes no `DevServerID` field at all).
`infra-fleet-service` also has no `GetDevServer` RPC to validate against
(only `ListDevServers`). Simplest correct behavior, adopted here instead of
inventing a new RPC: let `CreateConnection` be the validation — an unknown
`dev_server_id` fails there and `ScanNested` fails closed with that error,
no separate pre-check.

**`ImportNested`'s transaction**: composing `CreateProjectGroup`'s and
`AddRepo`'s existing *usecases* can't share one DB transaction (they're
called through separate repository ports, each opening its own connection).
Instead, `ProjectGroupRepository` gets one new method,
`ImportNested`, that hand-rolls the multi-table insert inside a single
`pool.Begin(ctx)` transaction — the same "one repository method owns its
own multi-row transaction" convention `RepoRepository.ReorderRepos` already
uses (`internal/adapter/postgres/repo_repository.go`), just extended across
tables within the same database/schema.

**Open dependency, called out explicitly**: the JSON-RPC method name
`fs.scanNestedRepos` and the shape `domain.ParseNestedRepoCandidates`
expects from `RelayResponse.result_json` are this task's proposal, not
confirmed against the Dev Server Agent's actual handler. Confirm before
relying on `ScanNested` returning real candidates in an integration
environment.

## Changes to make

### Step 1 — migration: `migrations/0006_project_groups_project_id.up.sql`

```sql
-- project.project_groups.project_id — links a group to the specific
-- project it was created for during nested-repo import (MoveProject's
-- leaf-group node). Nullable: most groups stay pure organizational folders.
ALTER TABLE project.project_groups
    ADD COLUMN project_id UUID REFERENCES project.projects (id) ON DELETE CASCADE;

-- Partial unique index: at most one leaf group per project — enforces
-- UpsertLeafGroupForProject's find-or-create invariant at the DB layer,
-- not just in application code.
CREATE UNIQUE INDEX idx_project_groups_project_id
    ON project.project_groups (project_id) WHERE project_id IS NOT NULL;
```

`migrations/0006_project_groups_project_id.down.sql`:

```sql
DROP INDEX IF EXISTS project.idx_project_groups_project_id;
ALTER TABLE project.project_groups DROP COLUMN IF EXISTS project_id;
```

### Step 2 — `internal/domain/project_group.go`: add `ProjectID`, `NestedRepoCandidate`, `ParseNestedRepoCandidates`

Find:

```go
type ProjectGroup struct {
	ID       string
	TenantID string
	Name     string
	// ParentGroupID is empty for a root-of-tree group.
	ParentGroupID string
}
```

Replace with:

```go
type ProjectGroup struct {
	ID       string
	TenantID string
	Name     string
	// ParentGroupID is empty for a root-of-tree group.
	ParentGroupID string
	// ProjectID is empty for a pure organizational folder node; set only
	// for a project's own leaf group (see UpsertLeafGroupForProject).
	ProjectID string
}
```

Append (new file section):

```go
import "encoding/json"

// NestedRepoCandidate is one filesystem entry ScanNested found under a
// scanned root path — mirrors project.proto's NestedRepoCandidate.
type NestedRepoCandidate struct {
	Path          string
	SuggestedName string
	IsGitRepo     bool
}

// nestedRepoCandidateWire is ParseNestedRepoCandidates's JSON decoding
// shape for one Dev Server Agent-reported candidate — snake_case keys,
// matching this codebase's other Agent JSON-RPC payloads (e.g.
// infra-fleet-service's ScanWorkspacePorts result shape). NOT yet
// confirmed against a real Agent handler — see this task's Context.
type nestedRepoCandidateWire struct {
	Path          string `json:"path"`
	SuggestedName string `json:"suggested_name"`
	IsGitRepo     bool   `json:"is_git_repo"`
}

// ParseNestedRepoCandidates decodes a Relay call's result_json into
// candidates — pure domain-layer JSON->struct mapping, no I/O (usecase.ScanNested
// is the one caller).
func ParseNestedRepoCandidates(resultJSON []byte) ([]NestedRepoCandidate, error) {
	var wire struct {
		Candidates []nestedRepoCandidateWire `json:"candidates"`
	}
	if err := json.Unmarshal(resultJSON, &wire); err != nil {
		return nil, err
	}
	out := make([]NestedRepoCandidate, 0, len(wire.Candidates))
	for _, c := range wire.Candidates {
		out = append(out, NestedRepoCandidate{Path: c.Path, SuggestedName: c.SuggestedName, IsGitRepo: c.IsGitRepo})
	}
	return out, nil
}
```

(Move the `import "encoding/json"` into this file's existing top-level
import block rather than a second `import` statement — Go doesn't allow two
`import` blocks with an unnamed single import like this; the sketch above
separates it only for readability in this task doc.)

### Step 3 — `internal/usecase/ports.go`: extend `ProjectGroupRepository`, add `DevServerRelay`

Find:

```go
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
}
```

Replace with:

```go
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
	ImportNested(ctx context.Context, tenantID, createdBy, parentGroupID string, candidates []domain.NestedRepoCandidate) ([]domain.ProjectGroup, []domain.Project, error)
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
```

### Step 4 — `internal/usecase/move_project.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type MoveProjectInput struct {
	ProjectID           string
	TargetParentGroupID string
}

// MoveProject requires owner (or global admin) — same tier as
// UpdateProject/DeleteProject, project-service.md §9.
type MoveProject struct {
	repo      ProjectRepository
	groupRepo ProjectGroupRepository
	opa       OPAClient
}

func NewMoveProject(repo ProjectRepository, groupRepo ProjectGroupRepository, opa OPAClient) *MoveProject {
	return &MoveProject{repo: repo, groupRepo: groupRepo, opa: opa}
}

func (uc *MoveProject) Execute(ctx context.Context, tenantID string, in MoveProjectInput) (domain.ProjectGroup, error) {
	if err := requireProjectAccess(ctx, uc.repo, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.ProjectGroup{}, err
	}

	if in.TargetParentGroupID != "" {
		if _, err := uc.groupRepo.GetProjectGroup(ctx, tenantID, in.TargetParentGroupID); err != nil {
			return domain.ProjectGroup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "target parent group does not exist", err)
		}
	}

	project, err := uc.repo.Get(ctx, tenantID, in.ProjectID)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project does not exist", err)
	}

	// A leaf group holding exactly one project_id has no children by
	// construction — no cycle check needed, same reasoning
	// domain.ErrGroupSelfParent's doc comment documents for the general
	// parent-assignment case.
	group, err := uc.groupRepo.UpsertLeafGroupForProject(ctx, tenantID, in.ProjectID, project.Name, in.TargetParentGroupID)
	if err != nil {
		return domain.ProjectGroup{}, apperrors.New(apperrors.KindInternal, "PROJECT_MOVE_FAILED", "failed to move project", err)
	}
	return group, nil
}
```

### Step 5 — `internal/usecase/scan_nested.go` (new)

```go
package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ScanNestedInput struct {
	DevServerID string
	RootPath    string
}

// ScanNested relays a filesystem scan to the Dev Server Agent via
// infra-fleet-service's CreateConnection+Relay — never checked against
// project-service's own host (the "legacy desktop-app assumption" BUG-021
// flags). Requires only an authenticated tenant — there is no project yet
// to check membership against at this pre-import stage.
type ScanNested struct {
	relay DevServerRelay
}

func NewScanNested(relay DevServerRelay) *ScanNested {
	return &ScanNested{relay: relay}
}

func (uc *ScanNested) Execute(ctx context.Context, in ScanNestedInput) ([]domain.NestedRepoCandidate, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	connID, err := uc.relay.CreateConnection(ctx, in.DevServerID, in.RootPath, "")
	if err != nil {
		// An unknown/unreachable dev_server_id fails here — the only
		// validation this usecase performs, see this task's Context.
		return nil, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED", "failed to connect to dev server", err)
	}
	params, err := json.Marshal(map[string]string{"path": in.RootPath})
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_SCAN_PARAMS_FAILED", "failed to encode scan params", err)
	}
	resultJSON, err := uc.relay.Relay(ctx, connID, "fs.scanNestedRepos", params)
	if err != nil {
		// Fails closed — no local-disk fallback, matching
		// infra-fleet-service.md §10's correctness bar for ScanWorkspacePorts.
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_SCAN_NESTED_FAILED", "failed to scan nested repos on dev server", err)
	}
	candidates, err := domain.ParseNestedRepoCandidates(resultJSON)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_SCAN_PARSE_FAILED", "failed to parse scan result", err)
	}
	return candidates, nil
}
```

### Step 6 — `internal/usecase/import_nested.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ImportNestedInput struct {
	ParentGroupID string
	Selected      []domain.NestedRepoCandidate
}

// ImportNested needs no relay call — once the user has selected candidates,
// materializing them into project_groups (+ a Project/Repo per group) is
// pure metadata writes, one DB transaction (see
// ProjectGroupRepository.ImportNested's doc comment).
type ImportNested struct {
	groupRepo ProjectGroupRepository
}

func NewImportNested(groupRepo ProjectGroupRepository) *ImportNested {
	return &ImportNested{groupRepo: groupRepo}
}

func (uc *ImportNested) Execute(ctx context.Context, in ImportNestedInput) ([]domain.ProjectGroup, []domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return nil, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	if in.ParentGroupID != "" {
		if _, err := uc.groupRepo.GetProjectGroup(ctx, tenantID, in.ParentGroupID); err != nil {
			return nil, nil, apperrors.New(apperrors.KindNotFound, "PROJECT_GROUP_NOT_FOUND", "parent group does not exist", err)
		}
	}

	groups, projects, err := uc.groupRepo.ImportNested(ctx, tenantID, userID, in.ParentGroupID, in.Selected)
	if err != nil {
		return nil, nil, apperrors.New(apperrors.KindInternal, "PROJECT_IMPORT_NESTED_FAILED", "failed to import nested repos", err)
	}
	return groups, projects, nil
}
```

### Step 7 — `internal/adapter/postgres/project_group_repository.go`: add the 2 new methods

Update `projectGroupColumns` and `scanProjectGroup` first:

```go
const projectGroupColumns = `id, tenant_id, name, COALESCE(parent_group_id::text, ''), COALESCE(project_id::text, '')`
```

```go
func scanProjectGroup(row rowScanner) (domain.ProjectGroup, error) {
	var g domain.ProjectGroup
	if err := row.Scan(&g.ID, &g.TenantID, &g.Name, &g.ParentGroupID, &g.ProjectID); err != nil {
		return domain.ProjectGroup{}, err
	}
	return g, nil
}
```

(Every existing `RETURNING `+projectGroupColumns` call site keeps working
unchanged — `scanProjectGroup` is their one scan point.)

Append:

```go
func (r *ProjectGroupRepository) UpsertLeafGroupForProject(ctx context.Context, tenantID, projectID, projectName, targetParentGroupID string) (domain.ProjectGroup, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.project_groups (id, tenant_id, name, parent_group_id, project_id)
		VALUES (gen_random_uuid(), $1, $3, $4, $2)
		ON CONFLICT (project_id) WHERE project_id IS NOT NULL
		DO UPDATE SET parent_group_id = EXCLUDED.parent_group_id
		RETURNING `+projectGroupColumns,
		tenantID, projectID, projectName, nullableString(targetParentGroupID),
	)
	out, err := scanProjectGroup(row)
	if err != nil {
		return domain.ProjectGroup{}, fmt.Errorf("postgres: upsert leaf project group: %w", err)
	}
	return out, nil
}

// ImportNested creates one ProjectGroup + one Project + one Repo per
// candidate, atomically — see usecase.ImportNested's doc comment for why
// this is one hand-rolled multi-table transaction rather than composed
// usecase calls (mirrors RepoRepository.ReorderRepos's existing
// "one repository method owns its own multi-row transaction" convention).
func (r *ProjectGroupRepository) ImportNested(ctx context.Context, tenantID, createdBy, parentGroupID string, candidates []domain.NestedRepoCandidate) ([]domain.ProjectGroup, []domain.Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("postgres: begin import nested transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	groups := make([]domain.ProjectGroup, 0, len(candidates))
	projects := make([]domain.Project, 0, len(candidates))

	for _, c := range candidates {
		name := c.SuggestedName
		if name == "" {
			name = c.Path
		}

		var p domain.Project
		if err := tx.QueryRow(ctx, `
			INSERT INTO project.projects (id, tenant_id, name, description, default_branch, visibility, created_by)
			VALUES (gen_random_uuid(), $1, $2, '', 'main', 'private', $3)
			RETURNING `+projectColumns,
			tenantID, name, createdBy,
		).Scan(&p.ID, &p.TenantID, &p.Name, &p.DevServerID, &p.Description, &p.DefaultBranch, &p.Visibility, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("postgres: insert imported project: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO project.repos (id, project_id, url, display_name, position)
			VALUES (gen_random_uuid(), $1, $2, $3, 0)
		`, p.ID, c.Path, name); err != nil {
			return nil, nil, fmt.Errorf("postgres: insert imported repo: %w", err)
		}

		var g domain.ProjectGroup
		if err := tx.QueryRow(ctx, `
			INSERT INTO project.project_groups (id, tenant_id, name, parent_group_id, project_id)
			VALUES (gen_random_uuid(), $1, $2, $3, $4)
			RETURNING `+projectGroupColumns,
			tenantID, name, nullableString(parentGroupID), p.ID,
		).Scan(&g.ID, &g.TenantID, &g.Name, &g.ParentGroupID, &g.ProjectID); err != nil {
			return nil, nil, fmt.Errorf("postgres: insert imported project group: %w", err)
		}

		projects = append(projects, p)
		groups = append(groups, g)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("postgres: commit import nested transaction: %w", err)
	}
	return groups, projects, nil
}
```

`ImportNested` above uses `project.repos`' `url` column to store the
absolute on-disk path — a remote-clone-URL-shaped column reused for "this
is already a folder on the dev server, not something to clone." Flagged
here rather than silently assumed: if `RepoRepository`/`domain.Repo` later
gain a distinct `path` field (TASK-141 adds exactly this for
`AddRepoRequest`), migrate this insert to use it instead.

### Step 8 — `internal/adapter/grpcclient/dev_server_relay.go` (new)

Mirrors `WorkflowExecutionChecker`/`TaskExecutionChecker`'s exact dial
pattern.

```go
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// DevServerRelay implements usecase.DevServerRelay by dialing
// infra-fleet-service's real CreateConnection/Relay RPCs — the same
// already-generic connectionId+method+params primitives
// devServer.*/fleet.* wscompat channels use (infrafleet.proto:31's doc
// comment). See usecase.ScanNested's doc comment for why project-service
// relays through the dev server rather than checking its own host.
type DevServerRelay struct {
	conn   *grpc.ClientConn
	client infrafleetv1.InfraFleetServiceClient
}

// NewDevServerRelay dials infra-fleet-service at addr. Lazy connection
// (grpc.NewClient doesn't block on connect), same convention as every
// other outbound client in this package.
func NewDevServerRelay(addr string) (*DevServerRelay, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return &DevServerRelay{conn: conn, client: infrafleetv1.NewInfraFleetServiceClient(conn)}, nil
}

func (c *DevServerRelay) Close() error {
	return c.conn.Close()
}

func (c *DevServerRelay) CreateConnection(ctx context.Context, devServerID, repoPath, worktreeID string) (string, error) {
	resp, err := c.client.CreateConnection(ctx, &infrafleetv1.CreateConnectionRequest{
		DevServerId: devServerID, RepoPath: repoPath, WorktreeId: worktreeID,
	})
	if err != nil {
		return "", fmt.Errorf("grpcclient: infra-fleet-service CreateConnection: %w", err)
	}
	return resp.GetConnectionId(), nil
}

func (c *DevServerRelay) Relay(ctx context.Context, connectionID, method string, paramsJSON []byte) ([]byte, error) {
	resp, err := c.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: method, ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: infra-fleet-service Relay: %w", err)
	}
	return []byte(resp.GetResultJson()), nil
}
```

### Step 9 — `internal/adapter/grpc/server.go`: register the 3 new RPC handlers

Add 3 fields (`moveProject`, `scanNested`, `importNested`) to
`Server`/`Deps`/`New`, mechanically identical to the existing
`ProjectGroup` fields' wiring.

```go
func (s *Server) MoveProject(ctx context.Context, req *projectv1.MoveProjectRequest) (*projectv1.MoveProjectResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err))
	}
	group, err := s.moveProject.Execute(ctx, tenantID, usecase.MoveProjectInput{
		ProjectID: req.GetProjectId(), TargetParentGroupID: req.GetTargetParentGroupId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.MoveProjectResponse{Group: toProtoProjectGroup(group)}, nil
}

func (s *Server) ScanNested(ctx context.Context, req *projectv1.ScanNestedRequest) (*projectv1.ScanNestedResponse, error) {
	candidates, err := s.scanNested.Execute(ctx, usecase.ScanNestedInput{
		DevServerID: req.GetDevServerId(), RootPath: req.GetRootPath(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.NestedRepoCandidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, &projectv1.NestedRepoCandidate{Path: c.Path, SuggestedName: c.SuggestedName, IsGitRepo: c.IsGitRepo})
	}
	return &projectv1.ScanNestedResponse{Candidates: out}, nil
}

func (s *Server) ImportNested(ctx context.Context, req *projectv1.ImportNestedRequest) (*projectv1.ImportNestedResponse, error) {
	selected := make([]domain.NestedRepoCandidate, 0, len(req.GetSelected()))
	for _, c := range req.GetSelected() {
		selected = append(selected, domain.NestedRepoCandidate{Path: c.GetPath(), SuggestedName: c.GetSuggestedName(), IsGitRepo: c.GetIsGitRepo()})
	}
	groups, projects, err := s.importNested.Execute(ctx, usecase.ImportNestedInput{
		ParentGroupID: req.GetParentGroupId(), Selected: selected,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	outGroups := make([]*projectv1.ProjectGroup, 0, len(groups))
	for _, g := range groups {
		outGroups = append(outGroups, toProtoProjectGroup(g))
	}
	outProjects := make([]*projectv1.Project, 0, len(projects))
	for _, p := range projects {
		outProjects = append(outProjects, toProtoProject(p))
	}
	return &projectv1.ImportNestedResponse{CreatedGroups: outGroups, CreatedProjects: outProjects}, nil
}
```

Also update `toProtoProjectGroup` (adds `ProjectId`):

```go
func toProtoProjectGroup(g domain.ProjectGroup) *projectv1.ProjectGroup {
	return &projectv1.ProjectGroup{
		Id:            g.ID,
		TenantId:      g.TenantID,
		Name:          g.Name,
		ParentGroupId: g.ParentGroupID,
		ProjectId:     g.ProjectID,
	}
}
```

Add `"github.com/stablyai/orca-go/common/tenant"` to this file's import
block for `MoveProject`'s `tenant.RequireTenantID` call.

### Step 10 — `internal/config/config.go`: add `InfraFleetServiceAddr`

```go
type Config struct {
	commonconfig.Base
	WorkflowServiceAddr string
	TaskServiceAddr     string
	// InfraFleetServiceAddr is ScanNested/ImportNested's (and SOL-022's
	// SetupExistingFolder's) DevServerRelay dependency.
	InfraFleetServiceAddr string
	OPABundlePath         string
}
```

```go
		WorkflowServiceAddr:   commonconfig.StringEnv("WORKFLOW_SERVICE_ADDR", "workflow-service:9090"),
		TaskServiceAddr:       commonconfig.StringEnv("TASK_SERVICE_ADDR", "task-service:9090"),
		InfraFleetServiceAddr: commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
		OPABundlePath:         commonconfig.StringEnv("OPA_BUNDLE_PATH", "../../policy/orca-authz"),
```

### Step 11 — `cmd/server/main.go`: wire the new client + usecases

Near `taskChecker, err := projectgrpcclient.NewTaskExecutionChecker(...)`, add:

```go
devServerRelay, err := projectgrpcclient.NewDevServerRelay(cfg.InfraFleetServiceAddr)
if err != nil {
	return fmt.Errorf("dialing infra-fleet-service: %w", err)
}
defer func() { _ = devServerRelay.Close() }()
```

Near `createProjectGroupUC := usecase.NewCreateProjectGroup(projectGroupRepo)`, add:

```go
moveProjectUC := usecase.NewMoveProject(repo, projectGroupRepo, opa)
scanNestedUC := usecase.NewScanNested(devServerRelay)
importNestedUC := usecase.NewImportNested(projectGroupRepo)
```

Add the 3 new fields to `projectgrpc.Deps{...}`, next to
`ListProjectGroups: listProjectGroupsUC,`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./... && go vet ./...
```

Expected: clean build. `cmd/server/main.go` build failure until Step 11 is
applied is expected mid-task.
