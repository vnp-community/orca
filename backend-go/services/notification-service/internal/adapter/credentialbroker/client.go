// Package credentialbroker is notification-service's client to
// credential-broker-service for push credential material (APNs/FCM) —
// CREDENTIAL_CATEGORY_SERVICE_SECRET, owner_id "apns"/"fcm". Mirrors
// scm-integration-service's internal/adapter/credentialbroker/client.go
// (same RPC, different category) — see that package's doc comment for
// why ResolveCredentialByOwner (not ResolveCredential) is the right RPC
// for a caller that only ever knows (tenant_id, category, owner_id), never
// an opaque credential_id.
package credentialbroker

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

type Resolver struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service —
// cmd/server/main.go reuses the SAME connection already dialed for
// vaultsigner.New(...) (see that file's brokerConn), not a second dial.
func New(conn grpc.ClientConnInterface) *Resolver {
	return &Resolver{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

func (r *Resolver) Resolve(ctx context.Context, tenantID, ownerID string) ([]byte, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SERVICE_SECRET,
		OwnerId:  ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("credentialbroker: resolving %s push credential: %w", ownerID, err)
	}
	return resp.GetValue(), nil
}
