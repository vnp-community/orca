package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

func TestExchangeCode_IssuesAudienceBoundShortLivedToken(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	out := h.tokens(a)

	c := h.signer.last
	if len(c.Audience) != 1 || c.Audience[0] != testResource {
		t.Fatalf("aud = %v, want exactly [%s]", c.Audience, testResource)
	}
	if c.Issuer != jwtauth.Issuer || c.Subject != h.user.ID || c.TenantID != h.tenantID {
		t.Fatalf("identity claims wrong: %+v", c)
	}
	if c.TokenUse != "mcp_oauth" || c.ClientID != h.clientID || c.GrantID != a.grantID || c.FamilyID == "" || c.ID == "" {
		t.Fatalf("mcp claims wrong: %+v", c)
	}
	if c.Role != "" {
		t.Fatal("role must not be baked into an MCP token")
	}
	if c.Scope != "orca:read orca:write" || out.Scope != c.Scope {
		t.Fatalf("scope = %q / %q", c.Scope, out.Scope)
	}
	ttl := c.Expiry.Time().Sub(c.IssuedAt.Time())
	if ttl > 15*time.Minute || ttl <= 0 || out.ExpiresIn != int(ttl.Seconds()) {
		t.Fatalf("ttl = %v expires_in = %d", ttl, out.ExpiresIn)
	}
	if out.RefreshToken == "" || out.AccessToken == "" {
		t.Fatal("missing tokens")
	}
	if _, ok := h.repo.refresh[out.RefreshToken]; ok {
		t.Fatal("raw refresh token stored")
	}
}

func TestAccessTokenTTLClampedToFifteenMinutes(t *testing.T) {
	h := newOAuthHarness(t)
	cfg := h.cfg
	cfg.AccessTokenTTL = 24 * time.Hour
	h.exchange = NewOAuthExchangeToken(h.repo, h.users, h.signer, h.audit, h.clock, cfg)
	h.tokens(h.authorize())
	if ttl := h.signer.last.Expiry.Time().Sub(h.signer.last.IssuedAt.Time()); ttl > 15*time.Minute {
		t.Fatalf("access ttl %v exceeds 15m", ttl)
	}
}

func TestExchangeCode_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*oauthHarness, *OAuthExchangeTokenInput)
		code   string
	}{
		{"wrong verifier", func(_ *oauthHarness, in *OAuthExchangeTokenInput) {
			v, _ := pkcePair("other")
			in.CodeVerifier = v
		}, CodeOAuthInvalidGrant},
		{"verifier equals challenge (plain downgrade)", func(h *oauthHarness, in *OAuthExchangeTokenInput) {
			for _, c := range h.repo.codes {
				in.CodeVerifier = c.CodeChallenge
			}
		}, CodeOAuthInvalidGrant},
		{"missing verifier", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.CodeVerifier = "" }, CodeOAuthInvalidRequest},
		{"short verifier", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.CodeVerifier = "abc" }, CodeOAuthInvalidGrant},
		{"redirect one char off", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.RedirectURI += "x" }, CodeOAuthInvalidGrant},
		{"other client (confused deputy)", func(h *oauthHarness, in *OAuthExchangeTokenInput) {
			h.repo.clients["client-2"] = domain.OAuthClient{ClientID: "client-2", ClientName: "B", RedirectURIs: []string{testRedirectURI}}
			in.ClientID = "client-2"
		}, CodeOAuthInvalidGrant},
		{"unknown code", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.Code = "nope" }, CodeOAuthInvalidGrant},
		{"resource differs", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.Resource = "https://evil/mcp" }, CodeOAuthInvalidTarget},
		{"expired code", func(h *oauthHarness, _ *OAuthExchangeTokenInput) { h.clock.now = h.clock.now.Add(61 * time.Second) }, CodeOAuthInvalidGrant},
		{"client blocked after consent", func(h *oauthHarness, _ *OAuthExchangeTokenInput) { h.setStatus(domain.OAuthClientBlocked) }, CodeOAuthUnauthorizedClient},
		{"grant revoked after consent", func(h *oauthHarness, _ *OAuthExchangeTokenInput) {
			for _, f := range h.repo.families {
				h.repo.grants[f.GrantID] = true
			}
		}, CodeOAuthInvalidGrant},
		{"user deactivated after consent", func(h *oauthHarness, _ *OAuthExchangeTokenInput) {
			u := h.user
			u.IsActive = false
			h.users.seed(u, "x")
		}, CodeOAuthInvalidGrant},
		{"unsupported grant type", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.GrantType = "password" }, CodeOAuthUnsupportedGrantType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newOAuthHarness(t)
			in := h.codeInput(h.authorize())
			tc.mutate(h, &in)
			_, err := h.exchange.Execute(context.Background(), in)
			wantCode(t, err, tc.code)
		})
	}
}

func TestExchangeCode_ReplayRejectedAndRevokesFamily(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	first := h.tokens(a)

	_, err := h.exchange.Execute(context.Background(), h.codeInput(a))
	wantCode(t, err, CodeOAuthInvalidGrant)

	fam := h.familyOfGrant(a.grantID)
	if fam.RevokedAt == nil || fam.RevokeReason != domain.OAuthRevokeReuseDetected {
		t.Fatalf("family not revoked on replay: %+v", fam)
	}
	// Tokens issued from the replayed code are dead too.
	_, err = h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	wantCode(t, err, CodeOAuthInvalidGrant)
	if !hasAudit(h, "oauth.code_replay") {
		t.Fatal("missing oauth.code_replay audit entry")
	}
}

func TestExchangeCode_LostClaimRaceTreatedAsReplay(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	h.repo.forceCodeRace = true
	_, err := h.exchange.Execute(context.Background(), h.codeInput(a))
	wantCode(t, err, CodeOAuthInvalidGrant)
	if f := h.familyOfGrant(a.grantID); f.RevokedAt == nil {
		t.Fatal("family must be revoked when the claim race is lost")
	}
}

func TestRefresh_RotationAndReuseDetection(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	first := h.tokens(a)

	second, err := h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if second.RefreshToken == "" || second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	if len(h.signer.last.Audience) != 1 || h.signer.last.Audience[0] != testResource {
		t.Fatal("refreshed access token lost its audience")
	}

	// Replaying the old refresh token = theft signal: whole family dies,
	// including the freshly rotated token.
	_, err = h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	wantCode(t, err, CodeOAuthInvalidGrant)
	if f := h.familyOfGrant(a.grantID); f.RevokedAt == nil || f.RevokeReason != domain.OAuthRevokeReuseDetected {
		t.Fatalf("family not revoked on reuse: %+v", f)
	}
	_, err = h.exchange.Execute(context.Background(), h.refreshInput(second.RefreshToken))
	wantCode(t, err, CodeOAuthInvalidGrant)
	if !hasAudit(h, "oauth.refresh_reuse_detected") {
		t.Fatal("missing oauth.refresh_reuse_detected audit entry")
	}
}

func TestRefresh_LostRotationRaceTreatedAsReuse(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	first := h.tokens(a)
	h.repo.forceRefreshRace = true
	_, err := h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	wantCode(t, err, CodeOAuthInvalidGrant)
	if f := h.familyOfGrant(a.grantID); f.RevokedAt == nil {
		t.Fatal("family must be revoked when rotation race is lost")
	}
}

func TestRefresh_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*oauthHarness, *OAuthExchangeTokenInput)
		code   string
	}{
		{"other client", func(h *oauthHarness, in *OAuthExchangeTokenInput) {
			h.repo.clients["c2"] = domain.OAuthClient{ClientID: "c2"}
			in.ClientID = "c2"
		}, CodeOAuthInvalidGrant},
		{"missing client_id", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.ClientID = "" }, CodeOAuthInvalidRequest},
		{"unknown token", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.RefreshToken = "nope" }, CodeOAuthInvalidGrant},
		{"client blocked", func(h *oauthHarness, _ *OAuthExchangeTokenInput) { h.setStatus(domain.OAuthClientBlocked) }, CodeOAuthUnauthorizedClient},
		{"scope widening", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.Scope = "orca:read orca:exec" }, CodeOAuthInvalidScope},
		{"resource differs", func(_ *oauthHarness, in *OAuthExchangeTokenInput) { in.Resource = "https://evil/mcp" }, CodeOAuthInvalidTarget},
		{"past absolute lifetime", func(h *oauthHarness, _ *OAuthExchangeTokenInput) {
			h.clock.now = h.clock.now.Add(31 * 24 * time.Hour)
		}, CodeOAuthInvalidGrant},
		{"grant revoked", func(h *oauthHarness, _ *OAuthExchangeTokenInput) {
			for _, f := range h.repo.families {
				h.repo.grants[f.GrantID] = true
			}
		}, CodeOAuthInvalidGrant},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newOAuthHarness(t)
			first := h.tokens(h.authorize())
			in := h.refreshInput(first.RefreshToken)
			tc.mutate(h, &in)
			_, err := h.exchange.Execute(context.Background(), in)
			wantCode(t, err, tc.code)
		})
	}
}

func TestRefresh_ScopeCanOnlyNarrow(t *testing.T) {
	h := newOAuthHarness(t)
	first := h.tokens(h.authorize())
	in := h.refreshInput(first.RefreshToken)
	in.Scope = "orca:read"
	out, err := h.exchange.Execute(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Scope != "orca:read" || h.signer.last.Scope != "orca:read" {
		t.Fatalf("scope = %q", out.Scope)
	}
}

func TestRefresh_RoleDemotionShrinksScope(t *testing.T) {
	h := newOAuthHarness(t)
	admin := h.user
	admin.Role = domain.RoleAdmin
	h.users.seed(admin, "x")
	first := h.tokens(h.authorize(domain.OAuthScopeRead, domain.OAuthScopeAdmin))
	if h.signer.last.Scope != "orca:read orca:admin" {
		t.Fatalf("scope = %q", h.signer.last.Scope)
	}
	h.users.seed(h.user, "x") // back to role user
	out, err := h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	if out.Scope != "orca:read" {
		t.Fatalf("demoted user kept scope %q", out.Scope)
	}
}

func TestRefresh_RotationDoesNotExtendAbsoluteLifetime(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	first := h.tokens(a)
	famCreated := h.familyOfGrant(a.grantID).CreatedAt
	h.clock.now = h.clock.now.Add(20 * 24 * time.Hour)
	out, err := h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	want := famCreated.Add(DefaultOAuthRefreshTokenTTL)
	if got := h.repo.refresh[hashToken(out.RefreshToken)].ExpiresAt; !got.Equal(want) {
		t.Fatalf("rotated refresh expires %v, want fixed %v", got, want)
	}
}

func TestSetClientStatus_BlockRevokesLiveTokensAndRejectsExchange(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	first := h.tokens(a)

	view, err := h.setSt.Execute(context.Background(), OAuthSetClientStatusInput{
		TenantID: h.tenantID, ClientID: h.clientID, Status: domain.OAuthClientBlocked, ActorUserID: h.user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status.Status != domain.OAuthClientBlocked {
		t.Fatalf("status = %v", view.Status.Status)
	}
	if f := h.familyOfGrant(a.grantID); f.RevokedAt == nil || f.RevokeReason != domain.OAuthRevokeClientBlocked {
		t.Fatalf("family not revoked: %+v", f)
	}
	_, err = h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	wantCode(t, err, CodeOAuthInvalidGrant)

	// New authorization attempts are refused too.
	_, err = h.issue.Execute(context.Background(), OAuthIssueAuthCodeInput{
		TenantID: h.tenantID, UserID: h.user.ID, ClientID: h.clientID, RedirectURI: testRedirectURI,
		Scopes: []string{"orca:read"}, CodeChallenge: a.challenge, Resource: testResource, GrantID: "5d0f5a2c-84d4-4b4e-93a4-0a1d6c1c2222",
	})
	wantCode(t, err, CodeOAuthUnauthorizedClient)
	if !hasAudit(h, "oauth.client_status_changed") {
		t.Fatal("missing audit entry")
	}
}

func TestSetClientStatus_Validation(t *testing.T) {
	h := newOAuthHarness(t)
	_, err := h.setSt.Execute(context.Background(), OAuthSetClientStatusInput{TenantID: h.tenantID, ClientID: h.clientID, Status: domain.OAuthClientPending})
	wantCode(t, err, CodeOAuthInvalidRequest)
	_, err = h.setSt.Execute(context.Background(), OAuthSetClientStatusInput{TenantID: "other-tenant", ClientID: h.clientID, Status: domain.OAuthClientBlocked})
	wantCode(t, err, CodeOAuthClientNotFound)
}

func TestRevokeGrant_IsIdempotentAndKillsTokens(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	first := h.tokens(a)
	in := OAuthRevokeGrantInput{TenantID: h.tenantID, GrantID: a.grantID, Reason: domain.OAuthRevokeAdminRevoked, ActorUserID: h.user.ID}
	for i := 0; i < 2; i++ {
		if err := h.revokeG.Execute(context.Background(), in); err != nil {
			t.Fatalf("revoke #%d: %v", i, err)
		}
	}
	if f := h.familyOfGrant(a.grantID); f.RevokeReason != domain.OAuthRevokeAdminRevoked {
		t.Fatalf("reason = %q", f.RevokeReason)
	}
	_, err := h.exchange.Execute(context.Background(), h.refreshInput(first.RefreshToken))
	wantCode(t, err, CodeOAuthInvalidGrant)

	bad := in
	bad.Reason = "reuse_detected"
	wantCode(t, h.revokeG.Execute(context.Background(), bad), CodeOAuthInvalidRequest)
	bad = in
	bad.GrantID = "nope"
	wantCode(t, h.revokeG.Execute(context.Background(), bad), CodeOAuthInvalidRequest)
}

func TestRevokeGrant_OtherTenantCannotRevoke(t *testing.T) {
	h := newOAuthHarness(t)
	a := h.authorize()
	h.tokens(a)
	err := h.revokeG.Execute(context.Background(), OAuthRevokeGrantInput{TenantID: "22222222-2222-2222-2222-222222222222", GrantID: a.grantID})
	if err != nil {
		t.Fatal(err)
	}
	if f := h.familyOfGrant(a.grantID); f.RevokedAt != nil {
		t.Fatal("another tenant revoked this tenant's family")
	}
}

func TestRevokeToken(t *testing.T) {
	t.Run("refresh token revokes family", func(t *testing.T) {
		h := newOAuthHarness(t)
		a := h.authorize()
		out := h.tokens(a)
		if err := h.revoke.Execute(context.Background(), OAuthRevokeTokenInput{Token: out.RefreshToken, ClientID: h.clientID}); err != nil {
			t.Fatal(err)
		}
		if h.familyOfGrant(a.grantID).RevokedAt == nil {
			t.Fatal("family not revoked")
		}
	})
	t.Run("access token revokes family", func(t *testing.T) {
		h := newOAuthHarness(t)
		a := h.authorize()
		out := h.tokens(a)
		if err := h.revoke.Execute(context.Background(), OAuthRevokeTokenInput{Token: out.AccessToken}); err != nil {
			t.Fatal(err)
		}
		if h.familyOfGrant(a.grantID).RevokedAt == nil {
			t.Fatal("family not revoked")
		}
	})
	t.Run("forged access token is ignored", func(t *testing.T) {
		h := newOAuthHarness(t)
		a := h.authorize()
		h.tokens(a)
		forger := newRSATestSigner()
		forged, err := forger.Sign(context.Background(), jwtauth.Claims{FamilyID: h.familyOfGrant(a.grantID).FamilyID, TokenUse: "mcp_oauth"})
		if err != nil {
			t.Fatal(err)
		}
		if err := h.revoke.Execute(context.Background(), OAuthRevokeTokenInput{Token: forged}); err != nil {
			t.Fatal(err)
		}
		if h.familyOfGrant(a.grantID).RevokedAt != nil {
			t.Fatal("unverified fid claim revoked a family")
		}
	})
	t.Run("other client cannot revoke", func(t *testing.T) {
		h := newOAuthHarness(t)
		a := h.authorize()
		out := h.tokens(a)
		if err := h.revoke.Execute(context.Background(), OAuthRevokeTokenInput{Token: out.RefreshToken, ClientID: "someone-else"}); err != nil {
			t.Fatal(err)
		}
		if h.familyOfGrant(a.grantID).RevokedAt != nil {
			t.Fatal("foreign client revoked the family")
		}
	})
	t.Run("unknown or empty token succeeds silently", func(t *testing.T) {
		h := newOAuthHarness(t)
		for _, tok := range []string{"", "garbage", "a.b.c"} {
			if err := h.revoke.Execute(context.Background(), OAuthRevokeTokenInput{Token: tok}); err != nil {
				t.Fatalf("token %q: %v", tok, err)
			}
		}
	})
}

func hasAudit(h *oauthHarness, action string) bool {
	for _, e := range h.audit.entries {
		if strings.EqualFold(e.Action, action) {
			return true
		}
	}
	return false
}
