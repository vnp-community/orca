package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcptokens"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// BE-MCP-SOL-005 section G / BE-MCP-SOL-006 section F channels. Every channel
// takes exactly one object in args[0] (CONTRACT C10), derives identity only
// from the session (C2) and returns CONTRACT camelCase JSON.

const mcpRPCTimeout = 5 * time.Second

// McpConsentRequest / McpGrant / McpOAuthClient mirror CONTRACT section 1.
type McpConsentRequest struct {
	RequestID        string               `json:"requestId"`
	ClientID         string               `json:"clientId"`
	ClientName       string               `json:"clientName"`
	ClientURI        string               `json:"clientUri,omitempty"`
	RedirectHost     string               `json:"redirectHost"`
	Scopes           []McpScopeDescriptor `json:"scopes"`
	AlreadyGranted   []string             `json:"alreadyGranted"`
	Tenant           McpTenantRef         `json:"tenant"`
	IsNewClient      bool                 `json:"isNewClient"`
	RegisteredViaDcr bool                 `json:"registeredViaDcr"`
	ExpiresAt        string               `json:"expiresAt"`
}

type McpTenantRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type McpGrant struct {
	ID         string   `json:"id"`
	ClientID   string   `json:"clientId"`
	ClientName string   `json:"clientName"`
	ClientURI  string   `json:"clientUri,omitempty"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"createdAt"`
	LastUsedAt string   `json:"lastUsedAt,omitempty"`
	Status     string   `json:"status"`
	UserID     string   `json:"userId,omitempty"`
	UserName   string   `json:"userName,omitempty"`
}

type McpOAuthClient struct {
	ClientID      string   `json:"clientId"`
	Name          string   `json:"name"`
	RedirectURIs  []string `json:"redirectUris"`
	RegisteredVia string   `json:"registeredVia"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"createdAt"`
	LastUsedAt    string   `json:"lastUsedAt,omitempty"`
	ActiveGrants  int      `json:"activeGrants"`
}

func registerMcpAuthorizationChannels(r *Registry, d McpChannelDeps) {
	r.Register("mcp.consent.get", mcpHandler(d, false, mcpConsentGet(d)))
	r.Register("mcp.consent.decide", mcpHandler(d, false, mcpConsentDecide(d)))
	r.Register("mcp.grant.list", mcpHandler(d, false, mcpGrantList(d, false)))
	r.Register("mcp.grant.revoke", mcpHandler(d, false, mcpGrantRevoke(d, false)))
	r.Register("mcp.admin.grant.list", mcpHandler(d, true, mcpGrantList(d, true)))
	r.Register("mcp.admin.grant.revoke", mcpHandler(d, true, mcpGrantRevoke(d, true)))
	r.Register("mcp.admin.client.list", mcpHandler(d, true, mcpClientList(d)))
	r.Register("mcp.admin.client.setStatus", mcpHandler(d, true, mcpClientSetStatus(d)))
	r.Register("mcp.token.list", mcpHandler(d, false, mcpTokenList(d)))
	r.Register("mcp.token.create", mcpHandler(d, false, mcpTokenCreate(d)))
	r.Register("mcp.token.revoke", mcpHandler(d, false, mcpTokenRevoke(d)))
}

// mcpArgs decodes the single args[0] object; a missing or malformed argument
// is a CONTRACT error, never the raw decoder message.
func mcpArgs[T any](args []json.RawMessage) (T, error) {
	var v T
	if len(args) < 1 {
		return v, errors.New("MCP_INVALID_ARGUMENT: expected one object argument")
	}
	if err := json.Unmarshal(args[0], &v); err != nil {
		return v, errors.New("MCP_INVALID_ARGUMENT: argument is not a valid object")
	}
	return v, nil
}

// mcpCtx only bounds the call: Registry.Dispatch already attached the
// session identity (tenant, user, role) as outgoing gRPC metadata.
func mcpCtx(ctx context.Context, _ Identity) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, mcpRPCTimeout)
}

func mcpClient(d McpChannelDeps) (mcpv1.McpServiceClient, error) {
	if d.Client == nil {
		return nil, errors.New("MCP_UNAVAILABLE: mcp-service is not configured")
	}
	return d.Client, nil
}

func tsString(t *timestamppb.Timestamp) string {
	if t == nil || (t.GetSeconds() == 0 && t.GetNanos() == 0) {
		return ""
	}
	return t.AsTime().UTC().Format(time.RFC3339)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func mcpConsentGet(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			RequestID string `json:"requestId"`
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
		v, err := c.GetConsentRequest(ctx, &mcpv1.GetConsentRequestRequest{RequestId: in.RequestID})
		if err != nil {
			return nil, err
		}
		scopes := make([]McpScopeDescriptor, 0, len(v.GetScopes()))
		for _, s := range v.GetScopes() {
			scopes = append(scopes, McpScopeDescriptor{ID: s.GetId(), Label: s.GetLabel(), Description: s.GetDescription(), Risk: s.GetRisk()})
		}
		return McpConsentRequest{
			RequestID: v.GetRequestId(), ClientID: v.GetClientId(), ClientName: v.GetClientName(), ClientURI: v.GetClientUri(),
			RedirectHost: v.GetRedirectHost(), Scopes: scopes, AlreadyGranted: nonNil(v.GetAlreadyGranted()),
			Tenant:      McpTenantRef{ID: v.GetTenantId(), Name: tenantDisplayName(ctx, d.Tenant, v.GetTenantId())},
			IsNewClient: v.GetIsNewClient(), RegisteredViaDcr: v.GetRegisteredViaDcr(), ExpiresAt: tsString(v.GetExpiresAt()),
		}, nil
	}
}

// tenantDisplayName resolves the organization name; any failure falls back to
// the id so the consent screen is never blocked by tenant-service.
func tenantDisplayName(ctx context.Context, c tenantv1.TenantServiceClient, tenantID string) string {
	if c == nil || tenantID == "" {
		return tenantID
	}
	resp, err := c.GetCompany(ctx, &tenantv1.GetCompanyRequest{Id: tenantID})
	if err != nil || resp.GetCompany().GetName() == "" {
		return tenantID
	}
	return resp.GetCompany().GetName()
}

func mcpConsentDecide(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			RequestID string   `json:"requestId"`
			Decision  string   `json:"decision"`
			Scopes    []string `json:"scopes"`
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
		resp, err := c.DecideConsent(ctx, &mcpv1.DecideConsentRequest{RequestId: in.RequestID, Decision: in.Decision, Scopes: in.Scopes})
		if err != nil {
			return nil, err
		}
		return map[string]string{"redirectUrl": resp.GetRedirectUrl()}, nil
	}
}

func toMcpGrant(g *mcpv1.Grant, admin bool) McpGrant {
	out := McpGrant{
		ID: g.GetId(), ClientID: g.GetClientId(), ClientName: g.GetClientName(), ClientURI: g.GetClientUri(),
		Scopes: nonNil(g.GetScopes()), CreatedAt: tsString(g.GetCreatedAt()), LastUsedAt: tsString(g.GetLastUsedAt()), Status: g.GetStatus(),
	}
	if admin { // CONTRACT: userId/userName only on the admin channel
		out.UserID, out.UserName = g.GetUserId(), g.GetUserName()
	}
	return out
}

func mcpGrantList(d McpChannelDeps, admin bool) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		req := &mcpv1.ListGrantsRequest{AllUsers: admin}
		if admin {
			in := decodeOptionalArg[struct {
				UserID string `json:"userId"`
			}](args, 0)
			req.UserId = in.UserID
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ListGrants(ctx, req)
		if err != nil {
			return nil, err
		}
		out := make([]McpGrant, 0, len(resp.GetGrants()))
		for _, g := range resp.GetGrants() {
			out = append(out, toMcpGrant(g, admin))
		}
		return out, nil
	}
}

func mcpGrantRevoke(d McpChannelDeps, admin bool) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			GrantID string `json:"grantId"`
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
		if _, err := c.RevokeGrant(ctx, &mcpv1.RevokeGrantRequest{GrantId: in.GrantID, Admin: admin}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}

func toMcpClient(c *mcpv1.OAuthClient) McpOAuthClient {
	return McpOAuthClient{
		ClientID: c.GetClientId(), Name: c.GetName(), RedirectURIs: nonNil(c.GetRedirectUris()), RegisteredVia: c.GetRegisteredVia(),
		Status: c.GetStatus(), CreatedAt: tsString(c.GetCreatedAt()), LastUsedAt: tsString(c.GetLastUsedAt()), ActiveGrants: int(c.GetActiveGrants()),
	}
}

func mcpClientList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ListOAuthClients(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		out := make([]McpOAuthClient, 0, len(resp.GetClients()))
		for _, cl := range resp.GetClients() {
			out = append(out, toMcpClient(cl))
		}
		return out, nil
	}
}

func mcpClientSetStatus(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			ClientID string `json:"clientId"`
			Status   string `json:"status"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.Status != "allowed" && in.Status != "blocked" {
			return nil, errors.New("MCP_INVALID_ARGUMENT: status must be allowed or blocked")
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		cl, err := c.SetOAuthClientStatus(ctx, &mcpv1.SetOAuthClientStatusRequest{ClientId: in.ClientID, Status: in.Status})
		if err != nil {
			return nil, err
		}
		return toMcpClient(cl), nil
	}
}

// --- personal access tokens (BE-MCP-SOL-006) ---

func tokenService(d McpChannelDeps) error {
	if d.Tokens == nil {
		return errors.New("MCP_UNAVAILABLE: token service is not configured")
	}
	return nil
}

func mcpTokenList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		if err := tokenService(d); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, mcpRPCTimeout)
		defer cancel()
		return d.Tokens.List(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
	}
}

// mcpTokenCreate returns the one-time secret. Neither this handler nor the
// registry logs args or results, and the secret is not retained anywhere.
func mcpTokenCreate(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if err := tokenService(d); err != nil {
			return nil, err
		}
		in, err := mcpArgs[struct {
			Name          string   `json:"name"`
			Scopes        []string `json:"scopes"`
			ExpiresInDays int      `json:"expiresInDays"`
		}](args)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, mcpRPCTimeout)
		defer cancel()
		return d.Tokens.Create(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role},
			mcptokens.CreateInput{Name: in.Name, Scopes: in.Scopes, ExpiresInDays: in.ExpiresInDays})
	}
}

func mcpTokenRevoke(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if err := tokenService(d); err != nil {
			return nil, err
		}
		in, err := mcpArgs[struct {
			TokenID string `json:"tokenId"`
		}](args)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, mcpRPCTimeout)
		defer cancel()
		if err := d.Tokens.Revoke(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role}, in.TokenID); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}
