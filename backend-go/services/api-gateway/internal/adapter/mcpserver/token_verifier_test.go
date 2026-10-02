package mcpserver_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

const verifierResource = "https://orca.example.com/mcp"

type testKeys struct{ key *rsa.PrivateKey }

func (k testKeys) PublicKey(_ context.Context, kid string) (any, error) {
	if kid != "k1" {
		return nil, errors.New("unknown kid")
	}
	return &k.key.PublicKey, nil
}

func (k testKeys) sign(t *testing.T, c jwtauth.Claims) string {
	t.Helper()
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: k.key, KeyID: "k1"}}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := jwt.Signed(sig).Claims(c).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type stubResolver struct {
	res   usecase.McpResolveResult
	err   error
	calls atomic.Int32
}

func (s *stubResolver) Resolve(context.Context, usecase.McpResolveInput) (usecase.McpResolveResult, error) {
	s.calls.Add(1)
	return s.res, s.err
}

func claims(mut func(*jwtauth.Claims)) jwtauth.Claims {
	c := jwtauth.Claims{
		Claims: jwt.Claims{Issuer: jwtauth.Issuer, Subject: "u1", Audience: jwt.Audience{verifierResource}, ID: "jti1",
			IssuedAt: jwt.NewNumericDate(time.Now()), Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		TenantID: "t1", Scope: "orca:read", TokenUse: "mcp_pat",
	}
	if mut != nil {
		mut(&c)
	}
	return c
}

func verifierServer(t *testing.T, res *stubResolver) (*httptest.Server, testKeys) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := testKeys{key: k}
	v := usecase.NewAuthValidator(keys)
	v.McpPrincipals = res
	h := mcpserver.NewHandler(mcpserver.Deps{
		Config:   mcpserver.Config{ResourceURL: verifierResource, IssuerURL: "https://orca.example.com", ScopesSupported: []string{"orca:read"}},
		Verifier: mcpserver.NewBearerTokenVerifier(v, verifierResource),
	})
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, keys
}

const verifierInitBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`

func post(t *testing.T, url string, hdr map[string]string, cookie string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+"/mcp", strings.NewReader(verifierInitBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "orca_session", Value: cookie})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func TestBearerTokenVerifier_EndToEnd(t *testing.T) {
	res := &stubResolver{res: usecase.McpResolveResult{Active: true, Role: "user"}}
	srv, keys := verifierServer(t, res)
	good := keys.sign(t, claims(nil))

	// PAT (omp_ prefixed) and OAuth-style bearer both authenticate.
	for _, tok := range []string{"omp_" + good, keys.sign(t, claims(func(c *jwtauth.Claims) { c.TokenUse = "mcp_oauth"; c.FamilyID = "f"; c.GrantID = "g"; c.ClientID = "c" }))} {
		if code, body, _ := post(t, srv.URL, map[string]string{"Authorization": "Bearer " + tok}, ""); code != http.StatusOK {
			t.Fatalf("valid token: %d %s", code, body)
		}
	}

	wwwAuth := func(h http.Header) string { return h.Get("WWW-Authenticate") }
	for name, tc := range map[string]struct {
		hdr    map[string]string
		cookie string
	}{
		"no credentials":                {nil, ""},
		"cookie only (valid JWT value)": {nil, good},
		"cookie plus nothing":           {nil, "opaque-session-token"},
		"aud orca-cli":                  {map[string]string{"Authorization": "Bearer " + keys.sign(t, claims(func(c *jwtauth.Claims) { c.Audience = jwt.Audience{"orca-cli"} }))}, ""},
		"no aud":                        {map[string]string{"Authorization": "Bearer " + keys.sign(t, claims(func(c *jwtauth.Claims) { c.Audience = nil }))}, ""},
		"garbage":                       {map[string]string{"Authorization": "Bearer not-a-jwt"}, ""},
		"wrong signer":                  {map[string]string{"Authorization": "Bearer " + testKeys{key: mustKey(t)}.sign(t, claims(nil))}, ""},
	} {
		code, _, h := post(t, srv.URL, tc.hdr, tc.cookie)
		if code != http.StatusUnauthorized || !strings.Contains(wwwAuth(h), "resource_metadata=") {
			t.Errorf("%s: %d %q", name, code, wwwAuth(h))
		}
	}
}

func mustKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestBearerTokenVerifier_RevokedDeactivatedAndOutage(t *testing.T) {
	res := &stubResolver{res: usecase.McpResolveResult{Active: false, InactiveReason: "revoked"}}
	srv, keys := verifierServer(t, res)
	tok := keys.sign(t, claims(nil))
	hdr := map[string]string{"Authorization": "Bearer " + tok}

	for _, reason := range []string{"revoked", "user_inactive", "client_blocked", "grant_revoked", "expired"} {
		res.res = usecase.McpResolveResult{Active: false, InactiveReason: reason}
		if code, _, h := post(t, srv.URL, hdr, ""); code != http.StatusUnauthorized || !strings.Contains(h.Get("WWW-Authenticate"), `error="invalid_token"`) {
			t.Errorf("%s: %d %q", reason, code, h.Get("WWW-Authenticate"))
		}
	}
	// Lookup failure fails closed as 503, not 401 (no re-login loop).
	res.err = errors.New("auth-service down")
	if code, _, _ := post(t, srv.URL, hdr, ""); code != http.StatusServiceUnavailable {
		t.Errorf("outage: %d", code)
	}
}

// The role is whatever auth-service says now; the token carries none.
func TestBearerTokenVerifier_PrincipalRoleAndScopesAreLive(t *testing.T) {
	res := &stubResolver{res: usecase.McpResolveResult{Active: true, Role: "user"}}
	_, keys := verifierServer(t, res)
	v := usecase.NewAuthValidator(keys)
	v.McpPrincipals = res
	ver := mcpserver.NewBearerTokenVerifier(v, verifierResource)
	tok := keys.sign(t, claims(func(c *jwtauth.Claims) { c.Scope = "orca:read orca:admin"; c.Role = "admin" })) // forged role claim

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	p, err := ver.Verify(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if p.Role != "user" || len(p.Scopes) != 1 || p.Scopes[0] != "orca:read" || p.TenantID != "t1" || p.UserID != "u1" || p.TokenID != "jti1" {
		t.Fatalf("principal = %+v", p)
	}
	res.res = usecase.McpResolveResult{Active: true, Role: "admin"}
	if p, _ := ver.Verify(context.Background(), req); len(p.Scopes) != 2 {
		t.Fatalf("admin keeps orca:admin: %+v", p)
	}
}

// mcp_depth / mcp_root are read from the VERIFIED claims into the Principal
// (agent recursion guard); a person's token has depth 0.
func TestBearerTokenVerifier_DepthAndRootFromVerifiedClaims(t *testing.T) {
	res := &stubResolver{res: usecase.McpResolveResult{Active: true, Role: "user"}}
	_, keys := verifierServer(t, res)
	v := usecase.NewAuthValidator(keys)
	v.McpPrincipals = res
	ver := mcpserver.NewBearerTokenVerifier(v, verifierResource)
	verify := func(tok string) mcpserver.Principal {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		p, err := ver.Verify(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := verify(keys.sign(t, claims(func(c *jwtauth.Claims) { c.McpDepth = 2; c.McpRoot = "root-sess" }))); p.Depth != 2 || p.Root != "root-sess" {
		t.Errorf("agent token: %+v", p)
	}
	if p := verify(keys.sign(t, claims(nil))); p.Depth != 0 || p.Root != "" {
		t.Errorf("human token: %+v", p)
	}
}
