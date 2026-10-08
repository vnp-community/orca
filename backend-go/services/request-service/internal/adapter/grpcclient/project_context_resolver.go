package grpcclient

import (
	"context"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// ProjectContextResolver reads the project name and repo URL for the prompt. Callers treat any error as "no context".
type ProjectContextResolver struct {
	projects projectv1.ProjectServiceClient
}

var _ usecase.ProjectContextReader = (*ProjectContextResolver)(nil)

func NewProjectContextResolver(projects projectv1.ProjectServiceClient) *ProjectContextResolver {
	return &ProjectContextResolver{projects: projects}
}

func (r *ProjectContextResolver) Read(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return usecase.ProjectContext{}, err
	}
	resp, err := r.projects.GetProjectContext(pctx, &projectv1.GetProjectContextRequest{ProjectId: projectID})
	if err != nil {
		return usecase.ProjectContext{}, err
	}
	return usecase.ProjectContext{Name: resp.GetProjectName(), RepoURL: resp.GetRepoUrl()}, nil
}
