package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// RegistryServer exposes the external server registry (BE-MCP-SOL-014). Requests
// are never logged or formatted: SetExternalServerSecret carries plaintext.
type RegistryServer struct {
	mcpv1.UnimplementedMcpRegistryServiceServer
	reg     *usecase.ExternalServerRegistry
	resolve *usecase.ResolveAgentMcpConfig
}

func NewRegistryServer(reg *usecase.ExternalServerRegistry, resolve *usecase.ResolveAgentMcpConfig) *RegistryServer {
	return &RegistryServer{reg: reg, resolve: resolve}
}

func toProtoRefs(refs []domain.SecretRef) []*mcpv1.ExternalSecretRef {
	out := make([]*mcpv1.ExternalSecretRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, &mcpv1.ExternalSecretRef{Name: r.Name, HasSecret: r.HasSecret()})
	}
	return out
}

func toProtoServer(s domain.ExternalServer) *mcpv1.ExternalServer {
	p := &mcpv1.ExternalServer{
		Id: s.ID, Scope: s.Scope, ScopeId: s.ScopeID, Name: s.Name, Transport: s.Transport, Url: s.URL, Command: s.Command,
		Args: s.Args, EnvRefs: toProtoRefs(s.EnvRefs), HeaderRefs: toProtoRefs(s.HeaderRefs), Status: s.Status,
		ToolsDigest: s.ToolsDigest(), ToolsChanged: s.ToolsChanged(), CreatedBy: s.CreatedBy, ReviewedBy: s.ReviewedBy,
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
	if s.Health != nil {
		p.HasHealth, p.HealthOk, p.HealthCheckedAt, p.HealthError = true, s.Health.OK, timestamppb.New(s.Health.CheckedAt), s.Health.Error
	}
	return p
}

func toProtoTools(ts []domain.ToolInfo) []*mcpv1.ExternalToolInfo {
	out := make([]*mcpv1.ExternalToolInfo, 0, len(ts))
	for _, t := range ts {
		out = append(out, &mcpv1.ExternalToolInfo{Name: t.Name, Description: t.Description})
	}
	return out
}

func (s *RegistryServer) ListExternalServers(ctx context.Context, req *mcpv1.ListExternalServersRequest) (*mcpv1.ListExternalServersResponse, error) {
	list, err := s.reg.List(ctx, req.GetScope())
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListExternalServersResponse{Servers: make([]*mcpv1.ExternalServer, 0, len(list))}
	for _, x := range list {
		resp.Servers = append(resp.Servers, toProtoServer(x))
	}
	return resp, nil
}

func (s *RegistryServer) UpsertExternalServer(ctx context.Context, req *mcpv1.UpsertExternalServerRequest) (*mcpv1.UpsertExternalServerResponse, error) {
	out, err := s.reg.Upsert(ctx, usecase.UpsertExternalServerInput{ID: req.GetId(), Spec: domain.ServerSpec{
		Scope: req.GetScope(), ScopeID: req.GetScopeId(), Name: req.GetName(), Transport: req.GetTransport(), URL: req.GetUrl(),
		Command: req.GetCommand(), Args: req.GetArgs(), EnvNames: req.GetEnvRefNames(), HeaderNames: req.GetHeaderRefNames(),
	}})
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.UpsertExternalServerResponse{Server: toProtoServer(out)}, nil
}

func (s *RegistryServer) SetExternalServerSecret(ctx context.Context, req *mcpv1.SetExternalServerSecretRequest) (*mcpv1.SetExternalServerSecretResponse, error) {
	err := s.reg.SetSecret(ctx, usecase.SetSecretInput{
		ServerID: req.GetServerId(), Kind: req.GetKind(), Name: req.GetName(), Value: domain.NewSecretValue(req.GetValue()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.SetExternalServerSecretResponse{HasSecret: true}, nil
}

func (s *RegistryServer) ProbeExternalServer(ctx context.Context, req *mcpv1.ProbeExternalServerRequest) (*mcpv1.ProbeExternalServerResponse, error) {
	out, err := s.reg.Probe(ctx, req.GetServerId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.ProbeExternalServerResponse{
		Transport: out.Transport, Tools: toProtoTools(out.Tools), Digest: out.Digest,
		ApprovedTools: toProtoTools(out.ApprovedTools), ToolsChanged: out.ToolsChanged,
	}, nil
}

func (s *RegistryServer) ReviewExternalServer(ctx context.Context, req *mcpv1.ReviewExternalServerRequest) (*mcpv1.ReviewExternalServerResponse, error) {
	out, err := s.reg.Review(ctx, usecase.ReviewInput{ServerID: req.GetServerId(), Decision: req.GetDecision(), ToolsDigest: req.GetToolsDigest()})
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.ReviewExternalServerResponse{Server: toProtoServer(out)}, nil
}

func (s *RegistryServer) DeleteExternalServer(ctx context.Context, req *mcpv1.DeleteExternalServerRequest) (*mcpv1.DeleteExternalServerResponse, error) {
	if err := s.reg.Delete(ctx, req.GetServerId()); err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.DeleteExternalServerResponse{Ok: true}, nil
}

func (s *RegistryServer) ResolveAgentMcpConfig(ctx context.Context, req *mcpv1.ResolveAgentMcpConfigRequest) (*mcpv1.ResolveAgentMcpConfigResponse, error) {
	out, err := s.resolve.Execute(ctx, usecase.ResolveAgentMcpConfigInput{
		UserID: req.GetUserId(), ProjectID: req.GetProjectId(), AgentKind: req.GetAgentKind(), HostOS: req.GetHostOs(),
		HostKind: req.GetHostKind(), ParentDepth: int(req.GetParentMcpDepth()), ParentSessionID: req.GetParentMcpSessionId(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ResolveAgentMcpConfigResponse{ExtraArgs: out.ExtraArgs, McpDepth: int32(out.Depth)}
	for _, f := range out.Files {
		resp.Files = append(resp.Files, &mcpv1.AgentConfigFile{Path: f.Path, Content: f.Content, Mode: f.Mode})
	}
	for _, e := range out.Env {
		resp.Env = append(resp.Env, &mcpv1.AgentEnvVar{Name: e.Name, Value: string(e.Value.Reveal())})
		e.Value.Zero()
	}
	for _, w := range out.Warnings {
		resp.Warnings = append(resp.Warnings, &mcpv1.AgentConfigWarning{ServerName: w.ServerName, Reason: w.Reason})
	}
	return resp, nil
}
