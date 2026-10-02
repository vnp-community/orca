package usecase

import (
	"context"
	"strings"
	"testing"
)

func registerInput() OAuthRegisterClientInput {
	return OAuthRegisterClientInput{
		ClientName: "Claude Code", ClientURI: "https://claude.example.com",
		RedirectURIs:            []string{"http://127.0.0.1:0/callback", "https://app.example.com/cb"},
		TokenEndpointAuthMethod: "none",
	}
}

func newRegister(h *oauthHarness, mutate func(*OAuthConfig)) *OAuthRegisterClient {
	cfg := h.cfg
	if mutate != nil {
		mutate(&cfg)
	}
	return NewOAuthRegisterClient(h.repo, h.audit, h.clock, cfg)
}

func TestRegisterClient_PublicClientOnly(t *testing.T) {
	h := newOAuthHarness(t)
	out, err := newRegister(h, nil).Execute(context.Background(), registerInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Client.ClientID) < 40 || out.Client.RegisteredVia != "dcr" {
		t.Fatalf("client = %+v", out.Client)
	}
	if len(out.GrantTypes) != 2 || out.ResponseTypes[0] != "code" {
		t.Fatalf("defaults = %v %v", out.GrantTypes, out.ResponseTypes)
	}
	if !hasAudit(h, "oauth.client_registered") {
		t.Fatal("missing audit entry")
	}
}

func TestRegisterClient_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*OAuthRegisterClientInput)
		code   string
	}{
		{"confidential auth method", func(in *OAuthRegisterClientInput) { in.TokenEndpointAuthMethod = "client_secret_basic" }, CodeOAuthInvalidClientMetadata},
		{"password grant", func(in *OAuthRegisterClientInput) { in.GrantTypes = []string{"password"} }, CodeOAuthInvalidClientMetadata},
		{"implicit grant", func(in *OAuthRegisterClientInput) { in.GrantTypes = []string{"implicit"} }, CodeOAuthInvalidClientMetadata},
		{"refresh without code grant", func(in *OAuthRegisterClientInput) { in.GrantTypes = []string{"refresh_token"} }, CodeOAuthInvalidClientMetadata},
		{"response_type token", func(in *OAuthRegisterClientInput) { in.ResponseTypes = []string{"token"} }, CodeOAuthInvalidClientMetadata},
		{"no redirect uris", func(in *OAuthRegisterClientInput) { in.RedirectURIs = nil }, CodeOAuthInvalidClientMetadata},
		{"six redirect uris", func(in *OAuthRegisterClientInput) {
			in.RedirectURIs = []string{"https://a/1", "https://a/2", "https://a/3", "https://a/4", "https://a/5", "https://a/6"}
		}, CodeOAuthInvalidClientMetadata},
		{"http non-loopback", func(in *OAuthRegisterClientInput) { in.RedirectURIs = []string{"http://evil.example.com/cb"} }, CodeOAuthInvalidRedirectURI},
		{"wildcard", func(in *OAuthRegisterClientInput) { in.RedirectURIs = []string{"https://*.example.com/cb"} }, CodeOAuthInvalidRedirectURI},
		{"fragment", func(in *OAuthRegisterClientInput) { in.RedirectURIs = []string{"https://a.example.com/cb#x"} }, CodeOAuthInvalidRedirectURI},
		{"custom scheme", func(in *OAuthRegisterClientInput) { in.RedirectURIs = []string{"myapp://cb"} }, CodeOAuthInvalidRedirectURI},
		{"empty name", func(in *OAuthRegisterClientInput) { in.ClientName = "  \x00 " }, CodeOAuthInvalidClientMetadata},
		{"name is a url", func(in *OAuthRegisterClientInput) { in.ClientName = "visit https://evil.example.com" }, CodeOAuthInvalidClientMetadata},
		{"name too long", func(in *OAuthRegisterClientInput) { in.ClientName = strings.Repeat("a", 101) }, CodeOAuthInvalidClientMetadata},
		{"client_uri http", func(in *OAuthRegisterClientInput) { in.ClientURI = "http://example.com" }, CodeOAuthInvalidClientMetadata},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newOAuthHarness(t)
			in := registerInput()
			tc.mutate(&in)
			_, err := newRegister(h, nil).Execute(context.Background(), in)
			wantCode(t, err, tc.code)
		})
	}
}

func TestRegisterClient_NameControlCharsStripped(t *testing.T) {
	h := newOAuthHarness(t)
	in := registerInput()
	in.ClientName = "My\x00 ‮App\n"
	out, err := newRegister(h, nil).Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Client.ClientName != "My App" {
		t.Fatalf("name = %q", out.Client.ClientName)
	}
}

func TestRegisterClient_DisabledAndCap(t *testing.T) {
	h := newOAuthHarness(t)
	_, err := newRegister(h, func(c *OAuthConfig) { c.DCREnabled = false }).Execute(context.Background(), registerInput())
	wantCode(t, err, CodeOAuthDCRDisabled)

	uc := newRegister(h, func(c *OAuthConfig) { c.DCRMaxClients = 3 })
	for i := 0; i < 2; i++ { // harness already holds one client
		if _, err := uc.Execute(context.Background(), registerInput()); err != nil {
			t.Fatalf("register #%d: %v", i, err)
		}
	}
	_, err = uc.Execute(context.Background(), registerInput())
	wantCode(t, err, CodeOAuthDCRLimitReached)
}

func TestEnsureClientForTenant_StatusFollowsDCRSetting(t *testing.T) {
	h := newOAuthHarness(t)
	ctx := context.Background()
	uc := NewOAuthEnsureClientForTenant(h.repo, h.clock)
	reg, err := newRegister(h, nil).Execute(ctx, registerInput())
	if err != nil {
		t.Fatal(err)
	}
	id := reg.Client.ClientID

	v, err := uc.Execute(ctx, OAuthEnsureClientForTenantInput{TenantID: "t-closed", ClientID: id, DCREnabled: false})
	if err != nil || v.Status.Status != "pending" {
		t.Fatalf("closed tenant: %v %+v", err, v.Status)
	}
	v, err = uc.Execute(ctx, OAuthEnsureClientForTenantInput{TenantID: "t-open", ClientID: id, DCREnabled: true})
	if err != nil || v.Status.Status != "allowed" {
		t.Fatalf("open tenant: %v %+v", err, v.Status)
	}
	// An existing row is never rewritten: a blocked client stays blocked.
	h.repo.statuses[statusKey("t-open", id)] = blockedStatus("t-open", id)
	v, _ = uc.Execute(ctx, OAuthEnsureClientForTenantInput{TenantID: "t-open", ClientID: id, DCREnabled: true})
	if v.Status.Status != "blocked" {
		t.Fatalf("ensure overwrote a block: %+v", v.Status)
	}
	_, err = uc.Execute(ctx, OAuthEnsureClientForTenantInput{TenantID: "t", ClientID: "ghost"})
	wantCode(t, err, CodeOAuthInvalidClient)
}

func TestListClientsForTenant_OnlyThatTenant(t *testing.T) {
	h := newOAuthHarness(t)
	h.repo.statuses[statusKey("other", h.clientID)] = blockedStatus("other", h.clientID)
	views, err := NewOAuthListClientsForTenant(h.repo).Execute(context.Background(), h.tenantID)
	if err != nil || len(views) != 1 || views[0].Status.TenantID != h.tenantID {
		t.Fatalf("views = %+v err = %v", views, err)
	}
}
