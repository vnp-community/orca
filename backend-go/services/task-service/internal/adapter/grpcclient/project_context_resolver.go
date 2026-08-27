// This file implements usecase.ProjectContextResolver against
// project-service — a genuine scope addition (a new task-service ->
// project-service dependency edge, absent from
// 02-microservices-decomposition.md's dependency graph), flagged explicitly
// per TASK-TG-02-04's Context section, same posture as TASK-TG-02-03's
// git-gateway-service dependency.
package grpcclient

import (
	"context"
	"fmt"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// ProjectContextResolver implements usecase.ProjectContextResolver by
// calling project-service's GetProject (name) and ListRepos (repo URL —
// the first repo by position, since AIDecompose's prompt only has room for
// one "primary" repo URL; a project with zero repos just gets an empty
// repoURL, not an error).
type ProjectContextResolver struct {
	client projectv1.ProjectServiceClient
}

func NewProjectContextResolver(client projectv1.ProjectServiceClient) *ProjectContextResolver {
	return &ProjectContextResolver{client: client}
}

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
