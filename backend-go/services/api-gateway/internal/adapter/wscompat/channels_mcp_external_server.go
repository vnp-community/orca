package wscompat

import (
	"context"
	"encoding/json"
	"errors"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// BE-MCP-SOL-014 channels mcp.externalServer.* (CONTRACT section 2.2). All take
// one object in args[0] (C10) and shape errors with mcpChannelError (C4).
// Authorization is enforced by mcp-service; the gateway only gates `review`
// (admin) so a non-admin never reaches it. setSecret carries plaintext once
// (D1): its `value` is never logged, traced, echoed or placed in an error (C11).

// McpExternalSecretRef / McpExternalServer mirror CONTRACT section 1.
type McpExternalSecretRef struct {
	Name      string `json:"name"`
	HasSecret bool   `json:"hasSecret"`
}

type McpExternalServerHealth struct {
	OK        bool   `json:"ok"`
	CheckedAt string `json:"checkedAt"`
	Error     string `json:"error,omitempty"`
}

type McpExternalServer struct {
	ID           string                   `json:"id"`
	Scope        string                   `json:"scope"`
	ScopeID      string                   `json:"scopeId,omitempty"`
	Name         string                   `json:"name"`
	Transport    string                   `json:"transport"`
	URL          string                   `json:"url,omitempty"`
	Command      string                   `json:"command,omitempty"`
	Args         []string                 `json:"args,omitempty"`
	EnvRefs      []McpExternalSecretRef   `json:"envRefs"`
	HeaderRefs   []McpExternalSecretRef   `json:"headerRefs"`
	Status       string                   `json:"status"`
	ToolsDigest  string                   `json:"toolsDigest,omitempty"`
	ToolsChanged bool                     `json:"toolsChanged"`
	Health       *McpExternalServerHealth `json:"health,omitempty"`
	CreatedBy    string                   `json:"createdBy"`
	ReviewedBy   string                   `json:"reviewedBy,omitempty"`
	UpdatedAt    string                   `json:"updatedAt,omitempty"`
}

type McpExternalTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type McpExternalProbeResult struct {
	Transport     string            `json:"transport"`
	Tools         []McpExternalTool `json:"tools"`
	Digest        string            `json:"digest"`
	ApprovedTools []McpExternalTool `json:"approvedTools,omitempty"`
}

func registerMcpExternalServerChannels(r *Registry, d McpChannelDeps) {
	r.Register("mcp.externalServer.list", mcpHandler(d, false, mcpExternalList(d)))
	r.Register("mcp.externalServer.upsert", mcpHandler(d, false, mcpExternalUpsert(d)))
	r.Register("mcp.externalServer.setSecret", mcpHandler(d, false, mcpExternalSetSecret(d)))
	r.Register("mcp.externalServer.probe", mcpHandler(d, false, mcpExternalProbe(d)))
	r.Register("mcp.externalServer.review", mcpHandler(d, true, mcpExternalReview(d)))
	r.Register("mcp.externalServer.delete", mcpHandler(d, false, mcpExternalDelete(d)))
}

func mcpRegistryClient(d McpChannelDeps) (mcpv1.McpRegistryServiceClient, error) {
	if d.Registry == nil {
		return nil, errors.New("MCP_UNAVAILABLE: mcp-service is not configured")
	}
	return d.Registry, nil
}

func toMcpRefs(in []*mcpv1.ExternalSecretRef) []McpExternalSecretRef {
	out := make([]McpExternalSecretRef, 0, len(in))
	for _, r := range in {
		out = append(out, McpExternalSecretRef{Name: r.GetName(), HasSecret: r.GetHasSecret()})
	}
	return out
}

func toMcpExternalServer(s *mcpv1.ExternalServer) McpExternalServer {
	out := McpExternalServer{
		ID: s.GetId(), Scope: s.GetScope(), ScopeID: s.GetScopeId(), Name: s.GetName(), Transport: s.GetTransport(), URL: s.GetUrl(),
		Command: s.GetCommand(), Args: s.GetArgs(), EnvRefs: toMcpRefs(s.GetEnvRefs()), HeaderRefs: toMcpRefs(s.GetHeaderRefs()),
		Status: s.GetStatus(), ToolsDigest: s.GetToolsDigest(), ToolsChanged: s.GetToolsChanged(), CreatedBy: s.GetCreatedBy(),
		ReviewedBy: s.GetReviewedBy(), UpdatedAt: tsString(s.GetUpdatedAt()),
	}
	if s.GetHasHealth() {
		out.Health = &McpExternalServerHealth{OK: s.GetHealthOk(), CheckedAt: tsString(s.GetHealthCheckedAt()), Error: s.GetHealthError()}
	}
	return out
}

func toMcpTools(in []*mcpv1.ExternalToolInfo) []McpExternalTool {
	out := make([]McpExternalTool, 0, len(in))
	for _, t := range in {
		out = append(out, McpExternalTool{Name: t.GetName(), Description: t.GetDescription()})
	}
	return out
}

func mcpExternalList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		var in struct {
			Scope string `json:"scope"`
		}
		if len(args) > 0 && string(args[0]) != "null" {
			v, err := mcpArgs[struct {
				Scope string `json:"scope"`
			}](args)
			if err != nil {
				return nil, err
			}
			in = v
		}
		c, err := mcpRegistryClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ListExternalServers(ctx, &mcpv1.ListExternalServersRequest{Scope: in.Scope})
		if err != nil {
			return nil, err
		}
		out := make([]McpExternalServer, 0, len(resp.GetServers()))
		for _, s := range resp.GetServers() {
			out = append(out, toMcpExternalServer(s))
		}
		return out, nil
	}
}

func refNames(refs []McpExternalSecretRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return out
}

// mcpExternalUpsert forwards only the spec fields. status, toolsDigest,
// hasSecret, health and the actor fields a client may send are ignored: the
// server computes them (CONTRACT).
func mcpExternalUpsert(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[McpExternalServer](args)
		if err != nil {
			return nil, err
		}
		c, err := mcpRegistryClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.UpsertExternalServer(ctx, &mcpv1.UpsertExternalServerRequest{
			Id: in.ID, Scope: in.Scope, ScopeId: in.ScopeID, Name: in.Name, Transport: in.Transport, Url: in.URL, Command: in.Command,
			Args: in.Args, EnvRefNames: refNames(in.EnvRefs), HeaderRefNames: refNames(in.HeaderRefs),
		})
		if err != nil {
			return nil, err
		}
		return toMcpExternalServer(resp.GetServer()), nil
	}
}

// mcpExternalSetSecret never includes the decoder's error text (it could quote
// input) and never returns the value.
func mcpExternalSetSecret(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		var in struct {
			ServerID string `json:"serverId"`
			Kind     string `json:"kind"`
			Name     string `json:"name"`
			Value    string `json:"value"`
		}
		if len(args) < 1 || json.Unmarshal(args[0], &in) != nil {
			return nil, errors.New("MCP_INVALID_ARGUMENT: expected {serverId, kind, name, value}")
		}
		if in.ServerID == "" || in.Kind == "" || in.Name == "" {
			return nil, errors.New("MCP_INVALID_ARGUMENT: serverId, kind and name are required")
		}
		if in.Value == "" {
			return nil, errors.New("MCP_INVALID_ARGUMENT: value is required")
		}
		c, err := mcpRegistryClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.SetExternalServerSecret(ctx, &mcpv1.SetExternalServerSecretRequest{
			ServerId: in.ServerID, Kind: in.Kind, Name: in.Name, Value: in.Value})
		if err != nil {
			return nil, err
		}
		return struct {
			HasSecret bool `json:"hasSecret"`
		}{HasSecret: resp.GetHasSecret()}, nil
	}
}

func mcpExternalProbe(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			ServerID string `json:"serverId"`
		}](args)
		if err != nil {
			return nil, err
		}
		c, err := mcpRegistryClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ProbeExternalServer(ctx, &mcpv1.ProbeExternalServerRequest{ServerId: in.ServerID})
		if err != nil {
			return nil, err
		}
		out := McpExternalProbeResult{Transport: resp.GetTransport(), Tools: toMcpTools(resp.GetTools()), Digest: resp.GetDigest()}
		if len(resp.GetApprovedTools()) > 0 {
			out.ApprovedTools = toMcpTools(resp.GetApprovedTools())
		}
		return out, nil
	}
}

func mcpExternalReview(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			ServerID    string `json:"serverId"`
			Decision    string `json:"decision"`
			ToolsDigest string `json:"toolsDigest"`
		}](args)
		if err != nil {
			return nil, err
		}
		c, err := mcpRegistryClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ReviewExternalServer(ctx, &mcpv1.ReviewExternalServerRequest{
			ServerId: in.ServerID, Decision: in.Decision, ToolsDigest: in.ToolsDigest})
		if err != nil {
			return nil, err
		}
		return toMcpExternalServer(resp.GetServer()), nil
	}
}

func mcpExternalDelete(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			ServerID string `json:"serverId"`
		}](args)
		if err != nil {
			return nil, err
		}
		c, err := mcpRegistryClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		if _, err := c.DeleteExternalServer(ctx, &mcpv1.DeleteExternalServerRequest{ServerId: in.ServerID}); err != nil {
			return nil, err
		}
		return struct {
			OK bool `json:"ok"`
		}{OK: true}, nil
	}
}
