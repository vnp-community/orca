package httpgateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/jwtauth"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcptokens"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

const patResource = "https://orca.example.com/mcp"

type patAuth struct{ issueCalls int }

func (p *patAuth) IssueMcpToken(_ context.Context, in *authv1.IssueMcpTokenRequest, _ ...grpc.CallOption) (*authv1.IssueMcpTokenResponse, error) {
	p.issueCalls++
	return &authv1.IssueMcpTokenResponse{
		Token:  &authv1.McpTokenInfo{Jti: "j1", Name: in.GetName(), Scopes: in.GetScopes(), CreatedAt: timestamppb.Now(), ExpiresAt: timestamppb.New(time.Now().Add(time.Hour))},
		Secret: "omp_ONE.TIME.SECRET",
	}, nil
}
func (p *patAuth) ListMcpTokens(context.Context, *emptypb.Empty, ...grpc.CallOption) (*authv1.ListMcpTokensResponse, error) {
	return &authv1.ListMcpTokensResponse{Tokens: []*authv1.McpTokenInfo{{Jti: "j1", Name: "n", ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)), CreatedAt: timestamppb.Now()}}}, nil
}
func (p *patAuth) RevokeMcpToken(context.Context, *authv1.RevokeMcpTokenRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

type patPolicy struct{}

func (patPolicy) GetServerInfo(context.Context, *mcpv1.GetServerInfoRequest, ...grpc.CallOption) (*mcpv1.GetServerInfoResponse, error) {
	return &mcpv1.GetServerInfoResponse{Enabled: true, MaxTokenDays: 90}, nil
}

type rsaJWKS struct{ k *rsa.PrivateKey }

func (r rsaJWKS) PublicKey(context.Context, string) (any, error) { return &r.k.PublicKey, nil }

func patRouter(t *testing.T, a *patAuth) (http.Handler, func(aud jwt.Audience, tokenUse string) string) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sign := func(aud jwt.Audience, tokenUse string) string {
		sig, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: k, KeyID: "k"}}, (&jose.SignerOptions{}).WithType("JWT"))
		tok, err := jwt.Signed(sig).Claims(jwtauth.Claims{
			Claims:   jwt.Claims{Issuer: jwtauth.Issuer, Subject: "u1", Audience: aud, ID: "jti", IssuedAt: jwt.NewNumericDate(time.Now()), Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour))},
			TenantID: "t1", TokenUse: tokenUse, Scope: "orca:read",
		}).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	v := usecase.NewAuthValidator(rsaJWKS{k})
	v.RejectAudiences = []string{patResource}
	h := NewRouter(Deps{
		Registry: domain.NewDefaultServiceRegistry(), AuthValidator: v, RateLimiter: usecase.NewRateLimiter(1000, 1000),
		CookieValidator: fakeCookie{id: wscompat.Identity{TenantID: "t1", UserID: "u1", Role: "user"}},
		McpTokens:       &McpTokenRoutes{Service: &mcptokens.Service{Auth: a, Policy: patPolicy{}}},
	})
	return h, sign
}

func TestMcpTokenRoutes_Lifecycle(t *testing.T) {
	a := &patAuth{}
	h, _ := patRouter(t, a)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.AddCookie(&http.Cookie{Name: "orca_session", Value: "s"})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	rec := do(http.MethodPost, "/v1/auth/mcp-tokens/", `{"name":"ci","scopes":["orca:read"],"expires_in_days":30,"user_id":"someone-else","audience":"orca-cli"}`)
	if rec.Code != http.StatusCreated || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
	for _, want := range []string{`"secret":"omp_ONE.TIME.SECRET"`, `"token":{"id":"j1"`, `"status":"active"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("create body lacks %s: %s", want, rec.Body.String())
		}
	}
	rec = do(http.MethodGet, "/v1/auth/mcp-tokens/", "")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "secret") || strings.Contains(rec.Body.String(), "omp_") {
		t.Fatalf("list must never carry a secret: %d %s", rec.Code, rec.Body.String())
	}
	if rec = do(http.MethodDelete, "/v1/auth/mcp-tokens/j1", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if rec = do(http.MethodPost, "/v1/auth/mcp-tokens/", `{"name":"ci","scopes":["orca:read"],"expires_in_days":400}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), "MCP_TOKEN_TOO_LONG") {
		t.Fatalf("too long: %d %s", rec.Code, rec.Body.String())
	}
}

// Audience binding, REST side: an MCP token (PAT or OAuth) cannot reach the
// REST edge, so it cannot mint new PATs; the CLI token still can.
func TestMcpTokenRoutes_PATCannotMintPAT_CLITokenCan(t *testing.T) {
	a := &patAuth{}
	h, sign := patRouter(t, a)
	post := func(tok string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/mcp-tokens/", strings.NewReader(`{"name":"x","scopes":["orca:read"],"expires_in_days":5}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, use := range []string{"mcp_pat", "mcp_oauth"} {
		if c := post(sign(jwt.Audience{patResource}, use)); c != http.StatusUnauthorized {
			t.Errorf("%s token on REST: %d, want 401", use, c)
		}
	}
	if a.issueCalls != 0 {
		t.Fatal("rejected token must not reach auth-service")
	}
	if c := post(sign(jwt.Audience{"orca-cli"}, "")); c != http.StatusCreated {
		t.Errorf("CLI token: %d, want 201", c)
	}
}
