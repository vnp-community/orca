// Package vaultsigner implements usecase.VaultSigner against
// credential-broker-service's SignVapidPayload RPC — Epic B
// (docs/execution-plan.md §8).
//
// Before Epic B, this package called common/secrets.TransitEncrypt
// directly against Vault — the one documented exception to
// architecture/06-secrets-vault-architecture.md's "no other service talks
// to Vault directly for tenant secret material" rule (VAPID keys are
// tenant-scoped, one per tenant_id). Epic B closes that exception for
// real, not by amending the rule: credential-broker-service now owns the
// "vapid-signing-<tenant_id>" Transit key and this adapter is a thin gRPC
// client, exactly like every other Vault-touching operation in this
// system. See credentialbroker.proto's SignVapidPayload doc comment for
// why it's a narrow, dedicated RPC rather than a generic "Transit-sign
// anything" surface.
package vaultsigner

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// Signer implements usecase.VaultSigner against a real
// credential-broker-service connection.
type Signer struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service (see
// cmd/server/main.go's composition root for the dial + insecure-transport-
// credentials rationale shared by every peer-service client in this
// workspace).
func New(conn grpc.ClientConnInterface) *Signer {
	return &Signer{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

// SignVapidPayload signs payload under tenantID's VAPID Transit key,
// provisioned and owned by credential-broker-service (key name convention:
// "vapid-signing-<tenant_id>", unchanged from this adapter's pre-Epic-B
// direct-Vault version — see credential-broker-service's
// usecase.vapidKeyName doc comment).
func (s *Signer) SignVapidPayload(ctx context.Context, tenantID string, payload []byte) (string, error) {
	resp, err := s.client.SignVapidPayload(ctx, &credentialbrokerv1.SignVapidPayloadRequest{
		TenantId: tenantID,
		Payload:  payload,
	})
	if err != nil {
		return "", fmt.Errorf("vaultsigner: credential-broker-service SignVapidPayload: %w", err)
	}
	return resp.GetSignature(), nil
}
