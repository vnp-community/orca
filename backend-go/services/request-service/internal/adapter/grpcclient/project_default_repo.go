package grpcclient

import (
	"context"
	"fmt"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// defaultRepoID is the project's first repo (lowest position): the repo tasks execute on, so the proposal worktree and the base
// ref check look at the same repository the code will be written in.
func defaultRepoID(ctx context.Context, projects projectv1.ProjectServiceClient, projectID string) (string, error) {
	pctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return "", err
	}
	resp, err := projects.ListRepos(pctx, &projectv1.ListReposRequest{ProjectId: projectID})
	if err != nil {
		return "", mapWorkspaceError("list the project's repos", err)
	}
	if len(resp.GetRepos()) == 0 {
		return "", fmt.Errorf("%w: project %s has no repo", domain.ErrEngineRepoNotResolved, projectID)
	}
	return resp.GetRepos()[0].GetId(), nil
}
