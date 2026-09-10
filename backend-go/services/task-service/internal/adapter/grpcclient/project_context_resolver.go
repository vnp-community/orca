// This file implements two usecase ports against the same project-service
// client, both a genuine task-service -> project-service dependency edge
// (absent from 02-microservices-decomposition.md's dependency graph, flagged
// per TASK-TG-02-04's Context section, same posture as TASK-TG-02-03's
// git-gateway-service dependency):
//   - usecase.ProjectContextResolver (TASK-PRF-04-01/02): GetProjectContext,
//     dialed by SimpleExecutor for the profile-aware context preamble.
//   - usecase.ProjectInfoResolver (TASK-TG-02-04): Resolve, dialed by
//     AIDecompose for its context bundle's name/repo-URL pair.
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// ProjectContextResolver implements both usecase.ProjectContextResolver and
// usecase.ProjectInfoResolver by dialing project-service's ProjectService
// client — GetProjectContext directly, and Resolve via GetProject + ListRepos.
type ProjectContextResolver struct {
	client projectv1.ProjectServiceClient
}

func NewProjectContextResolver(client projectv1.ProjectServiceClient) *ProjectContextResolver {
	return &ProjectContextResolver{client: client}
}

// GetProjectContext is membership-gated on the server side
// (project-service's requireProjectAccess, projectActionAnyMember) — it
// needs BOTH tenant AND acting-user identity forwarded as outbound
// metadata, not just tenant (see withTenantMetadata's doc comment, scoped to
// infra-fleet-service calls only). The caller (SimpleExecutor) is expected
// to have already set tenant.WithUserID(ctx, <target user>) before reaching
// this resolver — see TASK-PRF-04-08.
func (r *ProjectContextResolver) GetProjectContext(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return usecase.ProjectContext{}, fmt.Errorf("grpcclient: project context resolver: %w", err)
	}
	userID, _ := tenant.UserID(ctx) // absent -> project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome
	outCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)

	resp, err := r.client.GetProjectContext(outCtx, &projectv1.GetProjectContextRequest{ProjectId: projectID})
	if err != nil {
		return usecase.ProjectContext{}, fmt.Errorf("grpcclient: project-service GetProjectContext: %w", err)
	}
	return usecase.ProjectContext{
		ProjectID: resp.GetProjectId(), ProjectName: resp.GetProjectName(), Description: resp.GetDescription(),
		RepoURL: resp.GetRepoUrl(), DevServerID: resp.GetDevServerId(), DevServerHostname: resp.GetDevServerHostname(),
	}, nil
}

// Resolve calls project-service's GetProject (name) and ListRepos (repo URL
// — the first repo by position, since AIDecompose's prompt only has room for
// one "primary" repo URL; a project with zero repos just gets an empty
// repoURL, not an error).
func (r *ProjectContextResolver) Resolve(ctx context.Context, tenantID, projectID string) (name, repoURL string, err error) {
	ctx, err = withTenantMetadata(ctx)
	if err != nil {
		return "", "", err
	}
	projResp, err := r.client.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
	if err != nil {
		return "", "", fmt.Errorf("grpcclient: GetProject(%q): %w", projectID, err)
	}
	name = projResp.GetProject().GetName()

	reposResp, err := r.client.ListRepos(ctx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		// Best-effort for the repo URL half only — a project name without a
		// repo URL is still useful prompt context, so this degrades rather
		// than failing the whole resolve.
		return name, "", nil
	}
	if repos := reposResp.GetRepos(); len(repos) > 0 {
		repoURL = repos[0].GetUrl()
	}
	return name, repoURL, nil
}
