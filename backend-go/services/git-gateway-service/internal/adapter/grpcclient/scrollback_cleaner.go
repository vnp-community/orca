package grpcclient

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// ScrollbackCleaner implements usecase.ScrollbackCleaner against
// infra-fleet-service's DeleteTerminalScrollbackSnapshots RPC — shares the
// same *grpc.ClientConn as ConnectionResolver/RelayExecutor.
type ScrollbackCleaner struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewScrollbackCleaner(client infrafleetv1.InfraFleetServiceClient) *ScrollbackCleaner {
	return &ScrollbackCleaner{client: client}
}

func (c *ScrollbackCleaner) DeleteTerminalScrollbackSnapshots(ctx context.Context, worktreeID string) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = c.client.DeleteTerminalScrollbackSnapshots(ctx, &infrafleetv1.DeleteTerminalScrollbackSnapshotsRequest{WorktreeId: worktreeID})
	return err
}
