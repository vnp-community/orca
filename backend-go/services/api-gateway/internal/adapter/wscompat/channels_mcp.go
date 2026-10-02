package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcptokens"
)

// McpChannelDeps feeds the mcp.* WS channels (CONTRACT section 2). Later MCP
// solutions add their channels in their own channels_mcp_<group>.go files and
// call them from RegisterMcpChannels, always wrapped with mcpHandler.
type McpChannelDeps struct {
	// Enabled is the PROCESS flag MCP_ENABLED (not the per-tenant switch).
	Enabled bool
	// Client is nil when MCP is disabled or MCP_SERVICE_ADDR is unset.
	Client mcpv1.McpServiceClient
	// Registry backs mcp.externalServer.* (BE-MCP-SOL-014); nil answers MCP_UNAVAILABLE.
	Registry mcpv1.McpRegistryServiceClient
	// ResourceURL / AuthorizationServer come from config (MCP_PUBLIC_BASE_URL,
	// MCP_ISSUER_URL); empty when MCP is disabled.
	ResourceURL         string
	AuthorizationServer string
	// Tenant resolves the organization name shown on the consent screen
	// (optional: the tenant id is shown when nil or failing).
	Tenant tenantv1.TenantServiceClient
	// Tokens backs mcp.token.* (nil answers MCP_UNAVAILABLE).
	Tokens *mcptokens.Service
	// ToolCatalog backs mcp.admin.tool.list (nil answers MCP_UNAVAILABLE).
	ToolCatalog McpToolLister
	// Audit backs mcp.admin.audit.query (auth-service client; nil answers MCP_UNAVAILABLE).
	Audit McpAuditQuerier
	// SessionCloser, when set, makes mcp.session.close also end the live
	// session on every gateway replica (BE-MCP-SOL-004).
	SessionCloser McpSessionCloser
}

// RegisterMcpChannels registers every mcp.* channel that has a real
// implementation. Channels without an owning solution are deliberately NOT
// stubbed: they fall through to notImplementedHandler (CONTRACT/SOL-002).
func RegisterMcpChannels(r *Registry, d McpChannelDeps) {
	registerMcpServerInfo(r, d)
	registerMcpAuthorizationChannels(r, d)
	registerMcpToolCatalogChannel(r, d)
	registerMcpGovernanceChannels(r, d)
	registerMcpApprovalChannels(r, d)
	registerMcpAuditChannel(r, d)
	registerMcpEventsChannel(r, d)
	registerMcpExternalServerChannels(r, d)
	registerMcpSessionChannels(r, d)
	registerMcpPromptChannels(r, d)
}

// mcpHandler gates and normalizes one mcp.* channel: process-level disable
// (C8), admin-only (C3, fail-closed on empty role) and error shaping (C4).
// mcp.server.info is the one channel that must NOT use it.
func mcpHandler(d McpChannelDeps, adminOnly bool, fn ChannelHandler) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if !d.Enabled {
			return nil, errors.New("MCP_DISABLED: MCP is disabled on this server")
		}
		if adminOnly && id.Role != "admin" {
			return nil, errors.New("MCP_NOT_ADMIN: admin role required")
		}
		out, err := fn(ctx, id, args)
		return out, mcpChannelError(err)
	}
}

var (
	mcpCodedMessage  = regexp.MustCompile(`^MCP_[A-Z0-9_]+: .+`)
	grpcErrorWrapper = regexp.MustCompile(`rpc error: code = [A-Za-z]+ desc = `)
)

// mcpChannelError guarantees "<CODE>: <message>" (CONTRACT C4). It strips the
// "rpc error: code = ... desc = " envelope, passes MCP_* coded messages
// through, and maps everything else to a generic coded message so no internal
// detail (service names, SQL, stacks) or request argument (C11) can leak.
func mcpChannelError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if st, ok := status.FromError(err); ok {
		msg = st.Message()
	}
	// Keep only what follows the innermost "rpc error: code = X desc = ", which
	// also drops any "wrap: " prefix added by callers' %w chains.
	if locs := grpcErrorWrapper.FindAllStringIndex(msg, -1); len(locs) > 0 {
		msg = msg[locs[len(locs)-1][1]:]
	}
	msg = strings.TrimSpace(msg)
	if mcpCodedMessage.MatchString(msg) {
		return errors.New(msg)
	}
	code := status.Code(err)
	switch {
	case code == codes.Unavailable:
		return errors.New("MCP_UNAVAILABLE: mcp-service temporarily unavailable")
	case code == codes.DeadlineExceeded, errors.Is(err, context.DeadlineExceeded):
		return errors.New("MCP_TIMEOUT: mcp-service did not respond in time")
	case code == codes.NotFound, code == codes.PermissionDenied:
		// Never distinguish "missing" from "not yours".
		return errors.New("MCP_NOT_FOUND: not found")
	case code == codes.InvalidArgument:
		return errors.New("MCP_INVALID_ARGUMENT: invalid argument")
	default:
		return errors.New("MCP_INTERNAL: internal error")
	}
}

// McpServerInfo / McpScopeDescriptor / McpKillSwitch mirror CONTRACT section 1.
type McpServerInfo struct {
	Enabled             bool                 `json:"enabled"`
	TenantEnabled       *bool                `json:"tenantEnabled,omitempty"`
	ResourceURL         string               `json:"resourceUrl"`
	ProtocolVersions    []string             `json:"protocolVersions"`
	AuthorizationServer string               `json:"authorizationServer"`
	ScopesSupported     []McpScopeDescriptor `json:"scopesSupported"`
	DCREnabled          bool                 `json:"dcrEnabled"`
	MaxTokenDays        int                  `json:"maxTokenDays"`
	KillSwitch          McpKillSwitch        `json:"killSwitch"`
}

type McpScopeDescriptor struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

type McpKillSwitch struct {
	Active bool   `json:"active"`
	Reason string `json:"reason,omitempty"`
	At     string `json:"at,omitempty"`
}

const mcpServerInfoTimeout = 5 * time.Second

// registerMcpServerInfo registers mcp.server.info WITHOUT mcpHandler gating:
// the UI calls it before it knows whether MCP is on, so "off" is a normal
// answer ({enabled:false}), never an error (C8).
func registerMcpServerInfo(r *Registry, d McpChannelDeps) {
	r.Register("mcp.server.info", func(ctx context.Context, _ Identity, _ []json.RawMessage) (any, error) {
		if !d.Enabled {
			return McpServerInfo{ProtocolVersions: []string{}, ScopesSupported: []McpScopeDescriptor{}}, nil
		}
		if d.Client == nil {
			return nil, errors.New("MCP_UNAVAILABLE: mcp-service is not configured")
		}
		ctx, cancel := context.WithTimeout(ctx, mcpServerInfoTimeout)
		defer cancel()
		resp, err := d.Client.GetServerInfo(ctx, &mcpv1.GetServerInfoRequest{})
		if err != nil {
			return nil, mcpChannelError(err)
		}
		tenantEnabled := resp.GetEnabled()
		scopes := make([]McpScopeDescriptor, 0, len(resp.GetScopes()))
		for _, s := range resp.GetScopes() {
			scopes = append(scopes, McpScopeDescriptor{ID: s.GetId(), Label: s.GetLabel(), Description: s.GetDescription(), Risk: s.GetRisk()})
		}
		ks := McpKillSwitch{Active: resp.GetKillSwitch().GetActive(), Reason: resp.GetKillSwitch().GetReason()}
		if at := resp.GetKillSwitch().GetAt(); at.IsValid() && (at.GetSeconds() != 0 || at.GetNanos() != 0) {
			ks.At = at.AsTime().UTC().Format(time.RFC3339)
		}
		return McpServerInfo{
			Enabled:             true,
			TenantEnabled:       &tenantEnabled,
			ResourceURL:         d.ResourceURL,
			ProtocolVersions:    append([]string{}, mcpserver.SupportedProtocolVersions...),
			AuthorizationServer: d.AuthorizationServer,
			ScopesSupported:     scopes,
			DCREnabled:          resp.GetDcrEnabled(),
			MaxTokenDays:        int(resp.GetMaxTokenDays()),
			KillSwitch:          ks,
		}, nil
	})
}
