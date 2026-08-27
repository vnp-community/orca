# TASK-PRF-03-04: Bind `CreateProject` to a dev server with health + repoPath validation

**From Solution:** SOL-PRF-03
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/usecase/create_project.go`
**Depends on:** TASK-PRF-03-01, TASK-PRF-03-02, TASK-PRF-03-03
**Status:** `[ ]` TODO

---

## Context

`CreateProject.Execute` today never accepts a `dev_server_id` — a project
always starts unbound, contradicting BL-PRF-03's creation flow which binds at
create time. This task adds optional binding: existence + health + repoPath
checks all run **before** the project row is persisted, so a failure never
leaves an orphaned bound-but-repo-less project (correctness requirement
flagged explicitly in SOL-PRF-03 — do not persist the project row first and
check after, as the SOL's own initial sketch did before catching itself).

## Changes to make

In `backend-go/services/project-service/internal/usecase/create_project.go`:

```go
type CreateProjectInput struct {
	Name          string
	Description   string
	DefaultBranch string
	Visibility    string
	DevServerID   string // NEW — "" = create unbound, existing behavior preserved
	RepoPath      string // NEW — required iff DevServerID is set
}

type CreateProject struct {
	repo       ProjectRepository
	repos      RepoRepository         // NEW
	devServers DevServerLister        // NEW — existing port, reused
	health     DevServerHealthChecker // NEW
	relay      DevServerRelay         // NEW — existing port, reused (SetupExistingFolder's pattern)
}

func NewCreateProject(repo ProjectRepository, repos RepoRepository, devServers DevServerLister, health DevServerHealthChecker, relay DevServerRelay) *CreateProject {
	return &CreateProject{repo: repo, repos: repos, devServers: devServers, health: health, relay: relay}
}

func (uc *CreateProject) Execute(ctx context.Context, in CreateProjectInput) (domain.Project, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Project{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	devServerID := in.DevServerID
	var repoPathValid bool
	if devServerID != "" {
		exists, err := uc.devServers.Exists(ctx, tenantID, devServerID)
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
		}
		if !exists {
			return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
		}
		reachable, err := uc.health.IsReachable(ctx, tenantID, devServerID)
		if err != nil {
			// Fail closed — a health-check outage must never silently bind to
			// a server that might be down.
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED", "failed to verify dev server is online, failing closed", err)
		}
		if !reachable {
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE", "dev server is not online", nil)
		}
		if in.RepoPath == "" {
			return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_PATH_REQUIRED", "repo_path is required when dev_server_id is set", nil)
		}

		// repoPath existence check runs BEFORE the project row is created —
		// SetupExistingFolder's exact CreateConnection/fs.checkPath pattern
		// (setup_existing_folder.go), reused not reinvented. A repoPath
		// failure here must never leave an orphaned project row.
		connID, err := uc.relay.CreateConnection(ctx, devServerID, in.RepoPath, "")
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED", "failed to connect to dev server", err)
		}
		params, err := json.Marshal(map[string]string{"path": in.RepoPath})
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_PARAMS_FAILED", "failed to encode check-path params", err)
		}
		resultJSON, err := uc.relay.Relay(ctx, connID, "fs.checkPath", params)
		if err != nil {
			return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_FAILED", "failed to validate repo path on dev server", err)
		}
		var check struct {
			Exists bool `json:"exists"`
			IsDir  bool `json:"isDir"`
		}
		if err := json.Unmarshal(resultJSON, &check); err != nil || !check.Exists || !check.IsDir {
			return domain.Project{}, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_REPO_PATH_NOT_FOUND", "repo_path does not exist on dev server", nil)
		}
		repoPathValid = true
	}

	project, err := domain.NewProject(uuid.NewString(), tenantID, in.Name, devServerID)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID", err.Error(), err)
	}

	defaultBranch := in.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = domain.DefaultBranch
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = domain.DefaultVisibility
	}
	if !domain.ValidVisibility(visibility) {
		return domain.Project{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY", domain.ErrInvalidVisibility.Error(), domain.ErrInvalidVisibility)
	}

	project.Description = in.Description
	project.DefaultBranch = defaultBranch
	project.Visibility = visibility
	project.CreatedBy = userID

	created, err := uc.repo.Create(ctx, project)
	if err != nil {
		return domain.Project{}, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_FAILED", "failed to persist project", err)
	}

	if repoPathValid {
		repo, err := domain.NewRepo(uuid.NewString(), created.ID, in.RepoPath, in.Name)
		if err == nil {
			_, _ = uc.repos.AddRepo(ctx, repo) // best-effort attach — the project itself is already valid and persisted
		}
	}
	return created, nil
}
```

Add `"encoding/json"` to imports. Confirm `domain.NewRepo`'s real signature
(`(id, projectID, url, displayName string) (domain.Repo, error)` is this
task's best guess from `RepoRepository.AddRepo`'s doc comment in
`ports.go`) against `internal/domain/repo.go` before wiring — adjust
argument order/names if it differs.

Update the `NewCreateProject(repo)` call site in `cmd/server/main.go` — done
in TASK-PRF-03-07 alongside the gRPC handler wiring.

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/project-service/internal/usecase/create_project.go
```

Add test cases per SOL-PRF-03's Test plan: `DevServerID` empty -> unchanged
existing behavior (no relay/health calls at all); `DevServerID` set +
`RepoPath` empty -> `KindInvalidArgument` before any repo/health call; fake
`DevServerLister.Exists` false -> `KindInvalidArgument`, no health/relay
call; fake `DevServerHealthChecker` `reachable=false` ->
`KindFailedPrecondition`, no relay call; fake `DevServerRelay` returning
`{exists:false}` -> `KindFailedPrecondition`, and no orphaned project row in
the fake repository (assert `repo.Create` was never called, or if it must be
called for `RepoPath` reasons, assert the test fake never persisted a row
without its repo attached — see this task's ordering note above). Full
build/test lands with TASK-PRF-03-07.
