package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcptokens"
)

type authzMcpClient struct {
	mcpv1.McpServiceClient
	consent    *mcpv1.ConsentRequestView
	consentErr error
	decideReq  *mcpv1.DecideConsentRequest
	grantsReq  *mcpv1.ListGrantsRequest
	revokeReq  *mcpv1.RevokeGrantRequest
	setReq     *mcpv1.SetOAuthClientStatusRequest
	lastMD     metadata.MD
	calls      int
}

func (f *authzMcpClient) rec(ctx context.Context) {
	f.calls++
	f.lastMD, _ = metadata.FromOutgoingContext(ctx)
}

func (f *authzMcpClient) GetConsentRequest(ctx context.Context, _ *mcpv1.GetConsentRequestRequest, _ ...grpc.CallOption) (*mcpv1.ConsentRequestView, error) {
	f.rec(ctx)
	return f.consent, f.consentErr
}
func (f *authzMcpClient) DecideConsent(ctx context.Context, in *mcpv1.DecideConsentRequest, _ ...grpc.CallOption) (*mcpv1.DecideConsentResponse, error) {
	f.rec(ctx)
	f.decideReq = in
	return &mcpv1.DecideConsentResponse{RedirectUrl: "https://app.example/cb?code=C&state=S&iss=I"}, nil
}
func (f *authzMcpClient) ListGrants(ctx context.Context, in *mcpv1.ListGrantsRequest, _ ...grpc.CallOption) (*mcpv1.ListGrantsResponse, error) {
	f.rec(ctx)
	f.grantsReq = in
	return &mcpv1.ListGrantsResponse{Grants: []*mcpv1.Grant{{
		Id: "g1", ClientId: "c1", ClientName: "App", Scopes: []string{"orca:read"}, Status: "active",
		CreatedAt: timestamppb.New(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)), UserId: "u9", UserName: "Una",
	}}}, nil
}
func (f *authzMcpClient) RevokeGrant(ctx context.Context, in *mcpv1.RevokeGrantRequest, _ ...grpc.CallOption) (*mcpv1.RevokeGrantResponse, error) {
	f.rec(ctx)
	f.revokeReq = in
	return &mcpv1.RevokeGrantResponse{}, nil
}
func (f *authzMcpClient) ListOAuthClients(ctx context.Context, _ *emptypb.Empty, _ ...grpc.CallOption) (*mcpv1.ListOAuthClientsResponse, error) {
	f.rec(ctx)
	return &mcpv1.ListOAuthClientsResponse{Clients: []*mcpv1.OAuthClient{{ClientId: "c1", Name: "App", RedirectUris: []string{"https://app.example/cb"}, RegisteredVia: "dcr", Status: "allowed", ActiveGrants: 2}}}, nil
}
func (f *authzMcpClient) SetOAuthClientStatus(ctx context.Context, in *mcpv1.SetOAuthClientStatusRequest, _ ...grpc.CallOption) (*mcpv1.OAuthClient, error) {
	f.rec(ctx)
	f.setReq = in
	return &mcpv1.OAuthClient{ClientId: in.GetClientId(), Name: "App", Status: in.GetStatus()}, nil
}

type fakeCompanyClient struct {
	tenantv1.TenantServiceClient
	name string
	err  error
}

func (f fakeCompanyClient) GetCompany(context.Context, *tenantv1.GetCompanyRequest, ...grpc.CallOption) (*tenantv1.GetCompanyResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &tenantv1.GetCompanyResponse{Company: &tenantv1.Company{Id: "t1", Name: f.name}}, nil
}

func callMcp(t *testing.T, d McpChannelDeps, role, channel string, arg any) (string, error) {
	t.Helper()
	r := NewRegistry()
	RegisterMcpChannels(r, d)
	var args []json.RawMessage
	if arg != nil {
		b, _ := json.Marshal(arg)
		args = []json.RawMessage{b}
	}
	out, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", Role: role}, channel, args)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func TestMcpConsentGet_FillsTenantNameAndFallsBackToID(t *testing.T) {
	view := &mcpv1.ConsentRequestView{
		RequestId: "r1", ClientId: "c1", ClientName: "App", RedirectHost: "app.example", TenantId: "t1", IsNewClient: true,
		Scopes:    []*mcpv1.ScopeDescriptor{{Id: "orca:read", Label: "Read", Description: "d", Risk: "read"}},
		ExpiresAt: timestamppb.New(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)),
	}
	c := &authzMcpClient{consent: view}
	d := McpChannelDeps{Enabled: true, Client: c, Tenant: fakeCompanyClient{name: "Acme Inc"}}
	got, err := callMcp(t, d, "user", "mcp.consent.get", map[string]string{"requestId": "r1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"requestId":"r1","clientId":"c1","clientName":"App","redirectHost":"app.example","scopes":[{"id":"orca:read","label":"Read","description":"d","risk":"read"}],` +
		`"alreadyGranted":[],"tenant":{"id":"t1","name":"Acme Inc"},"isNewClient":true,"registeredViaDcr":false,"expiresAt":"2026-10-01T12:00:00Z"}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if ids := c.lastMD.Get("x-orca-user-id"); len(ids) != 1 || ids[0] != "u1" {
		t.Fatalf("identity not forwarded: %v", c.lastMD)
	}
	d.Tenant = fakeCompanyClient{err: status.Error(codes.Unavailable, "down")}
	got, err = callMcp(t, d, "user", "mcp.consent.get", map[string]string{"requestId": "r1"})
	if err != nil || !strings.Contains(got, `"tenant":{"id":"t1","name":"t1"}`) {
		t.Fatalf("tenant lookup failure must not fail the consent screen: %v %s", err, got)
	}
}

func TestMcpConsentErrorsKeepCodes(t *testing.T) {
	for _, code := range []string{"MCP_CONSENT_NOT_FOUND", "MCP_CONSENT_EXPIRED"} {
		c := &authzMcpClient{consentErr: status.Error(codes.NotFound, code+": gone")}
		_, err := callMcp(t, McpChannelDeps{Enabled: true, Client: c}, "user", "mcp.consent.get", map[string]string{"requestId": "x"})
		if err == nil || err.Error() != code+": gone" {
			t.Errorf("%s: %v", code, err)
		}
	}
}

func TestMcpConsentDecide_SingleObjectArg(t *testing.T) {
	c := &authzMcpClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	got, err := callMcp(t, d, "user", "mcp.consent.decide", map[string]any{"requestId": "r1", "decision": "approve", "scopes": []string{"orca:read"}})
	if err != nil || !strings.Contains(got, `"redirectUrl":"https://app.example/cb?code=C`) {
		t.Fatalf("%v %s", err, got)
	}
	if c.decideReq.GetRequestId() != "r1" || c.decideReq.GetDecision() != "approve" || len(c.decideReq.GetScopes()) != 1 {
		t.Fatalf("req = %+v", c.decideReq)
	}
	// Missing or non-object args are a coded error, not a decoder message.
	for _, arg := range []any{nil, "just-a-string", []string{"r1", "approve"}} {
		if _, err := callMcp(t, d, "user", "mcp.consent.decide", arg); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT: ") {
			t.Errorf("arg %v: %v", arg, err)
		}
	}
}

func TestMcpGrantChannels_UserVsAdminShape(t *testing.T) {
	c := &authzMcpClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	own, err := callMcp(t, d, "user", "mcp.grant.list", nil)
	if err != nil || strings.Contains(own, "userId") || strings.Contains(own, "userName") || c.grantsReq.GetAllUsers() {
		t.Fatalf("own list must be self-scoped without user fields: %v %s %+v", err, own, c.grantsReq)
	}
	adm, err := callMcp(t, d, "admin", "mcp.admin.grant.list", map[string]string{"userId": "u9"})
	if err != nil || !strings.Contains(adm, `"userId":"u9"`) || !strings.Contains(adm, `"userName":"Una"`) || !c.grantsReq.GetAllUsers() || c.grantsReq.GetUserId() != "u9" {
		t.Fatalf("admin list: %v %s %+v", err, adm, c.grantsReq)
	}
	if _, err := callMcp(t, d, "user", "mcp.grant.revoke", map[string]string{"grantId": "g1"}); err != nil || c.revokeReq.GetAdmin() || c.revokeReq.GetGrantId() != "g1" {
		t.Fatalf("revoke: %v %+v", err, c.revokeReq)
	}
	if _, err := callMcp(t, d, "admin", "mcp.admin.grant.revoke", map[string]string{"grantId": "g2"}); err != nil || !c.revokeReq.GetAdmin() {
		t.Fatalf("admin revoke: %v %+v", err, c.revokeReq)
	}
}

func TestMcpAdminChannelsRequireAdmin(t *testing.T) {
	c := &authzMcpClient{}
	d := McpChannelDeps{Enabled: true, Client: c}
	for ch, arg := range map[string]any{
		"mcp.admin.client.list": nil, "mcp.admin.client.setStatus": map[string]string{"clientId": "c1", "status": "blocked"},
		"mcp.admin.grant.list": nil, "mcp.admin.grant.revoke": map[string]string{"grantId": "g"},
	} {
		for _, role := range []string{"user", ""} {
			if _, err := callMcp(t, d, role, ch, arg); err == nil || !strings.HasPrefix(err.Error(), "MCP_NOT_ADMIN: ") {
				t.Errorf("%s as %q: %v", ch, role, err)
			}
		}
	}
	if c.calls != 0 {
		t.Fatal("non-admins must never reach mcp-service")
	}
	got, err := callMcp(t, d, "admin", "mcp.admin.client.list", nil)
	if err != nil || !strings.Contains(got, `"clientId":"c1"`) || !strings.Contains(got, `"activeGrants":2`) || !strings.Contains(got, `"registeredVia":"dcr"`) {
		t.Fatalf("%v %s", err, got)
	}
	got, err = callMcp(t, d, "admin", "mcp.admin.client.setStatus", map[string]string{"clientId": "c1", "status": "blocked"})
	if err != nil || !strings.Contains(got, `"status":"blocked"`) || c.setReq.GetStatus() != "blocked" {
		t.Fatalf("%v %s", err, got)
	}
	if _, err := callMcp(t, d, "admin", "mcp.admin.client.setStatus", map[string]string{"clientId": "c1", "status": "pending"}); err == nil || !strings.HasPrefix(err.Error(), "MCP_INVALID_ARGUMENT") {
		t.Fatalf("pending is not settable: %v", err)
	}
}

func TestMcpAuthorizationChannelsDisabled(t *testing.T) {
	for _, ch := range []string{"mcp.consent.get", "mcp.grant.list", "mcp.admin.client.list", "mcp.token.list", "mcp.token.create", "mcp.token.revoke"} {
		_, err := callMcp(t, McpChannelDeps{Enabled: false}, "admin", ch, map[string]string{})
		if err == nil || !strings.HasPrefix(err.Error(), "MCP_DISABLED: ") {
			t.Errorf("%s: %v", ch, err)
		}
	}
}

func TestMcpTokenChannels(t *testing.T) {
	a := &tokenChanAuth{}
	svc := &mcptokens.Service{Auth: a, Policy: tokenChanPolicy{}}
	d := McpChannelDeps{Enabled: true, Tokens: svc}
	got, err := callMcp(t, d, "user", "mcp.token.create", map[string]any{"name": "ci", "scopes": []string{"orca:read"}, "expiresInDays": 30})
	if err != nil || !strings.Contains(got, `"secret":"omp_S3CR3T"`) || !strings.Contains(got, `"token":{"id":"j1"`) {
		t.Fatalf("create: %v %s", err, got)
	}
	got, err = callMcp(t, d, "user", "mcp.token.list", nil)
	if err != nil || strings.Contains(got, "omp_") || !strings.Contains(got, `"id":"j1"`) {
		t.Fatalf("list: %v %s", err, got)
	}
	if got, err = callMcp(t, d, "user", "mcp.token.revoke", map[string]string{"tokenId": "j1"}); err != nil || got != `{"ok":true}` {
		t.Fatalf("revoke: %v %s", err, got)
	}
	_, err = callMcp(t, d, "user", "mcp.token.create", map[string]any{"name": "ci", "scopes": []string{"orca:read"}, "expiresInDays": 999})
	if err == nil || err.Error() != "MCP_TOKEN_TOO_LONG: maximum is 90 days for this organization" {
		t.Fatalf("too long: %v", err)
	}
	if _, err = callMcp(t, McpChannelDeps{Enabled: true}, "user", "mcp.token.list", nil); err == nil || !strings.HasPrefix(err.Error(), "MCP_UNAVAILABLE") {
		t.Fatalf("no service: %v", err)
	}
}
