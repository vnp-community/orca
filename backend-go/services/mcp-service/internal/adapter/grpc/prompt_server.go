package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// PromptServer adds the custom prompt RPCs (BE-MCP-SOL-011) on top of any
// McpServiceServer.
type PromptServer struct {
	mcpv1.McpServiceServer
	uc *usecase.PromptAdmin
}

func WithPrompts(base mcpv1.McpServiceServer, uc *usecase.PromptAdmin) *PromptServer {
	return &PromptServer{McpServiceServer: base, uc: uc}
}

func toProtoPrompt(p domain.CustomPrompt) *mcpv1.CustomPrompt {
	args := make([]*mcpv1.PromptArgument, 0, len(p.Arguments))
	for _, a := range p.Arguments {
		args = append(args, &mcpv1.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required})
	}
	return &mcpv1.CustomPrompt{
		Id: p.ID, Name: p.Name, Description: p.Description, Arguments: args, Template: p.Template,
		Version: int32(p.Version), UpdatedAt: timestamppb.New(p.UpdatedAt), UpdatedBy: p.UpdatedBy,
	}
}

func fromProtoPrompt(p *mcpv1.CustomPrompt) domain.CustomPrompt {
	args := make([]domain.PromptArgument, 0, len(p.GetArguments()))
	for _, a := range p.GetArguments() {
		args = append(args, domain.PromptArgument{Name: a.GetName(), Description: a.GetDescription(), Required: a.GetRequired()})
	}
	return domain.CustomPrompt{ID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(), Arguments: args,
		Template: p.GetTemplate(), Version: int(p.GetVersion())}
}

func (s *PromptServer) ListPrompts(ctx context.Context, _ *mcpv1.ListPromptsRequest) (*mcpv1.ListPromptsResponse, error) {
	ps, err := s.uc.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListPromptsResponse{Prompts: make([]*mcpv1.CustomPrompt, 0, len(ps))}
	for _, p := range ps {
		resp.Prompts = append(resp.Prompts, toProtoPrompt(p))
	}
	return resp, nil
}

func (s *PromptServer) UpsertPrompt(ctx context.Context, req *mcpv1.UpsertPromptRequest) (*mcpv1.CustomPrompt, error) {
	if req.GetPrompt() == nil {
		return nil, toStatus(domain.ErrPromptInvalid("prompt", "is required"))
	}
	out, err := s.uc.Upsert(ctx, fromProtoPrompt(req.GetPrompt()))
	if err != nil {
		return nil, toStatus(err)
	}
	return toProtoPrompt(out), nil
}

func (s *PromptServer) DeletePrompt(ctx context.Context, req *mcpv1.DeletePromptRequest) (*emptypb.Empty, error) {
	if err := s.uc.Delete(ctx, req.GetPromptId()); err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, nil
}
