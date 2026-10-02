package authclient

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// AgentTokenIssuer mints the Orca MCP token placed in an agent's config via
// auth-service IssueMcpToken. That RPC issues a personal access token with a
// 1-day minimum lifetime (no sub-day TTL, no mcp_depth claim yet: both are
// BE-MCP-SOL-006 follow-ups), so the depth travels in ORCA_MCP_DEPTH instead.
type AgentTokenIssuer struct{ c *Client }

var _ usecase.AgentTokenIssuer = (*AgentTokenIssuer)(nil)

func NewAgentTokenIssuer(c *Client) *AgentTokenIssuer { return &AgentTokenIssuer{c: c} }

func (i *AgentTokenIssuer) Issue(ctx context.Context, userID, name string, scopes []string, ttl time.Duration) (domain.SecretValue, error) {
	ctx = tenant.WithUserID(ctx, userID)
	ctx, cancel := i.c.call(ctx)
	defer cancel()
	days := int32((ttl + 24*time.Hour - 1) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	resp, err := i.c.api.IssueMcpToken(ctx, &authv1.IssueMcpTokenRequest{Name: name, Scopes: scopes, ExpiresInDays: days})
	if err != nil {
		return domain.SecretValue{}, translate(err)
	}
	return domain.NewSecretValue(resp.GetSecret()), nil
}
