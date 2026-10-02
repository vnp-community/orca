package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func validateParams(h *oauthHarness) OAuthAuthorizeParams {
	_, c := pkcePair("v")
	return OAuthAuthorizeParams{
		ResponseType: "code", ClientID: h.clientID, RedirectURI: testRedirectURI,
		CodeChallenge: c, CodeChallengeMethod: "S256", Resource: testResource,
	}
}

func TestValidateAuthorizeRequest(t *testing.T) {
	h := newOAuthHarness(t)
	uc := NewOAuthValidateAuthorizeRequest(h.repo, h.cfg)

	t.Run("valid defaults scope to orca:read", func(t *testing.T) {
		info, err := uc.Execute(context.Background(), validateParams(h))
		if err != nil {
			t.Fatal(err)
		}
		if len(info.Scopes) != 1 || info.Scopes[0] != domain.OAuthScopeRead {
			t.Fatalf("scopes = %v", info.Scopes)
		}
		if info.Client.ClientName != "Test App" {
			t.Fatalf("client = %+v", info.Client)
		}
	})

	cases := []struct {
		name   string
		mutate func(*OAuthAuthorizeParams)
		code   string
	}{
		{"PKCE challenge missing", func(p *OAuthAuthorizeParams) { p.CodeChallenge = "" }, CodeOAuthInvalidRequest},
		{"PKCE method plain", func(p *OAuthAuthorizeParams) { p.CodeChallengeMethod = "plain" }, CodeOAuthInvalidRequest},
		{"PKCE method missing", func(p *OAuthAuthorizeParams) { p.CodeChallengeMethod = "" }, CodeOAuthInvalidRequest},
		{"PKCE challenge too short", func(p *OAuthAuthorizeParams) { p.CodeChallenge = "abc" }, CodeOAuthInvalidRequest},
		{"resource missing", func(p *OAuthAuthorizeParams) { p.Resource = "" }, CodeOAuthInvalidTarget},
		{"resource other", func(p *OAuthAuthorizeParams) { p.Resource = "https://evil.example.com/mcp" }, CodeOAuthInvalidTarget},
		{"resource trailing slash is not exact", func(p *OAuthAuthorizeParams) { p.Resource = testResource + "/" }, CodeOAuthInvalidTarget},
		{"response_type token", func(p *OAuthAuthorizeParams) { p.ResponseType = "token" }, CodeOAuthUnsupportedResponse},
		{"unknown scope", func(p *OAuthAuthorizeParams) { p.Scope = "orca:read orca:nuke" }, CodeOAuthInvalidScope},
		{"unknown client", func(p *OAuthAuthorizeParams) { p.ClientID = "nope" }, CodeOAuthInvalidClient},
		{"missing client", func(p *OAuthAuthorizeParams) { p.ClientID = "" }, CodeOAuthInvalidClient},
		{"redirect one char off", func(p *OAuthAuthorizeParams) { p.RedirectURI = testRedirectURI + "x" }, CodeOAuthInvalidRedirectURI},
		{"redirect host one char off", func(p *OAuthAuthorizeParams) { p.RedirectURI = "https://client.example.org/callback" }, CodeOAuthInvalidRedirectURI},
		{"redirect missing", func(p *OAuthAuthorizeParams) { p.RedirectURI = "" }, CodeOAuthInvalidRedirectURI},
		{"redirect case differs", func(p *OAuthAuthorizeParams) { p.RedirectURI = "https://client.example.com/Callback" }, CodeOAuthInvalidRedirectURI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validateParams(h)
			tc.mutate(&p)
			_, err := uc.Execute(context.Background(), p)
			wantCode(t, err, tc.code)
		})
	}
}

// Client/redirect failures must be reported before PKCE errors so a hostile
// redirect_uri can never receive an error redirect.
func TestValidateAuthorizeRequest_ClientAndRedirectCheckedFirst(t *testing.T) {
	h := newOAuthHarness(t)
	uc := NewOAuthValidateAuthorizeRequest(h.repo, h.cfg)
	p := validateParams(h)
	p.RedirectURI = "https://evil.example.com/cb"
	p.CodeChallenge = ""
	_, err := uc.Execute(context.Background(), p)
	wantCode(t, err, CodeOAuthInvalidRedirectURI)
}

func TestValidateAuthorizeRequest_LoopbackPortMayVary(t *testing.T) {
	h := newOAuthHarness(t)
	h.repo.clients[h.clientID] = domain.OAuthClient{ClientID: h.clientID, ClientName: "CLI", RedirectURIs: []string{"http://127.0.0.1/cb"}}
	uc := NewOAuthValidateAuthorizeRequest(h.repo, h.cfg)
	p := validateParams(h)
	p.RedirectURI = "http://127.0.0.1:53211/cb"
	if _, err := uc.Execute(context.Background(), p); err != nil {
		t.Fatalf("loopback with other port should match: %v", err)
	}
	p.RedirectURI = "http://127.0.0.1:53211/other"
	_, err := uc.Execute(context.Background(), p)
	wantCode(t, err, CodeOAuthInvalidRedirectURI)
	p.RedirectURI = "http://localhost:53211/cb"
	_, err = uc.Execute(context.Background(), p)
	wantCode(t, err, CodeOAuthInvalidRedirectURI)
}

func TestIssueAuthCode_Guards(t *testing.T) {
	ctx := context.Background()
	base := func(h *oauthHarness) OAuthIssueAuthCodeInput {
		_, c := pkcePair("x")
		return OAuthIssueAuthCodeInput{
			TenantID: h.tenantID, UserID: h.user.ID, ClientID: h.clientID, RedirectURI: testRedirectURI,
			Scopes: []string{domain.OAuthScopeRead}, CodeChallenge: c, Resource: testResource, GrantID: "5d0f5a2c-84d4-4b4e-93a4-0a1d6c1c1111",
		}
	}
	cases := []struct {
		name   string
		mutate func(*oauthHarness, *OAuthIssueAuthCodeInput)
		code   string
	}{
		{"scope above role ceiling", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.Scopes = []string{domain.OAuthScopeAdmin} }, CodeOAuthInvalidScope},
		{"no scopes", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.Scopes = nil }, CodeOAuthInvalidScope},
		{"unknown scope", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.Scopes = []string{"x"} }, CodeOAuthInvalidScope},
		{"client blocked", func(h *oauthHarness, _ *OAuthIssueAuthCodeInput) { h.setStatus(domain.OAuthClientBlocked) }, CodeOAuthUnauthorizedClient},
		{"client pending", func(h *oauthHarness, _ *OAuthIssueAuthCodeInput) { h.setStatus(domain.OAuthClientPending) }, CodeOAuthUnauthorizedClient},
		{"client has no tenant row", func(h *oauthHarness, _ *OAuthIssueAuthCodeInput) {
			delete(h.repo.statuses, statusKey(h.tenantID, h.clientID))
		}, CodeOAuthUnauthorizedClient},
		{"redirect mismatch", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.RedirectURI += "/" }, CodeOAuthInvalidRedirectURI},
		{"plain-sized challenge", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.CodeChallenge = "short" }, CodeOAuthInvalidRequest},
		{"wrong resource", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.Resource = "https://x/mcp" }, CodeOAuthInvalidTarget},
		{"bad grant id", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.GrantID = "not-uuid" }, CodeOAuthInvalidRequest},
		{"no user context", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) { in.UserID = "" }, CodeOAuthAccessDenied},
		{"other tenant", func(_ *oauthHarness, in *OAuthIssueAuthCodeInput) {
			in.TenantID = "11111111-1111-1111-1111-111111111111"
		}, CodeOAuthAccessDenied},
		{"revoked grant", func(h *oauthHarness, in *OAuthIssueAuthCodeInput) { h.repo.grants[in.GrantID] = true }, CodeOAuthAccessDenied},
		{"inactive user", func(h *oauthHarness, _ *OAuthIssueAuthCodeInput) {
			u := h.user
			u.IsActive = false
			h.users.seed(u, "x")
		}, CodeOAuthAccessDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newOAuthHarness(t)
			in := base(h)
			tc.mutate(h, &in)
			_, err := h.issue.Execute(ctx, in)
			wantCode(t, err, tc.code)
		})
	}
}

func TestIssueAuthCode_AdminMayGrantAdminScope(t *testing.T) {
	h := newOAuthHarness(t)
	admin := h.user
	admin.Role = domain.RoleAdmin
	h.users.seed(admin, "x")
	a := h.authorize(domain.OAuthScopeAdmin)
	if a.code == "" {
		t.Fatal("no code")
	}
}

func TestIssueAuthCode_StoresOnlyHashOfCode(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	if _, ok := h.repo.codes[a.code]; ok {
		t.Fatal("raw code used as storage key")
	}
	if _, ok := h.repo.codes[hashToken(a.code)]; !ok {
		t.Fatal("code hash not stored")
	}
	for _, c := range h.repo.codes {
		if strings.Contains(c.CodeHash, a.code) {
			t.Fatal("raw code leaked into stored hash")
		}
	}
}
