package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// DevServerRelay implements usecase.DevServerRelay by dialing
// infra-fleet-service's real CreateConnection/Relay RPCs — the same
// already-generic connectionId+method+params primitives
// devServer.*/fleet.* wscompat channels use (infrafleet.proto:31's doc
// comment). See usecase.ScanNested's doc comment for why project-service
// relays through the dev server rather than checking its own host.
type DevServerRelay struct {
	conn   *grpc.ClientConn
	client infrafleetv1.InfraFleetServiceClient
}

// NewDevServerRelay dials infra-fleet-service at addr. Lazy connection
// (grpc.NewClient doesn't block on connect), same convention as every
// other outbound client in this package.
func NewDevServerRelay(addr string) (*DevServerRelay, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return &DevServerRelay{conn: conn, client: infrafleetv1.NewInfraFleetServiceClient(conn)}, nil
}

func (c *DevServerRelay) Close() error {
	return c.conn.Close()
}

func (c *DevServerRelay) CreateConnection(ctx context.Context, devServerID, repoPath, worktreeID string) (string, error) {
	resp, err := c.client.CreateConnection(ctx, &infrafleetv1.CreateConnectionRequest{
		DevServerId: devServerID, RepoPath: repoPath, WorktreeId: worktreeID,
	})
	if err != nil {
		return "", fmt.Errorf("grpcclient: infra-fleet-service CreateConnection: %w", err)
	}
	return resp.GetConnectionId(), nil
}

func (c *DevServerRelay) Relay(ctx context.Context, connectionID, method string, paramsJSON []byte) ([]byte, error) {
	resp, err := c.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID, Method: method, ParamsJson: string(paramsJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: infra-fleet-service Relay: %w", err)
	}
	return []byte(resp.GetResultJson()), nil
}
