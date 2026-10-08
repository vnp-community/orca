package grpcclient

import (
	"context"
	"fmt"
	"time"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const projectMembersTimeout = 10 * time.Second

// ProjectMembershipClient answers "is user a member of project" from project-service. The member list is read
// as the asking user, so a project the user cannot read answers "not a member" (permission denied, not found).
type ProjectMembershipClient struct {
	client projectv1.ProjectServiceClient
}

var _ usecase.ProjectMembership = (*ProjectMembershipClient)(nil)

func NewProjectMembershipClient(c projectv1.ProjectServiceClient) *ProjectMembershipClient {
	return &ProjectMembershipClient{client: c}
}

func (c *ProjectMembershipClient) IsMember(ctx context.Context, projectID, userID string) (bool, error) {
	ctx, err := withIdentityMetadata(ctx)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, projectMembersTimeout)
	defer cancel()
	resp, err := c.client.ListMembers(ctx, &projectv1.ListMembersRequest{ProjectId: projectID})
	if err != nil {
		switch status.Code(err) {
		case codes.PermissionDenied, codes.NotFound:
			return false, nil
		}
		return false, fmt.Errorf("grpcclient: list project members: %w", err)
	}
	for _, m := range resp.GetMembers() {
		if m.GetUserId() == userID {
			return true, nil
		}
	}
	return false, nil
}

// NoProjectMembership is used when PROJECT_SERVICE_ADDR is unset: only the reporter and admins then see a Request.
type NoProjectMembership struct{}

func (NoProjectMembership) IsMember(context.Context, string, string) (bool, error) { return false, nil }
