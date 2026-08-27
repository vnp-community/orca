# TASK-143: Implement `project-service` usecase/repository/grpc layers for `projectHostSetup.*`

**From Solution:** SOL-022
**Priority:** P1
**Service:** `project-service`
**File:** `internal/usecase/ports.go`, `internal/usecase/create_host_setup.go` (new), `list_host_setups.go` (new), `update_host_setup.go` (new), `delete_host_setup.go` (new), `setup_existing_folder.go` (new), `internal/adapter/postgres/host_setup_repository.go` (new), `internal/adapter/grpc/server.go`, `internal/adapter/grpcclient/infra_fleet_dev_server_lister.go` (new), `cmd/server/main.go`
**Depends on:** TASK-141, TASK-142, TASK-138 (reuses its `DevServerRelay` port/`grpcclient.DevServerRelay`/`InfraFleetServiceAddr` config)
**Status:** `[x]` DONE — full usecase/repository/grpc/config wiring implemented and building clean. Worktree `agent-a9271c5b2d89347e7`, uncommitted. Postgres integration tests (`host_setup_repository_test.go` etc.) not written — no live Postgres in this environment; SQL verified consistent with schema conventions.

---

## Context

`CreateHostSetup`/`ListHostSetups`/`UpdateHostSetup`/`DeleteHostSetup` are
plain CRUD. `SetupExistingFolder` is the one real usecase: it must **never**
stat the path against `project-service`'s own filesystem — reuses the
identical `DevServerRelay` port TASK-138 already added for
`ScanNested`/`ImportNested`, calling `fs.checkPath` instead of
`fs.scanNestedRepos`.

**`dev_server_id` validation on create**: `infra-fleet-service` has no
`GetDevServer` RPC (only `ListDevServers`) — same gap TASK-138 already
worked around for `ScanNested`. `CreateHostSetup` validates by listing that
tenant's dev servers and checking membership, via a small new port
(`DevServerLister`) rather than inventing a new infra-fleet-service RPC.

**Open dependency, called out explicitly**: `fs.checkPath`'s JSON-RPC
shape (`{path} -> {exists, isDir, isGitRepo}`) is this task's proposal —
same caveat as TASK-138's `fs.scanNestedRepos`, needs Agent-side
confirmation.

## Changes to make

### Step 1 — `internal/usecase/ports.go`: add `HostSetupRepository`, `DevServerLister`

Append:

```go
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
```

### Step 2 — `internal/usecase/create_host_setup.go` (new)

```go
package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type CreateHostSetupInput struct {
	DevServerID string
	FolderPath  string
	DisplayName string
}

type CreateHostSetup struct {
	repo       HostSetupRepository
	devServers DevServerLister
}

func NewCreateHostSetup(repo HostSetupRepository, devServers DevServerLister) *CreateHostSetup {
	return &CreateHostSetup{repo: repo, devServers: devServers}
}

func (uc *CreateHostSetup) Execute(ctx context.Context, in CreateHostSetupInput) (domain.HostSetup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.HostSetup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	exists, err := uc.devServers.Exists(ctx, tenantID, in.DevServerID)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
	}
	if !exists {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
	}

	setup, err := domain.NewHostSetup(uuid.NewString(), tenantID, in.DevServerID, in.FolderPath, in.DisplayName, userID)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_HOST_SETUP", err.Error(), err)
	}

	created, err := uc.repo.Create(ctx, setup)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_HOST_SETUP_FAILED", "failed to persist host setup", err)
	}
	return created, nil
}
```

### Step 3 — `internal/usecase/list_host_setups.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type ListHostSetups struct {
	repo HostSetupRepository
}

func NewListHostSetups(repo HostSetupRepository) *ListHostSetups {
	return &ListHostSetups{repo: repo}
}

func (uc *ListHostSetups) Execute(ctx context.Context) ([]domain.HostSetup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	setups, err := uc.repo.List(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_HOST_SETUPS_FAILED", "failed to list host setups", err)
	}
	return setups, nil
}
```

### Step 4 — `internal/usecase/update_host_setup.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type UpdateHostSetupInput struct {
	ID    string
	Patch domain.HostSetupPatch
}

type UpdateHostSetup struct {
	repo HostSetupRepository
}

func NewUpdateHostSetup(repo HostSetupRepository) *UpdateHostSetup {
	return &UpdateHostSetup{repo: repo}
}

func (uc *UpdateHostSetup) Execute(ctx context.Context, in UpdateHostSetupInput) (domain.HostSetup, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.HostSetup{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	setup, err := uc.repo.Update(ctx, tenantID, in.ID, in.Patch)
	if err != nil {
		if errors.Is(err, domain.ErrHostSetupNotFound) {
			return domain.HostSetup{}, apperrors.New(apperrors.KindNotFound, "PROJECT_HOST_SETUP_NOT_FOUND", "host setup does not exist", err)
		}
		return domain.HostSetup{}, apperrors.New(apperrors.KindInternal, "PROJECT_UPDATE_HOST_SETUP_FAILED", "failed to update host setup", err)
	}
	return setup, nil
}
```

### Step 5 — `internal/usecase/delete_host_setup.go` (new)

```go
package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type DeleteHostSetupInput struct {
	ID string
}

type DeleteHostSetup struct {
	repo HostSetupRepository
}

func NewDeleteHostSetup(repo HostSetupRepository) *DeleteHostSetup {
	return &DeleteHostSetup{repo: repo}
}

func (uc *DeleteHostSetup) Execute(ctx context.Context, in DeleteHostSetupInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := uc.repo.Delete(ctx, tenantID, in.ID); err != nil {
		if errors.Is(err, domain.ErrHostSetupNotFound) {
			return apperrors.New(apperrors.KindNotFound, "PROJECT_HOST_SETUP_NOT_FOUND", "host setup does not exist", err)
		}
		return apperrors.New(apperrors.KindInternal, "PROJECT_DELETE_HOST_SETUP_FAILED", "failed to delete host setup", err)
	}
	return nil
}
```

### Step 6 — `internal/usecase/setup_existing_folder.go` (new)

```go
package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type SetupExistingFolderInput struct {
	ID string // the HostSetup being finalized
}

// pathCheckResult is fs.checkPath's expected Relay result shape — see this
// task's Context for the "open dependency, not yet Agent-confirmed" caveat.
type pathCheckResult struct {
	Exists bool `json:"exists"`
	IsDir  bool `json:"isDir"`
}

// SetupExistingFolder validates folder_path on the DEV SERVER, never
// locally — the exact "legacy desktop-app assumption" both BUG-021 and
// BUG-022 flag — then creates a real Project + Repo from it.
type SetupExistingFolder struct {
	repo     HostSetupRepository
	projects ProjectRepository
	repos    RepoRepository
	relay    DevServerRelay
}

func NewSetupExistingFolder(repo HostSetupRepository, projects ProjectRepository, repos RepoRepository, relay DevServerRelay) *SetupExistingFolder {
	return &SetupExistingFolder{repo: repo, projects: projects, repos: repos, relay: relay}
}

func (uc *SetupExistingFolder) Execute(ctx context.Context, in SetupExistingFolderInput) (domain.HostSetup, domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	setup, err := uc.repo.Get(ctx, tenantID, in.ID)
	if err != nil {
		if errors.Is(err, domain.ErrHostSetupNotFound) {
			return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindNotFound, "PROJECT_HOST_SETUP_NOT_FOUND", "host setup does not exist", err)
		}
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_HOST_SETUP_LOOKUP_FAILED", "failed to look up host setup", err)
	}
	if setup.Status == domain.HostSetupCompleted {
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_HOST_SETUP_ALREADY_COMPLETED", domain.ErrHostSetupAlreadyCompleted.Error(), domain.ErrHostSetupAlreadyCompleted)
	}

	connID, err := uc.relay.CreateConnection(ctx, setup.DevServerID, setup.FolderPath, "")
	if err != nil {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED", "failed to connect to dev server", err)
	}
	params, err := json.Marshal(map[string]string{"path": setup.FolderPath})
	if err != nil {
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_PARAMS_FAILED", "failed to encode check-path params", err)
	}
	resultJSON, err := uc.relay.Relay(ctx, connID, "fs.checkPath", params)
	if err != nil {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_FAILED", "failed to validate folder on dev server", err)
	}
	var check pathCheckResult
	if err := json.Unmarshal(resultJSON, &check); err != nil || !check.Exists || !check.IsDir {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_FOLDER_NOT_FOUND_ON_HOST", domain.ErrFolderNotFoundOnHost.Error(), domain.ErrFolderNotFoundOnHost)
	}

	displayName := setup.DisplayName
	if displayName == "" {
		displayName = setup.FolderPath
	}
	project, err := domain.NewProject(newUUID(), tenantID, displayName, "")
	if err != nil {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID", err.Error(), err)
	}
	project.DefaultBranch = domain.DefaultBranch
	project.Visibility = domain.DefaultVisibility
	project.CreatedBy = userID
	project.DevServerID = setup.DevServerID

	created, err := uc.projects.Create(ctx, project)
	if err != nil {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_FAILED", "failed to create project", err)
	}

	// Reuses project.repos.url to carry the absolute on-disk path — see
	// TASK-141's Context for why this task does not add a distinct `path`
	// field/column (same simplification TASK-138's ImportNested applies).
	repo, err := domain.NewRepo(newUUID(), created.ID, setup.FolderPath, displayName)
	if err != nil {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_REPO", err.Error(), err)
	}
	if _, err := uc.repos.AddRepo(ctx, repo); err != nil {
		_ = uc.repo.SetStatus(ctx, tenantID, in.ID, domain.HostSetupFailed)
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_ADD_REPO_FAILED", "failed to attach repo", err)
	}

	completed, err := uc.repo.Complete(ctx, tenantID, in.ID, created.ID)
	if err != nil {
		return domain.HostSetup{}, domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_COMPLETE_HOST_SETUP_FAILED", "failed to finalize host setup", err)
	}
	return completed, created, nil
}
```

`newUUID()` above is `uuid.NewString()` (import
`"github.com/google/uuid"`) — named `newUUID` only to keep both call sites
in this snippet short; use `uuid.NewString()` directly, matching
`create_project.go`'s existing convention (no local wrapper needed).

### Step 7 — `internal/adapter/postgres/host_setup_repository.go` (new)

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

const hostSetupColumns = `id, tenant_id, dev_server_id, folder_path, COALESCE(display_name, ''), status, COALESCE(project_id::text, ''), created_by`

type HostSetupRepository struct {
	pool *pgxpool.Pool
}

func NewHostSetupRepository(pool *pgxpool.Pool) *HostSetupRepository {
	return &HostSetupRepository{pool: pool}
}

func (r *HostSetupRepository) Create(ctx context.Context, s domain.HostSetup) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO project.project_host_setups (id, tenant_id, dev_server_id, folder_path, display_name, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+hostSetupColumns,
		s.ID, s.TenantID, s.DevServerID, s.FolderPath, nullableString(s.DisplayName), string(s.Status), s.CreatedBy,
	)
	out, err := scanHostSetup(row)
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: insert host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) Get(ctx context.Context, tenantID, id string) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+hostSetupColumns+`
		FROM project.project_host_setups
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	out, err := scanHostSetup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: query host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) List(ctx context.Context, tenantID string) ([]domain.HostSetup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+hostSetupColumns+`
		FROM project.project_host_setups
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query host setups: %w", err)
	}
	defer rows.Close()

	var out []domain.HostSetup
	for rows.Next() {
		s, err := scanHostSetup(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: scan host setup row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *HostSetupRepository) Update(ctx context.Context, tenantID, id string, patch domain.HostSetupPatch) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.project_host_setups
		SET folder_path  = COALESCE(NULLIF($3, ''), folder_path),
		    display_name = COALESCE(NULLIF($4, ''), display_name),
		    updated_at   = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+hostSetupColumns,
		tenantID, id, patch.FolderPath, patch.DisplayName,
	)
	out, err := scanHostSetup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: update host setup: %w", err)
	}
	return out, nil
}

func (r *HostSetupRepository) Delete(ctx context.Context, tenantID, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM project.project_host_setups WHERE tenant_id = $1 AND id = $2
	`, tenantID, id)
	if err != nil {
		return fmt.Errorf("postgres: delete host setup: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrHostSetupNotFound
	}
	return nil
}

func (r *HostSetupRepository) SetStatus(ctx context.Context, tenantID, id string, status domain.HostSetupStatus) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE project.project_host_setups SET status = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, id, string(status))
	if err != nil {
		return fmt.Errorf("postgres: set host setup status: %w", err)
	}
	return nil
}

func (r *HostSetupRepository) Complete(ctx context.Context, tenantID, id, projectID string) (domain.HostSetup, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE project.project_host_setups
		SET status = $3, project_id = $4, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
		RETURNING `+hostSetupColumns,
		tenantID, id, string(domain.HostSetupCompleted), projectID,
	)
	out, err := scanHostSetup(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.HostSetup{}, domain.ErrHostSetupNotFound
	}
	if err != nil {
		return domain.HostSetup{}, fmt.Errorf("postgres: complete host setup: %w", err)
	}
	return out, nil
}

func scanHostSetup(row rowScanner) (domain.HostSetup, error) {
	var s domain.HostSetup
	var status string
	if err := row.Scan(&s.ID, &s.TenantID, &s.DevServerID, &s.FolderPath, &s.DisplayName, &status, &s.ProjectID, &s.CreatedBy); err != nil {
		return domain.HostSetup{}, err
	}
	s.Status = domain.HostSetupStatus(status)
	return s, nil
}
```

### Step 8 — `internal/adapter/grpcclient/infra_fleet_dev_server_lister.go` (new)

```go
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetDevServerLister implements usecase.DevServerLister by dialing
// infra-fleet-service's ListDevServers RPC — the only lookup available
// (no GetDevServer), same gap TASK-138's ScanNested already works around.
type InfraFleetDevServerLister struct {
	conn   *grpc.ClientConn
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetDevServerLister(addr string) (*InfraFleetDevServerLister, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return &InfraFleetDevServerLister{conn: conn, client: infrafleetv1.NewInfraFleetServiceClient(conn)}, nil
}

func (c *InfraFleetDevServerLister) Close() error {
	return c.conn.Close()
}

func (c *InfraFleetDevServerLister) Exists(ctx context.Context, tenantID, devServerID string) (bool, error) {
	resp, err := c.client.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{TenantId: tenantID})
	if err != nil {
		return false, fmt.Errorf("grpcclient: infra-fleet-service ListDevServers: %w", err)
	}
	for _, ds := range resp.GetDevServers() {
		if ds.GetId() == devServerID {
			return true, nil
		}
	}
	return false, nil
}
```

(If `ListDevServersRequest` has no `tenant_id` field in the current
generated stub, drop that field from the call and confirm
`infra-fleet-service`'s `ListDevServers` implementation already scopes by
the caller's tenant via `AttachIdentity`'s outbound metadata instead —
check `infrafleet.proto` before wiring.)

### Step 9 — `internal/adapter/grpc/server.go`: register the 5 new RPC handlers

Add 5 fields to `Server`/`Deps`/`New`, mechanically identical to the
`ProjectGroup` fields' wiring:

```go
func (s *Server) CreateHostSetup(ctx context.Context, req *projectv1.CreateHostSetupRequest) (*projectv1.CreateHostSetupResponse, error) {
	setup, err := s.createHostSetup.Execute(ctx, usecase.CreateHostSetupInput{
		DevServerID: req.GetDevServerId(), FolderPath: req.GetFolderPath(), DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.CreateHostSetupResponse{Setup: toProtoHostSetup(setup)}, nil
}

func (s *Server) ListHostSetups(ctx context.Context, _ *projectv1.ListHostSetupsRequest) (*projectv1.ListHostSetupsResponse, error) {
	setups, err := s.listHostSetups.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.HostSetup, 0, len(setups))
	for _, s := range setups {
		out = append(out, toProtoHostSetup(s))
	}
	return &projectv1.ListHostSetupsResponse{Setups: out}, nil
}

func (s *Server) UpdateHostSetup(ctx context.Context, req *projectv1.UpdateHostSetupRequest) (*projectv1.UpdateHostSetupResponse, error) {
	setup, err := s.updateHostSetup.Execute(ctx, usecase.UpdateHostSetupInput{
		ID: req.GetId(),
		Patch: domain.HostSetupPatch{FolderPath: req.GetFolderPath(), DisplayName: req.GetDisplayName()},
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.UpdateHostSetupResponse{Setup: toProtoHostSetup(setup)}, nil
}

func (s *Server) DeleteHostSetup(ctx context.Context, req *projectv1.DeleteHostSetupRequest) (*projectv1.DeleteHostSetupResponse, error) {
	if err := s.deleteHostSetup.Execute(ctx, usecase.DeleteHostSetupInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &projectv1.DeleteHostSetupResponse{}, nil
}

func (s *Server) SetupExistingFolder(ctx context.Context, req *projectv1.SetupExistingFolderRequest) (*projectv1.SetupExistingFolderResponse, error) {
	setup, project, err := s.setupExistingFolder.Execute(ctx, usecase.SetupExistingFolderInput{ID: req.GetId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &projectv1.SetupExistingFolderResponse{Setup: toProtoHostSetup(setup)}
	if setup.Status == domain.HostSetupCompleted {
		resp.Project = toProtoProject(project)
	}
	return resp, nil
}

func toProtoHostSetup(s domain.HostSetup) *projectv1.HostSetup {
	return &projectv1.HostSetup{
		Id: s.ID, TenantId: s.TenantID, DevServerId: s.DevServerID, FolderPath: s.FolderPath,
		DisplayName: s.DisplayName, Status: string(s.Status), ProjectId: s.ProjectID,
	}
}
```

### Step 10 — `cmd/server/main.go`: wire the new client + usecases

Near `devServerRelay, err := projectgrpcclient.NewDevServerRelay(...)`
(TASK-138), add:

```go
devServerLister, err := projectgrpcclient.NewInfraFleetDevServerLister(cfg.InfraFleetServiceAddr)
if err != nil {
	return fmt.Errorf("dialing infra-fleet-service (dev server lister): %w", err)
}
defer func() { _ = devServerLister.Close() }()
```

Near `projectGroupRepo := projectpostgres.NewProjectGroupRepository(pool)`,
add:

```go
hostSetupRepo := projectpostgres.NewHostSetupRepository(pool)
```

Near `importNestedUC := usecase.NewImportNested(projectGroupRepo)`, add:

```go
createHostSetupUC := usecase.NewCreateHostSetup(hostSetupRepo, devServerLister)
listHostSetupsUC := usecase.NewListHostSetups(hostSetupRepo)
updateHostSetupUC := usecase.NewUpdateHostSetup(hostSetupRepo)
deleteHostSetupUC := usecase.NewDeleteHostSetup(hostSetupRepo)
setupExistingFolderUC := usecase.NewSetupExistingFolder(hostSetupRepo, repo, repoRepo, devServerRelay)
```

Add the 5 new fields to `projectgrpc.Deps{...}`.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go build ./... && go vet ./...
```

Expected: clean build. `cmd/server/main.go` build failure until Step 10 is
applied is expected mid-task.
