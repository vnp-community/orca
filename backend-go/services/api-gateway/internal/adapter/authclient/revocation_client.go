package authclient

import (
	"context"
	"fmt"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// RevocationClient implements usecase.RevocationChecker against
// auth-service's real IsServiceTokenRevoked RPC (CR-CLI-002/
// TASK-BE-CLI-005) — a single cheap jti -> bool lookup, no caching: unlike
// JWKSClient (a key set that only changes on rotation), a revocation must
// be observed immediately, so every call is a live RPC.
type RevocationClient struct {
	client authv1.AuthServiceClient
}

// NewRevocationClient wraps an already-dialed connection to auth-service.
func NewRevocationClient(client authv1.AuthServiceClient) *RevocationClient {
	return &RevocationClient{client: client}
}

// IsRevoked reports whether jti has been revoked. An unknown jti (never
// existed, or minted before auth-service's revocation table existed)
// reports false, not an error — matching IsServiceTokenRevoked's own
// fail-open-on-unknown-jti contract (see that RPC's proto doc comment).
func (c *RevocationClient) IsRevoked(ctx context.Context, jti string) (bool, error) {
	resp, err := c.client.IsServiceTokenRevoked(ctx, &authv1.IsServiceTokenRevokedRequest{Jti: jti})
	if err != nil {
		return false, fmt.Errorf("authclient: checking token revocation: %w", err)
	}
	return resp.GetRevoked(), nil
}
