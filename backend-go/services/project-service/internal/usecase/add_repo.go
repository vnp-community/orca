package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

type AddRepoInput struct {
	ProjectID   string
	URL         string
	DisplayName string
	// DevServerID is this repo's own dev-server binding (Phase 10). Empty
	// falls back to the project's dev server, then to RepoHostPolicy
	// (PROJECT_REPO_HOST_REQUIRED unless local repos are allowed). Always
	// validated against infra-fleet, never trusted as caller-supplied.
	DevServerID string
}

// AddRepo appends a new repo to a project's catalog, at the next available
// position — see RepoRepository.AddRepo's doc comment.
//
// Authorization is a judgment call: AddRepo isn't named in
// project-service.md §9's matrix, but a repo belongs to exactly one
// project, so the project's owner-or-admin rule (the matrix's owner-only
// tier) is the natural fit for mutating that project's repo catalog. See
// this service's README "Known gaps" for the RPCs this same judgment call
// was NOT extended to.
type AddRepo struct {
	repo       RepoRepository
	projects   ProjectDirectory
	opa        OPAClient
	devServers DevServerLister
	policy     RepoHostPolicy
}

func NewAddRepo(repo RepoRepository, projects ProjectDirectory, opa OPAClient, devServers DevServerLister, policy RepoHostPolicy) *AddRepo {
	return &AddRepo{repo: repo, projects: projects, opa: opa, devServers: devServers, policy: policy}
}

func (uc *AddRepo) Execute(ctx context.Context, in AddRepoInput) (domain.Repo, error) {
	// Every usecase requires authenticated tenant context uniformly, even
	// though the repo's tenant scoping is transitive through its project_id
	// FK rather than a tenant_id column of its own — same convention as
	// AddMember.
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.projects, uc.opa, in.ProjectID, projectActionOwnerOnly); err != nil {
		return domain.Repo{}, err
	}

	// Request wins, then the project's own dev server: a repo added to a bound
	// project must live on that host, so the client need not repeat it.
	devServerID := in.DevServerID
	if devServerID == "" {
		project, err := uc.projects.Get(ctx, tenantID, in.ProjectID)
		if err != nil {
			if errors.Is(err, domain.ErrProjectNotFound) {
				return domain.Repo{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project not found", err)
			}
			return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_FETCH_FAILED", "failed to fetch project", err)
		}
		devServerID = project.DevServerID
	}
	if err := uc.policy.requireHost(devServerID); err != nil {
		return domain.Repo{}, err
	}
	if devServerID != "" {
		exists, err := uc.devServers.Exists(ctx, tenantID, devServerID)
		if err != nil {
			return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_DEV_SERVER_LOOKUP_FAILED", "failed to validate dev server", err)
		}
		if !exists {
			return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_DEV_SERVER_NOT_FOUND", "dev server does not exist", nil)
		}
	}

	r, err := domain.NewRepo(uuid.NewString(), in.ProjectID, in.URL, in.DisplayName, devServerID)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInvalidArgument, "PROJECT_REPO_INVALID", err.Error(), err)
	}

	created, err := uc.repo.AddRepo(ctx, r)
	if err != nil {
		return domain.Repo{}, apperrors.New(apperrors.KindInternal, "PROJECT_ADD_REPO_FAILED", "failed to persist repo", err)
	}
	return created, nil
}
