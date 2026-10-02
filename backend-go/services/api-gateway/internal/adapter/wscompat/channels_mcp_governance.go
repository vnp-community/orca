package wscompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// BE-MCP-SOL-012 / 013 admin channels: settings, tool policies, kill switch.
// All are admin-only (checked here AND again in mcp-service), take exactly one
// object in args[0] (C10) and surface errors through mcpChannelError (C4).

type McpToolPolicyMatch struct {
	Tool      string   `json:"tool,omitempty"`
	Namespace string   `json:"namespace,omitempty"`
	Risk      string   `json:"risk,omitempty"`
	ClientID  string   `json:"clientId,omitempty"`
	Roles     []string `json:"roles,omitempty"`
}

// McpToolPolicy mirrors CONTRACT section 1.
type McpToolPolicy struct {
	ID        string             `json:"id,omitempty"`
	Version   int                `json:"version"`
	UpdatedAt string             `json:"updatedAt,omitempty"`
	UpdatedBy string             `json:"updatedBy,omitempty"`
	Match     McpToolPolicyMatch `json:"match"`
	Decision  string             `json:"decision"`
	Note      string             `json:"note,omitempty"`
}

type McpAdminSettings struct {
	Enabled            bool          `json:"enabled"`
	DcrEnabled         bool          `json:"dcrEnabled"`
	MaxTokenDays       int           `json:"maxTokenDays"`
	ApprovalTTLSeconds int           `json:"approvalTtlSeconds"`
	KillSwitch         McpKillSwitch `json:"killSwitch"`
}

type McpKillSwitchEntry struct {
	Scope    string `json:"scope"`
	TargetID string `json:"targetId,omitempty"`
	Active   bool   `json:"active"`
	Reason   string `json:"reason"`
	At       string `json:"at"`
	By       string `json:"by"`
}

func registerMcpGovernanceChannels(r *Registry, d McpChannelDeps) {
	r.Register("mcp.admin.settings.get", mcpHandler(d, true, mcpSettingsGet(d)))
	r.Register("mcp.admin.settings.set", mcpHandler(d, true, mcpSettingsSet(d)))
	r.Register("mcp.admin.policy.list", mcpHandler(d, true, mcpPolicyList(d)))
	r.Register("mcp.admin.policy.upsert", mcpHandler(d, true, mcpPolicyUpsert(d)))
	r.Register("mcp.admin.policy.delete", mcpHandler(d, true, mcpPolicyDelete(d)))
	r.Register("mcp.admin.policy.explain", mcpHandler(d, true, mcpPolicyExplain(d)))
	r.Register("mcp.admin.killswitch.set", mcpHandler(d, true, mcpKillSwitchSet(d)))
	r.Register("mcp.admin.killswitch.list", mcpHandler(d, true, mcpKillSwitchList(d)))
}

func toMcpSettings(s *mcpv1.TenantSettings) McpAdminSettings {
	ks := McpKillSwitch{Active: s.GetKillSwitch().GetActive(), Reason: s.GetKillSwitch().GetReason(), At: tsString(s.GetKillSwitch().GetAt())}
	return McpAdminSettings{Enabled: s.GetEnabled(), DcrEnabled: s.GetDcrEnabled(), MaxTokenDays: int(s.GetMaxTokenDays()),
		ApprovalTTLSeconds: int(s.GetApprovalTtlSeconds()), KillSwitch: ks}
}

func mcpSettingsGet(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		s, err := c.GetTenantSettings(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		return toMcpSettings(s), nil
	}
}

// mcpSettingsSet accepts a partial patch of exactly four fields. Anything
// else (notably killSwitch, which has its own channel and audit trail) is an
// error rather than being silently ignored.
func mcpSettingsSet(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if len(args) < 1 {
			return nil, errors.New("MCP_INVALID_ARGUMENT: expected one object argument")
		}
		var in struct {
			Enabled            *bool `json:"enabled"`
			DcrEnabled         *bool `json:"dcrEnabled"`
			MaxTokenDays       *int  `json:"maxTokenDays"`
			ApprovalTTLSeconds *int  `json:"approvalTtlSeconds"`
		}
		dec := json.NewDecoder(bytes.NewReader(args[0]))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return nil, errors.New("MCP_INVALID_ARGUMENT: only enabled, dcrEnabled, maxTokenDays and approvalTtlSeconds can be set")
		}
		if in.Enabled == nil && in.DcrEnabled == nil && in.MaxTokenDays == nil && in.ApprovalTTLSeconds == nil {
			return nil, errors.New("MCP_INVALID_ARGUMENT: no settings to change")
		}
		req := &mcpv1.UpdateTenantSettingsRequest{Enabled: in.Enabled, DcrEnabled: in.DcrEnabled}
		if in.MaxTokenDays != nil {
			v := int32(*in.MaxTokenDays)
			req.MaxTokenDays = &v
		}
		if in.ApprovalTTLSeconds != nil {
			v := int32(*in.ApprovalTTLSeconds)
			req.ApprovalTtlSeconds = &v
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		s, err := c.UpdateTenantSettings(ctx, req)
		if err != nil {
			return nil, err
		}
		return toMcpSettings(s), nil
	}
}

func toMcpPolicy(p *mcpv1.ToolPolicy) McpToolPolicy {
	m := p.GetMatch()
	return McpToolPolicy{
		ID: p.GetId(), Version: int(p.GetVersion()), UpdatedAt: tsString(p.GetUpdatedAt()), UpdatedBy: p.GetUpdatedBy(),
		Match:    McpToolPolicyMatch{Tool: m.GetTool(), Namespace: m.GetNamespace(), Risk: m.GetRisk(), ClientID: m.GetClientId(), Roles: m.GetRoles()},
		Decision: p.GetDecision(), Note: p.GetNote(),
	}
}

func mcpPolicyList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ListToolPolicies(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		out := make([]McpToolPolicy, 0, len(resp.GetPolicies()))
		for _, p := range resp.GetPolicies() {
			out = append(out, toMcpPolicy(p))
		}
		return out, nil
	}
}

// touchedTools expands a policy match over the gateway's tool catalog so
// mcp-service can refuse policies that would loosen hard-denied tools.
// Hard-denied channels are part of the catalog views, so a namespace or
// client-wide policy that reaches one is caught. Without a catalog the list is
// empty and mcp-service applies its own name/namespace checks.
func touchedTools(ctx context.Context, d McpChannelDeps, tenantID string, m McpToolPolicyMatch) []*mcpv1.ToolRef {
	if d.ToolCatalog == nil {
		return nil
	}
	views, err := d.ToolCatalog.AdminToolViews(ctx, tenantID)
	if err != nil {
		return nil
	}
	var out []*mcpv1.ToolRef
	for _, v := range views {
		if (m.Tool != "" && v.Name != m.Tool) || (m.Namespace != "" && v.Namespace != m.Namespace) || (m.Risk != "" && v.Risk != m.Risk) {
			continue
		}
		out = append(out, viewToolRef(v))
	}
	return out
}

// viewToolRef converts a catalog view to the wire ToolRef. spawns_process is
// derived from the channel, never taken from anything a client supplies.
func viewToolRef(v McpToolView) *mcpv1.ToolRef {
	return &mcpv1.ToolRef{
		Name: v.Name, Channel: v.Channel, Namespace: v.Namespace, Risk: v.Risk, RequiredScope: v.RequiredScope, Title: v.Title,
		OpenWorld: v.Annotations.OpenWorld, SpawnsProcess: mcpSpawningChannels[v.Channel],
	}
}

var mcpSpawningChannels = map[string]bool{"agent.start": true, "agent.resume": true, "agent.switchAccount": true, "terminal.create": true}

func mcpPolicyUpsert(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[McpToolPolicy](args)
		if err != nil {
			return nil, err
		}
		in.Match.Tool, in.Match.Namespace = strings.TrimSpace(in.Match.Tool), strings.TrimSpace(in.Match.Namespace)
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.UpsertToolPolicy(ctx, &mcpv1.UpsertToolPolicyRequest{
			Policy: &mcpv1.ToolPolicy{
				Id: in.ID, Version: int32(in.Version), Decision: in.Decision, Note: in.Note,
				Match: &mcpv1.ToolPolicyMatch{Tool: in.Match.Tool, Namespace: in.Match.Namespace, Risk: in.Match.Risk, ClientId: in.Match.ClientID, Roles: in.Match.Roles},
			},
			TouchedTools: touchedTools(ctx, d, id.TenantID, in.Match),
		})
		if err != nil {
			return nil, err
		}
		return toMcpPolicy(resp), nil
	}
}

func mcpPolicyDelete(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			PolicyID string `json:"policyId"`
		}](args)
		if err != nil {
			return nil, err
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		if _, err := c.DeleteToolPolicy(ctx, &mcpv1.DeleteToolPolicyRequest{PolicyId: in.PolicyID}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}

func mcpPolicyExplain(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			Tool     string `json:"tool"`
			UserID   string `json:"userId"`
			ClientID string `json:"clientId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if d.ToolCatalog == nil {
			return nil, errors.New("MCP_UNAVAILABLE: tool catalog is not configured")
		}
		views, err := d.ToolCatalog.AdminToolViews(ctx, id.TenantID)
		if err != nil {
			return nil, err
		}
		var ref *mcpv1.ToolRef
		for _, v := range views {
			if v.Name == in.Tool {
				ref = viewToolRef(v)
				break
			}
		}
		if ref == nil {
			return nil, errors.New("MCP_NOT_FOUND: not found")
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		// The role of an arbitrary userId is not resolved here: mcp-service
		// answers for role "user" and adds the reason role_unresolved.
		resp, err := c.ExplainPolicy(ctx, &mcpv1.ExplainPolicyRequest{Tool: in.Tool, UserId: in.UserID, ClientId: in.ClientID, ToolRef: ref})
		if err != nil {
			return nil, err
		}
		reasons := resp.GetReasons()
		if reasons == nil {
			reasons = []string{}
		}
		return map[string]any{"decision": resp.GetDecision(), "reasons": reasons}, nil
	}
}

func mcpKillSwitchSet(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			Scope        string `json:"scope"`
			TargetID     string `json:"targetId"`
			Active       *bool  `json:"active"`
			Reason       string `json:"reason"`
			RevokeTokens bool   `json:"revokeTokens"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.Active == nil {
			return nil, errors.New("MCP_INVALID_ARGUMENT: active is required")
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		if _, err := c.SetKillSwitch(ctx, &mcpv1.SetKillSwitchRequest{
			Scope: in.Scope, TargetId: in.TargetID, Active: *in.Active, Reason: in.Reason, RevokeTokens: in.RevokeTokens,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}

func mcpKillSwitchList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		resp, err := c.ListKillSwitches(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		out := make([]McpKillSwitchEntry, 0, len(resp.GetEntries()))
		for _, e := range resp.GetEntries() {
			out = append(out, McpKillSwitchEntry{Scope: e.GetScope(), TargetID: e.GetTargetId(), Active: e.GetActive(), Reason: e.GetReason(), At: tsString(e.GetAt()), By: e.GetSetBy()})
		}
		return out, nil
	}
}
