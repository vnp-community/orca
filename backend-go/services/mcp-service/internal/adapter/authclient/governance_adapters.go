package authclient

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

// ClientStatuses answers "is this OAuth client allowed in the calling tenant"
// from auth-service's tenant view. Unknown clients are an error (not allowed).
type ClientStatuses struct{ as usecase.AuthorizationServer }

func NewClientStatuses(as usecase.AuthorizationServer) *ClientStatuses {
	return &ClientStatuses{as: as}
}

var errUnknownClient = errors.New("authclient: client not known in tenant")

// ClientStatus uses the tenant already on ctx (forwarded as metadata), which
// is why the tenantID argument is only a cache-key concern of the caller.
func (c *ClientStatuses) ClientStatus(ctx context.Context, _ string, clientID string) (string, error) {
	views, err := c.as.ListClientsForTenant(ctx)
	if err != nil {
		return "", err
	}
	for _, v := range views {
		if v.ClientID == clientID {
			return v.Status, nil
		}
	}
	return "", errUnknownClient
}

// GrantTokenRevoker revokes the refresh tokens of an OAuth grant at auth-service.
type GrantTokenRevoker struct{ as usecase.AuthorizationServer }

func NewGrantTokenRevoker(as usecase.AuthorizationServer) *GrantTokenRevoker {
	return &GrantTokenRevoker{as: as}
}

func (g *GrantTokenRevoker) RevokeGrantTokens(ctx context.Context, grantID, reason string) error {
	return g.as.RevokeGrant(ctx, grantID, reason)
}

var (
	_ usecase.ClientStatusReader  = (*ClientStatuses)(nil)
	_ usecase.RefreshTokenRevoker = (*GrantTokenRevoker)(nil)
)
