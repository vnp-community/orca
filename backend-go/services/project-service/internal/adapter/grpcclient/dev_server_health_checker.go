package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetHealthChecker implements usecase.DevServerHealthChecker by
// calling infra-fleet-service's GetFleetHealth RPC — already exists on the
// proto (infrafleet.proto), just never called by project-service before
// this task. GetFleetHealthRequest only carries tenant_id (no
// dev_server_id field — confirmed against the generated stub, differs from
// this task's original guess), and GetFleetHealthResponse returns
// statuses for every dev server in the tenant, so IsReachable filters the
// response for the matching id.
type InfraFleetHealthChecker struct {
	client infrafleetv1.InfraFleetServiceClient
}

// NewInfraFleetHealthChecker wraps an already-dialed client — reuse the
// same *grpc.ClientConn InfraFleetDevServerLister/DevServerRelay dial in
// cmd/server/main.go rather than opening a second connection.
func NewInfraFleetHealthChecker(client infrafleetv1.InfraFleetServiceClient) *InfraFleetHealthChecker {
	return &InfraFleetHealthChecker{client: client}
}

func (c *InfraFleetHealthChecker) IsReachable(ctx context.Context, tenantID, devServerID string) (bool, error) {
	resp, err := c.client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{TenantId: tenantID})
	if err != nil {
		return false, fmt.Errorf("grpcclient: infra-fleet-service GetFleetHealth: %w", err)
	}
	for _, status := range resp.GetStatuses() {
		if status.GetDevServerId() == devServerID {
			return status.GetReachable(), nil
		}
	}
	// No status entry for this dev server id — treat as unreachable
	// (fail closed), same posture as an outright RPC error.
	return false, nil
}
