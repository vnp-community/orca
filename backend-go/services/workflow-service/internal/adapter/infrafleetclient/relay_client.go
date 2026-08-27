// Package infrafleetclient holds workflow-service's outbound gRPC adapter
// toward infra-fleet-service's generic Relay RPC — the execution-plane hop
// AgentExecutor, ShellExecutor, and NotificationExecutor (this package's
// three domain.StepExecutor implementations) all share. Mirrors
// git-gateway-service's internal/adapter/grpcclient package (RelayExecutor +
// tenant_forwarding.go), the closest existing peer of this adapter.
package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// Dial opens a gRPC client connection to infra-fleet-service at addr. The
// dial is lazy (grpc.NewClient doesn't block on connect), so infra-fleet-
// service being down doesn't fail workflow-service's startup. Uses insecure
// transport credentials — acceptable for local dev only; production must
// dial with mesh mTLS client credentials, same known gap every other
// service's outbound Dial helper in this workspace carries (see
// api-gateway's internal/adapter/grpc.Dial doc comment).
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("infrafleetclient: dial infra-fleet-service at %q: %w", addr, err)
	}
	return conn, nil
}

// relay marshals params, forwards the caller's tenant identity (see
// withTenantMetadata), calls infra-fleet-service's Relay RPC for
// connectionID/method, and unmarshals result_json into out. Shared by all
// three step executors in this package.
func relay(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, connectionID, method string, params, out any) error {
	if connectionID == "" {
		return fmt.Errorf("infrafleetclient: %s: connectionId is required in step config", method)
	}

	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return fmt.Errorf("infrafleetclient: %s: %w", method, err)
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("infrafleetclient: %s: marshal params: %w", method, err)
	}

	resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID,
		Method:       method,
		ParamsJson:   string(paramsJSON),
	})
	if err != nil {
		return fmt.Errorf("infrafleetclient: %s: relay rpc failed: %w", method, err)
	}

	if out == nil || resp.GetResultJson() == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(resp.GetResultJson()), out); err != nil {
		return fmt.Errorf("infrafleetclient: %s: decoding result_json: %w", method, err)
	}
	return nil
}
