// Package projectclient implements usecase.ProjectClient against
// project-service's gRPC surface — used by ServerResolver to resolve
// TargetKindProject ("project:<id>") targets down to that project's bound
// dev server.
package projectclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// Dial opens an insecure gRPC connection to project-service — same pattern
// as infrafleetclient.Dial (this service's other outbound gRPC dependency).
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("projectclient: dial project-service at %q: %w", addr, err)
	}
	return conn, nil
}

// Client wraps projectv1.ProjectServiceClient.GetProject and unwraps the
// nested Project message correctly — GetProjectResponse.Project.DevServerId,
// NOT a top-level field on GetProjectResponse (BE-SOL-002's sketch called
// resp.GetDevServerId() directly, which does not compile; see
// TASK-WF-002-01's Context for the confirmed divergence).
type Client struct {
	client projectv1.ProjectServiceClient
}

func New(client projectv1.ProjectServiceClient) *Client {
	return &Client{client: client}
}

func (c *Client) GetProject(ctx context.Context, id string) (string, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return "", fmt.Errorf("projectclient: GetProject: %w", err)
	}
	resp, err := c.client.GetProject(ctx, &projectv1.GetProjectRequest{Id: id})
	if err != nil {
		return "", err
	}
	return resp.GetProject().GetDevServerId(), nil
}
