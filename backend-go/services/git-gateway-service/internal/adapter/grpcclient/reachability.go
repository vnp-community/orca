package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// DevServerReachability implements usecase.DevServerReachability by reading
// infra-fleet-service's GetFleetHealth and checking the sample for
// devServerID — see usecase/ports.go's doc comment for why this port
// exists instead of a worktree-keyed ConnectionResolver lookup.
type DevServerReachability struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewDevServerReachability(client infrafleetv1.InfraFleetServiceClient) *DevServerReachability {
	return &DevServerReachability{client: client}
}

func (d *DevServerReachability) IsReachable(ctx context.Context, devServerID string) (bool, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return false, err
	}
	resp, err := d.client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{})
	if err != nil {
		return false, fmt.Errorf("grpcclient: GetFleetHealth: %w", err)
	}
	for _, h := range resp.GetStatuses() {
		if h.GetDevServerId() == devServerID {
			return h.GetReachable(), nil
		}
	}
	return false, nil // no sample yet for this dev server — treat as not reachable, not an error
}
