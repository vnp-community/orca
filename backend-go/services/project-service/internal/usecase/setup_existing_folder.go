package usecase

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

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
	project, err := domain.NewProject(uuid.NewString(), tenantID, displayName, "")
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

	// Reuses project.repos.url to carry the absolute on-disk path — same
	// simplification TASK-138's ImportNested applies.
	repo, err := domain.NewRepo(uuid.NewString(), created.ID, setup.FolderPath, displayName, setup.DevServerID)
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
