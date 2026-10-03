// Package vapidprovisioner implements usecase.VapidKeyProvisioner against
// credential-broker-service's EnsureVapidSigningKey RPC.
package vapidprovisioner

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

const (
	serviceIDKey = "x-orca-service-id"
	serviceID    = "notification-service"
	// codeVaultForbidden is the broker's stable code for a Vault 403.
	codeVaultForbidden = "CREDBROKER_VAULT_FORBIDDEN"
)

type Client struct {
	api credentialbrokerv1.CredentialBrokerServiceClient
}

var _ usecase.VapidKeyProvisioner = (*Client)(nil)

func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

// NewWithClient is for tests.
func NewWithClient(api credentialbrokerv1.CredentialBrokerServiceClient) *Client {
	return &Client{api: api}
}

func (c *Client) EnsureVapidSigningKey(ctx context.Context, tenantID string) (string, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, serviceIDKey, serviceID, grpcmw.MetadataTenantID, tenantID)
	resp, err := c.api.EnsureVapidSigningKey(ctx, &credentialbrokerv1.EnsureVapidSigningKeyRequest{})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.PermissionDenied && strings.Contains(st.Message(), codeVaultForbidden) {
			return "", fmt.Errorf("%w: %s", usecase.ErrVapidProvisionForbidden, st.Message())
		}
		return "", fmt.Errorf("vapidprovisioner: broker EnsureVapidSigningKey: %w", err)
	}
	return resp.GetPublicKey(), nil
}
