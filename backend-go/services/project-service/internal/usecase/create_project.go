package usecase

import (
	"context"
	"encoding/json"
	"strings"

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
	DevServerID   string // "" = create unbound, existing behavior preserved
	RepoPath      string // required iff DevServerID is set
}

// CreateProject is project-service's core write path. TenantID and CreatedBy
// are NOT part of the input struct — both are pulled from context (see
// common/tenant), never trusted from the request body, per
// architecture/05-data-architecture.md's tenant-isolation rule.
//
// The project, its creator's owner membership and its first repo are written
// in ONE transaction (ProjectRepository.CreateWithOwner): a repo that cannot
// be stored never leaves a repo-less project behind, and the creator can
// always open the project they just made. Every check that can fail (input,
// dev server, repo location, repo path on the host) runs before that write.
type CreateProject struct {
	repo       ProjectRepository
	devServers DevServerLister
	health     DevServerHealthChecker
	relay      DevServerRelay // SetupExistingFolder's fs.checkPath pattern
	policy     RepoHostPolicy
	guard      repoLocationGuard
}

func NewCreateProject(repo ProjectRepository, repos RepoRepository, devServers DevServerLister, health DevServerHealthChecker, relay DevServerRelay, policy RepoHostPolicy) *CreateProject {
	return &CreateProject{repo: repo, devServers: devServers, health: health, relay: relay, policy: policy, guard: repoLocationGuard{repos: repos, members: repo}}
}

// Execute returns the new project and its first repo (nil when none was asked for).
func (uc *CreateProject) Execute(ctx context.Context, in CreateProjectInput) (domain.Project, *domain.Repo, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Project{}, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	userID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.Project{}, nil, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_USER", "no user in request context", nil)
	}

	devServerID := in.DevServerID
	repoPath := strings.TrimSpace(in.RepoPath)

	// Cheap input checks first: no network call for a request that cannot succeed.
	project, err := domain.NewProject(uuid.NewString(), tenantID, in.Name, devServerID)
	if err != nil {
		return domain.Project{}, nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID", err.Error(), err)
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
		return domain.Project{}, nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_INVALID_VISIBILITY", domain.ErrInvalidVisibility.Error(), domain.ErrInvalidVisibility)
	}
	project.Description = in.Description
	project.DefaultBranch = defaultBranch
	project.Visibility = visibility
	project.CreatedBy = userID

	// A repo_path with no dev server used to be dropped silently, leaving a
	// repo-less project; reject it instead so the caller picks a host.
	if repoPath != "" {
		if err := uc.policy.requireHost(devServerID); err != nil {
			return domain.Project{}, nil, err
		}
	}
	if devServerID != "" {
		exists, err := uc.devServers.Exists(ctx, tenantID, devServerID)
		if err != nil {
			return domain.Project{}, nil, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
		}
		if !exists {
			return domain.Project{}, nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
		}
		reachable, err := uc.health.IsReachable(ctx, tenantID, devServerID)
		if err != nil {
			// Fail closed — a health-check outage must never silently bind to
			// a server that might be down.
			return domain.Project{}, nil, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_HEALTH_CHECK_FAILED", "failed to verify dev server is online, failing closed", err)
		}
		if !reachable {
			return domain.Project{}, nil, apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_UNREACHABLE", "dev server is not online", nil)
		}
		if repoPath == "" {
			return domain.Project{}, nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_PATH_REQUIRED", "repo_path is required when dev_server_id is set", nil)
		}
	}

	var repo *domain.Repo
	if repoPath != "" {
		// Before the relay round trip: a taken location needs no host check.
		if err := uc.guard.ensureFree(ctx, tenantID, devServerID, repoPath, ""); err != nil {
			return domain.Project{}, nil, err
		}
		if devServerID != "" {
			if err := uc.requireRepoPathOnHost(ctx, devServerID, repoPath); err != nil {
				return domain.Project{}, nil, err
			}
		}
		r, err := domain.NewRepo(uuid.NewString(), project.ID, repoPath, in.Name, devServerID)
		if err != nil {
			return domain.Project{}, nil, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_INVALID", err.Error(), err)
		}
		r.RegistrationKey = domain.RepoRegistrationKey(tenantID, devServerID, repoPath)
		repo = &r
	}

	owner, err := domain.NewProjectMember(project.ID, userID, domain.ProjectRoleOwner)
	if err != nil {
		return domain.Project{}, nil, apperrors.New(apperrors.KindInternal, "PROJECT_OWNER_INVALID", err.Error(), err)
	}
	created, createdRepo, err := uc.repo.CreateWithOwner(ctx, project, owner, repo)
	if raced := uc.guard.translateWriteError(ctx, err, tenantID, devServerID, repoPath, ""); raced != nil {
		return domain.Project{}, nil, raced
	}
	if err != nil {
		return domain.Project{}, nil, apperrors.New(apperrors.KindInternal, "PROJECT_CREATE_FAILED", "failed to persist project", err)
	}
	return created, createdRepo, nil
}

// requireRepoPathOnHost reuses SetupExistingFolder's CreateConnection +
// fs.checkPath pattern: the folder must exist on the host before any row is written.
func (uc *CreateProject) requireRepoPathOnHost(ctx context.Context, devServerID, repoPath string) error {
	connID, err := uc.relay.CreateConnection(ctx, devServerID, repoPath, "")
	if err != nil {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED", "failed to connect to dev server", err)
	}
	params, err := json.Marshal(map[string]string{"path": repoPath})
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_PARAMS_FAILED", "failed to encode check-path params", err)
	}
	resultJSON, err := uc.relay.Relay(ctx, connID, "fs.checkPath", params)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "PROJECT_CHECK_PATH_FAILED", "failed to validate repo path on dev server", err)
	}
	var check struct {
		Exists bool `json:"exists"`
		IsDir  bool `json:"isDir"`
	}
	if err := json.Unmarshal(resultJSON, &check); err != nil || !check.Exists || !check.IsDir {
		return apperrors.New(apperrors.KindFailedPrecondition, "PROJECT_REPO_PATH_NOT_FOUND", "repo_path does not exist on dev server", nil)
	}
	return nil
}
