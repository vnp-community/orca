package wscompat

import (
	"context"
	"errors"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// WorktreeTargetResolver maps a worktree id to where its terminal must run: the
// infra-fleet connection (local, SSH or dev server alike) and the checkout path
// on that host. It is the server-side twin of the frontend's getConnectionId,
// so MCP terminals reach the same host as UI terminals and never assume local.
type WorktreeTargetResolver struct {
	Project projectv1.ProjectServiceClient
	Infra   infrafleetv1.InfraFleetServiceClient
}

// ResolveWorktreeTarget returns the connection id and path of worktreeID.
func (r WorktreeTargetResolver) ResolveWorktreeTarget(ctx context.Context, id Identity, worktreeID string) (connectionID, cwd string, err error) {
	if r.Project == nil || r.Infra == nil {
		return "", "", errors.New("worktree resolution is not configured")
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	wt, err := r.Project.GetWorktree(ctx, &projectv1.GetWorktreeRequest{WorktreeId: worktreeID})
	if err != nil {
		return "", "", err
	}
	connectionID, err = resolveConnectionIDForWorktree(ctx, r.Infra, worktreeID)
	if err != nil {
		return "", "", err
	}
	return connectionID, wt.GetPath(), nil
}
