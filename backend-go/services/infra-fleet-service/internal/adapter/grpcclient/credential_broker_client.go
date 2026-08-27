// Package grpcclient implements infra-fleet-service's CredentialBrokerClient
// port (internal/usecase/ports.go) against a real credential-broker-service
// gRPC connection — see SOL-AWS-01.
//
// UNLIKE ai-provider-service's identically-named adapter, this client DOES
// call ResolveCredential (the plaintext-returning RPC) — relay-websocket
// mode requires infra-fleet-service to present the token outbound as an
// Authorization: Bearer header, a genuinely different case from every
// other service's credential usage in this codebase, where the secret is
// only ever compared against, never re-presented. Do not copy this
// pattern to another service's CredentialBrokerClient without the same
// justification.
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// CredentialBrokerClient implements usecase.CredentialBrokerClient against
// a real credential-broker-service connection.
type CredentialBrokerClient struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service.
func New(conn grpc.ClientConnInterface) *CredentialBrokerClient {
	return &CredentialBrokerClient{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

var _ usecase.CredentialBrokerClient = (*CredentialBrokerClient)(nil)

const credentialCategory = credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN

func (c *CredentialBrokerClient) WriteCredential(ctx context.Context, tenantID, ownerID string, envelope []byte) (usecase.CredentialRef, error) {
	resp, err := c.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId: tenantID, OwnerId: ownerID, Category: credentialCategory, EncryptedEnvelope: envelope,
	})
	if err != nil {
		return usecase.CredentialRef{}, fmt.Errorf("grpcclient: credential-broker-service WriteCredential: %w", err)
	}
	return usecase.CredentialRef{ID: resp.GetMetadata().GetId()}, nil
}

func (c *CredentialBrokerClient) ResolveCredential(ctx context.Context, credentialRefID string) ([]byte, error) {
	resp, err := c.client.ResolveCredential(ctx, &credentialbrokerv1.ResolveCredentialRequest{CredentialId: credentialRefID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: credential-broker-service ResolveCredential: %w", err)
	}
	return resp.GetValue(), nil
}
