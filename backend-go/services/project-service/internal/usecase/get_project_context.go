package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// GetProjectContext is project-service.md §2's Boundary-decision RPC: a
// read-only project-context lookup workflow-service/task-service call to
// build an agent-spawn env/preamble — a two-step saga (resolve context
// here, then call the execution-owning service), not a synchronous
// cross-service execution call. Access control reuses the existing
// any-member gate unchanged — an execution-dispatch caller acts on behalf
// of an already-authenticated end user, and must present that user's
// membership, not a service-identity bypass.
type GetProjectContext struct {
	projects ProjectRepository
	repos    RepoRepository
	hosts    DevServerHostnameResolver
	opa      OPAClient
}

func NewGetProjectContext(projects ProjectRepository, repos RepoRepository, hosts DevServerHostnameResolver, opa OPAClient) *GetProjectContext {
	return &GetProjectContext{projects: projects, repos: repos, hosts: hosts, opa: opa}
}

func (uc *GetProjectContext) Execute(ctx context.Context, projectID string) (domain.ProjectContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ProjectContext{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireProjectAccess(ctx, uc.projects, uc.opa, projectID, projectActionAnyMember); err != nil {
		return domain.ProjectContext{}, err
	}

	project, err := uc.projects.Get(ctx, tenantID, projectID)
	if err != nil {
		return domain.ProjectContext{}, apperrors.New(apperrors.KindNotFound, "PROJECT_NOT_FOUND", "project does not exist", err)
	}

	repos, _ := uc.repos.ListRepos(ctx, projectID) // best-effort; a project with no repos yet has an empty RepoURL
	var repoURL string
	if len(repos) > 0 {
		repoURL = repos[0].URL
	}
	hostname, _ := uc.hosts.Hostname(ctx, tenantID, project.DevServerID) // "" on any failure — display-only field, never fails the read

	return domain.ProjectContext{
		ProjectID: project.ID, ProjectName: project.Name, Description: project.Description,
		RepoURL: repoURL, DevServerID: project.DevServerID, DevServerHostname: hostname,
	}, nil
}
