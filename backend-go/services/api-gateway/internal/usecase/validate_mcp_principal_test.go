package usecase

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/jwtauth"
)

const testMCPResource = "https://orca.example.com/mcp"

// mcpToken signs a token with arbitrary claims; mutate edits the defaults.
func mcpToken(t *testing.T, mutate func(*jwtauth.Claims)) (string, *fakeJWKSClient) {
	t.Helper()
	transit := newFakeTransit(t, 1)
	signer := jwtauth.NewTransitSigner("jwt-signing", transit.sign, transit.publicKeyVersions)
	c := jwtauth.Claims{
		Claims: jwt.Claims{
			Issuer: jwtauth.Issuer, Subject: "user-1", Audience: jwt.Audience{testMCPResource}, ID: "jti-1",
			IssuedAt: jwt.NewNumericDate(time.Now()), Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		TenantID: "tenant-1", Scope: "orca:read orca:write orca:admin", TokenUse: McpTokenUsePAT,
	}
	if mutate != nil {
		mutate(&c)
	}
	tok, err := jwtauth.Sign(context.Background(), signer, c)
	if err != nil {
		t.Fatal(err)
	}
	jwks, _ := signer.PublicJWKS(context.Background())
	return tok, &fakeJWKSClient{kid: jwks.Keys[0].KeyID, key: jwks.Keys[0].Key}
}

type fakeMcpResolver struct {
	res   McpResolveResult
	err   error
	calls int
	last  McpResolveInput
}

func (f *fakeMcpResolver) Resolve(_ context.Context, in McpResolveInput) (McpResolveResult, error) {
	f.calls++
	f.last = in
	return f.res, f.err
}

func mcpRequest(auth string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	return r
}

func TestValidateMCP_AcceptsPATAndFillsLivePrincipal(t *testing.T) {
	tok, jwks := mcpToken(t, nil)
	v := NewAuthValidator(jwks)
	res := &fakeMcpResolver{res: McpResolveResult{Active: true, Role: "user"}}
	v.McpPrincipals = res

	// "omp_" prefix (PAT secret form) and the bare JWT must both work.
	for _, header := range []string{"Bearer omp_" + tok, "Bearer " + tok} {
		p, err := v.ValidateMCP(mcpRequest(header), testMCPResource+"/")
		if err != nil {
			t.Fatalf("%q: %v", header[:12], err)
		}
		if p.TenantID != "tenant-1" || p.UserID != "user-1" || p.Role != "user" || p.JTI != "jti-1" || p.TokenUse != McpTokenUsePAT {
			t.Fatalf("principal = %+v", p)
		}
		// Role ceiling: orca:admin in the token is cut because the live role is user.
		if got := p.Scopes; len(got) != 2 || got[0] != "orca:read" || got[1] != "orca:write" {
			t.Fatalf("scopes = %v, want [read write]", got)
		}
	}
	if res.last.JTI != "jti-1" || res.last.TokenUse != McpTokenUsePAT {
		t.Fatalf("resolver input = %+v", res.last)
	}
}

func TestValidateMCP_Rejections(t *testing.T) {
	good := func(c *jwtauth.Claims) {}
	cases := []struct {
		name   string
		mutate func(*jwtauth.Claims)
		res    McpResolveResult
		rerr   error
		want   error
	}{
		{"aud orca-cli", func(c *jwtauth.Claims) { c.Audience = jwt.Audience{"orca-cli"} }, McpResolveResult{Active: true, Role: "user"}, nil, ErrAudienceMismatch},
		{"no aud", func(c *jwtauth.Claims) { c.Audience = nil }, McpResolveResult{Active: true, Role: "user"}, nil, ErrAudienceMismatch},
		{"multi aud", func(c *jwtauth.Claims) { c.Audience = jwt.Audience{testMCPResource, "orca-cli"} }, McpResolveResult{Active: true, Role: "user"}, nil, ErrAudienceMismatch},
		{"other resource", func(c *jwtauth.Claims) { c.Audience = jwt.Audience{"https://evil.example/mcp"} }, McpResolveResult{Active: true, Role: "user"}, nil, ErrAudienceMismatch},
		{"unknown token_use", func(c *jwtauth.Claims) { c.TokenUse = "" }, McpResolveResult{Active: true, Role: "user"}, nil, ErrMissingIdentityClaims},
		{"unknown scope", func(c *jwtauth.Claims) { c.Scope = "orca:read bogus" }, McpResolveResult{Active: true, Role: "user"}, nil, ErrMissingIdentityClaims},
		{"revoked", good, McpResolveResult{Active: false, InactiveReason: "revoked"}, nil, ErrPrincipalInactive},
		{"empty role", good, McpResolveResult{Active: true, Role: ""}, nil, ErrPrincipalInactive},
		{"resolver down", good, McpResolveResult{}, errors.New("boom"), ErrPrincipalLookupFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tok, jwks := mcpToken(t, c.mutate)
			v := NewAuthValidator(jwks)
			v.McpPrincipals = &fakeMcpResolver{res: c.res, err: c.rerr}
			if _, err := v.ValidateMCP(mcpRequest("Bearer "+tok), testMCPResource); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestValidateMCP_CookieIsNeverRead(t *testing.T) {
	tok, jwks := mcpToken(t, nil)
	v := NewAuthValidator(jwks)
	v.McpPrincipals = &fakeMcpResolver{res: McpResolveResult{Active: true, Role: "user"}}
	r := mcpRequest("")
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok}) // even a perfectly valid MCP JWT in the cookie
	if _, err := v.ValidateMCP(r, testMCPResource); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("cookie must not authenticate /mcp, got %v", err)
	}
}

func TestValidateMCP_MissingResolverFailsClosed(t *testing.T) {
	tok, jwks := mcpToken(t, nil)
	v := NewAuthValidator(jwks)
	if _, err := v.ValidateMCP(mcpRequest("Bearer "+tok), testMCPResource); !errors.Is(err, ErrPrincipalLookupFailed) {
		t.Fatalf("got %v", err)
	}
}

// Both directions of audience binding: the REST/WS validator refuses MCP
// tokens, still accepts orca-cli and audience-less device tokens.
func TestValidate_RejectsMCPAudienceButKeepsOthers(t *testing.T) {
	rest := func(tok string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/v1/auth/mcp-tokens", nil)
		r.Header.Set("Authorization", "Bearer "+tok)
		return r
	}
	tok, jwks := mcpToken(t, nil)
	v := NewAuthValidator(jwks)
	v.RejectAudiences = []string{testMCPResource}
	if _, err := v.Validate(rest(tok)); !errors.Is(err, ErrAudienceNotAccepted) {
		t.Fatalf("PAT on REST: got %v", err)
	}
	// Same token through the cookie path (the WS fallback) is refused too.
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	if _, err := v.Validate(r); !errors.Is(err, ErrAudienceNotAccepted) {
		t.Fatalf("PAT in cookie: got %v", err)
	}

	for name, aud := range map[string]jwt.Audience{"orca-cli": {"orca-cli"}, "no aud": nil} {
		tok2, jwks2 := mcpToken(t, func(c *jwtauth.Claims) { c.Audience = aud; c.TokenUse = ""; c.Scope = "" })
		v2 := NewAuthValidator(jwks2)
		v2.RejectAudiences = []string{testMCPResource}
		if id, err := v2.Validate(rest(tok2)); err != nil || id.UserID != "user-1" {
			t.Errorf("%s must still pass REST/WS: %v", name, err)
		}
	}
}

func TestValidate_MCPTokenUseRejectedEvenWithoutConfiguredAudience(t *testing.T) {
	tok, jwks := mcpToken(t, nil)
	v := NewAuthValidator(jwks) // RejectAudiences left empty (misconfiguration)
	r := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if _, err := v.Validate(r); !errors.Is(err, ErrAudienceNotAccepted) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateMCP_RejectionReasons(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*jwtauth.Claims)
		res    McpResolveResult
		want   string
	}{
		{"expired", func(c *jwtauth.Claims) {
			c.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
			c.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Hour))
		}, McpResolveResult{Active: true, Role: "user"}, McpReasonExpired},
		{"audience", func(c *jwtauth.Claims) { c.Audience = nil }, McpResolveResult{Active: true, Role: "user"}, McpReasonAudience},
		{"revoked", nil, McpResolveResult{InactiveReason: "revoked"}, McpReasonRevoked},
		{"grant revoked", nil, McpResolveResult{InactiveReason: "grant_revoked"}, McpReasonRevoked},
		{"suspended", nil, McpResolveResult{InactiveReason: "suspended"}, McpReasonKillSwitch},
		{"user inactive", nil, McpResolveResult{InactiveReason: "user_inactive"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tok, jwks := mcpToken(t, c.mutate)
			v := NewAuthValidator(jwks)
			v.McpPrincipals = &fakeMcpResolver{res: c.res}
			_, err := v.ValidateMCP(mcpRequest("Bearer "+tok), testMCPResource)
			if err == nil || McpRejectionReason(err) != c.want {
				t.Fatalf("err=%v reason=%q want %q", err, McpRejectionReason(err), c.want)
			}
		})
	}
}
