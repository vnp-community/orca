// Package authclient implements usecase.DeviceSecretResolver by dialing
// auth-service's internal-only ResolveDeviceSharedSecret RPC (SOL-MB-01) —
// never routed through api-gateway's REST facade.
package authclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// DeviceSecretResolver implements usecase.DeviceSecretResolver (defined in
// internal/usecase/deliver_push.go) against a real auth-service connection.
type DeviceSecretResolver struct {
	conn   *grpc.ClientConn
	client authv1.AuthServiceClient
}

// New dials auth-service directly — insecure transport credentials here
// are a local-dev/scaffold convenience only; production deploys terminate
// mTLS via the service mesh sidecar, matching every other peer-service
// client in this workspace (see e.g. ai-provider-service's
// InfraFleetClient dial).
func New(addr string) (*DeviceSecretResolver, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("authclient: dial auth-service at %q: %w", addr, err)
	}
	return &DeviceSecretResolver{conn: conn, client: authv1.NewAuthServiceClient(conn)}, nil
}

func (c *DeviceSecretResolver) Close() error { return c.conn.Close() }

// ResolveSharedSecret returns the paired device's raw 32-byte shared
// secret. The caller (DeliverPush) must not persist it — it exists only
// for the duration of one Seal call.
func (c *DeviceSecretResolver) ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error) {
	resp, err := c.client.ResolveDeviceSharedSecret(ctx, &authv1.ResolveDeviceSharedSecretRequest{DeviceId: deviceID})
	if err != nil {
		return nil, fmt.Errorf("authclient: resolve device shared secret: %w", err)
	}
	return resp.GetSharedSecret(), nil
}
