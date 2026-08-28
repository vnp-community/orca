package usecase

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type CreateProjectInput struct {
	Name string
	// Description/DefaultBranch/Visibility are all optional — empty
	// DefaultBranch/Visibility fall back to domain.DefaultBranch/
	// domain.DefaultVisibility, matching the DB column defaults.
	Description   string
	DefaultBranch string
	Visibility    string
	DevServerID   string // NEW — "" = create unbound, existing behavior preserved
	RepoPath      string // NEW — required iff DevServerID is set
}

// CreateProject is project-service's core write path. TenantID and CreatedBy
// are NOT part of the input struct — both are pulled from context (see
// common/tenant), never trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule. The creator
// becomes an implicit owner via a follow-up AddMember call by the caller
// (api-gateway) — see project-service.md §9; this usecase only creates the
// project row.
//
// When DevServerID is set, existence + health + repoPath checks all run
// BEFORE the project row is persisted, so a failure never leaves an
// orphaned bound-but-repo-less project — see this task's Context.
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
