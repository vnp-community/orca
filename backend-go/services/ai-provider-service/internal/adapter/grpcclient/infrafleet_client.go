// Package grpcclient (this file) implements ai-provider-service's
// InfraFleetClient port against a real infra-fleet-service gRPC
// connection. Two calls per Relay: resolve dev_server_id -> connectionId,
// then relay — see TASK-025 for the ResolveConnection addition this
// depends on.
package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetClient implements usecase.InfraFleetClient against a real
// infra-fleet-service connection.
type InfraFleetClient struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetClient(conn grpc.ClientConnInterface) *InfraFleetClient {
	return &InfraFleetClient{client: infrafleetv1.NewInfraFleetServiceClient(conn)}
}

var _ usecase.InfraFleetClient = (*InfraFleetClient)(nil)

func (c *InfraFleetClient) Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error) {
	resolved, err := c.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
	if err != nil {
		return nil, fmt.Errorf("infrafleet: resolving dev server %s: %w", devServerID, err)
	}
	if !resolved.GetConnected() {
		return nil, fmt.Errorf("infrafleet: dev server %s has no active connection", devServerID)
	}

	paramsJSON, err := marshalParams(params)
	if err != nil {
		return nil, err
	}
	// resolved.GetConnectionId() is infra.connections.id — a DIFFERENT id
	// space from resolved.GetDevServer().GetId() (dev_servers.id). Relay's
	// ConnectionId must be the former; see TASK-025's ResolveConnectionResponse
	// doc comment.
	resp, err := c.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: resolved.GetConnectionId(),
		Method:       method,
		ParamsJson:   paramsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("infrafleet: relaying %s to dev server %s: %w", method, devServerID, err)
	}
	return unmarshalResult(resp.GetResultJson())
}

func marshalParams(params map[string]any) (string, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("infrafleet: marshaling relay params: %w", err)
	}
	return string(b), nil
}

func unmarshalResult(resultJSON string) (map[string]any, error) {
	var out map[string]any
	if resultJSON == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(resultJSON), &out); err != nil {
		return nil, fmt.Errorf("infrafleet: unmarshaling relay result: %w", err)
	}
	return out, nil
}
