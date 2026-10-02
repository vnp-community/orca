package prompts

import (
	"context"
	"time"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// GRPCStore reads custom prompts from mcp-service with the caller's identity
// in gRPC metadata (never in the body).
type GRPCStore struct {
	Client  mcpv1.McpServiceClient
	Timeout time.Duration // default 5s
}

func (s GRPCStore) ListCustom(ctx context.Context, p mcpserver.Principal) ([]Custom, error) {
	t := s.Timeout
	if t <= 0 {
		t = 5 * time.Second
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: p.TenantID, UserID: p.UserID, Role: p.Role})
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	resp, err := s.Client.ListPrompts(ctx, &mcpv1.ListPromptsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]Custom, 0, len(resp.GetPrompts()))
	for _, c := range resp.GetPrompts() {
		cp := Custom{ID: c.GetId(), Name: c.GetName(), Description: c.GetDescription(), Template: c.GetTemplate(), Version: int(c.GetVersion())}
		for _, a := range c.GetArguments() {
			cp.Arguments = append(cp.Arguments, Argument{Name: a.GetName(), Description: a.GetDescription(), Required: a.GetRequired()})
		}
		out = append(out, cp)
	}
	return out, nil
}
