package authclient

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// PatSuspender suspends or restores the personal access tokens of a tenant via
// auth-service SetMcpPatSuspension (internal-caller guarded, idempotent).
type PatSuspender struct{ c *Client }

var _ usecase.PatSuspender = (*PatSuspender)(nil)

func NewPatSuspender(c *Client) *PatSuspender { return &PatSuspender{c: c} }

func (p *PatSuspender) SetPatSuspension(ctx context.Context, tenantID string, suspended bool, reason string) error {
	ctx = tenant.WithTenantID(ctx, tenantID)
	ctx, cancel := p.c.call(ctx)
	defer cancel()
	_, err := p.c.api.SetMcpPatSuspension(ctx, &authv1.SetMcpPatSuspensionRequest{Suspended: suspended, Reason: reason})
	return translate(err)
}
