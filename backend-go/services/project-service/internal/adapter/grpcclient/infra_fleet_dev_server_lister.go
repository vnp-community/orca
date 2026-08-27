package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetDevServerLister implements usecase.DevServerLister by dialing
// infra-fleet-service's ListDevServers RPC — the only lookup available
// (no GetDevServer), same gap TASK-138's ScanNested already works around.
type InfraFleetDevServerLister struct {
	conn   *grpc.ClientConn
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetDevServerLister(addr string) (*InfraFleetDevServerLister, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return &InfraFleetDevServerLister{conn: conn, client: infrafleetv1.NewInfraFleetServiceClient(conn)}, nil
}

func (c *InfraFleetDevServerLister) Close() error {
	return c.conn.Close()
}

// Exists lists the caller's dev servers and checks membership —
// infra-fleet-service.ListDevServersRequest has no tenant_id field in the
// current generated stub, so tenant scoping relies on the RPC's own
// AttachIdentity-based outbound metadata rather than an explicit request
// field (see infrafleet.proto). The tenantID parameter is kept on this
// port's signature to match usecase.DevServerLister's contract even though
// it isn't threaded into the request message itself.
func (c *InfraFleetDevServerLister) Exists(ctx context.Context, tenantID, devServerID string) (bool, error) {
	resp, err := c.client.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{})
	if err != nil {
		return false, fmt.Errorf("grpcclient: infra-fleet-service ListDevServers: %w", err)
	}
	for _, ds := range resp.GetDevServers() {
		if ds.GetId() == devServerID {
			return true, nil
		}
	}
	return false, nil
}
