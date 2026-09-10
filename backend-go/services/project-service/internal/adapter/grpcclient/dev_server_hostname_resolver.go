package grpcclient

import (
	"context"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetHostnameResolver implements usecase.DevServerHostnameResolver —
// a thin wrapper over infra-fleet-service's ListDevServers RPC, same dial
// pattern as InfraFleetDevServerLister. Used only by GetProjectContext's
// display-only dev_server_hostname field — a lookup failure never fails
// the whole read (see IsReachable's sibling, Hostname, below).
type InfraFleetHostnameResolver struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetHostnameResolver(client infrafleetv1.InfraFleetServiceClient) *InfraFleetHostnameResolver {
	return &InfraFleetHostnameResolver{client: client}
}

func (c *InfraFleetHostnameResolver) Hostname(ctx context.Context, tenantID, devServerID string) (string, error) {
	resp, err := c.client.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{})
	if err != nil {
		return "", fmt.Errorf("grpcclient: infra-fleet-service ListDevServers: %w", err)
	}
	for _, ds := range resp.GetDevServers() {
		if ds.GetId() == devServerID {
			return ds.GetHost(), nil
		}
	}
	return "", nil
}
