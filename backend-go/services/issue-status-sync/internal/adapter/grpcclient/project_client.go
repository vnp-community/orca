package grpcclient

import (
	"context"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// ProjectClient implements usecase.ProjectSettingsClient against
// project-service's GetProject RPC.
type ProjectClient struct {
	client projectv1.ProjectServiceClient
}

func NewProjectClient(client projectv1.ProjectServiceClient) *ProjectClient {
	return &ProjectClient{client: client}
}

func (c *ProjectClient) IsIssueStatusSyncEnabled(ctx context.Context, tenantID, projectID string) (bool, error) {
	ctx = withTenantMetadata(ctx, tenantID)
	resp, err := c.client.GetProject(ctx, &projectv1.GetProjectRequest{Id: projectID})
	if err != nil {
		return false, err
	}
	return resp.GetProject().GetIssueStatusSyncEnabled(), nil
}
