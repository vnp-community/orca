package authclient

import (
	"context"
	"fmt"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// DeviceSecretResolver implements wscompat.DeviceSecretResolver by calling
// auth-service's internal-only ResolveDeviceSharedSecret RPC — the same
// authv1.AuthServiceClient every other real client in this package wraps.
// Kept in this package (rather than wscompat itself) so wscompat never
// dials a downstream service directly, matching every other
// wscompat-consumed real client's composition-root wiring.
type DeviceSecretResolver struct {
	client authv1.AuthServiceClient
}

func NewDeviceSecretResolver(client authv1.AuthServiceClient) *DeviceSecretResolver {
	return &DeviceSecretResolver{client: client}
}

// ResolveSharedSecret returns the paired device's raw 32-byte NaCl shared
// secret — never persisted or logged by this method, per
// ResolveDeviceSharedSecretResponse's proto doc comment.
func (r *DeviceSecretResolver) ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error) {
	resp, err := r.client.ResolveDeviceSharedSecret(ctx, &authv1.ResolveDeviceSharedSecretRequest{DeviceId: deviceID})
	if err != nil {
		return nil, fmt.Errorf("authclient: resolving shared secret for device %q: %w", deviceID, err)
	}
	return resp.GetSharedSecret(), nil
}
