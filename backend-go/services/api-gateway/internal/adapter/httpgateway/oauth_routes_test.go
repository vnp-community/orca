package httpgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type fakeOAuthAuth struct {
	authv1.AuthServiceClient
	validate func(*authv1.OAuthValidateAuthorizeRequestRequest) (*authv1.OAuthAuthorizeRequestInfo, error)
	exchange func(*authv1.OAuthExchangeTokenRequest) (*authv1.OAuthTokenResponse, error)
	register func(*authv1.OAuthRegisterClientRequest) (*authv1.OAuthRegisterClientResponse, error)
	revoke   func(*authv1.OAuthRevokeTokenRequest) error
	lastCtx  context.Context
}

func (f *fakeOAuthAuth) OAuthValidateAuthorizeRequest(ctx context.Context, in *authv1.OAuthValidateAuthorizeRequestRequest, _ ...grpc.CallOption) (*authv1.OAuthAuthorizeRequestInfo, error) {
	f.lastCtx = ctx
	return f.validate(in)
}
func (f *fakeOAuthAuth) OAuthExchangeToken(_ context.Context, in *authv1.OAuthExchangeTokenRequest, _ ...grpc.CallOption) (*authv1.OAuthTokenResponse, error) {
	return f.exchange(in)
}
func (f *fakeOAuthAuth) OAuthRegisterClient(_ context.Context, in *authv1.OAuthRegisterClientRequest, _ ...grpc.CallOption) (*authv1.OAuthRegisterClientResponse, error) {
	return f.register(in)
}
func (f *fakeOAuthAuth) OAuthRevokeToken(_ context.Context, in *authv1.OAuthRevokeTokenRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, f.revoke(in)
}

type fakeConsent struct {
	resp    *mcpv1.CreateConsentRequestResponse
	err     error
	calls   int
	lastMD  metadata.MD
	lastReq *mcpv1.CreateConsentRequestRequest
}

func (f *fakeConsent) CreateConsentRequest(ctx context.Context, in *mcpv1.CreateConsentRequestRequest, _ ...grpc.CallOption) (*mcpv1.CreateConsentRequestResponse, error) {
	f.calls++
	f.lastMD, _ = metadata.FromOutgoingContext(ctx)
	f.lastReq = in
	return f.resp, f.err
}

type fakeCookie struct {
	id  wscompat.Identity
	err error
}

func (f fakeCookie) ValidateCookie(_ context.Context, r *http.Request) (wscompat.Identity, error) {
	if _, err := r.Cookie("orca_session"); err != nil || f.err != nil {
		return wscompat.Identity{}, status.Error(codes.Unauthenticated, "no session")
	}
	return f.id, nil
}

const validAuthorizeQuery = "response_type=code&client_id=c1&redirect_uri=https%3A%2F%2Fapp.example%2Fcb&scope=orca%3Aread&state=xyz" +
	"&code_challenge=abcdefghijklmnopqrstuvwxyzabcdefghijklmnopq&code_challenge_method=S256&resource=https%3A%2F%2Forca.example.com%2Fmcp"

func okInfo(*authv1.OAuthValidateAuthorizeRequestRequest) (*authv1.OAuthAuthorizeRequestInfo, error) {
	return &authv1.OAuthAuthorizeRequestInfo{ClientId: "c1", RedirectUri: "https://app.example/cb"}, nil
}

func newOAuthRouter(a *fakeOAuthAuth, c *fakeConsent) http.Handler {
	routes := &OAuthRoutes{
		Auth: a, Consent: c, Issuer: "https://orca.example.com", DCREnabled: true,
		CookieValidator: fakeCookie{id: wscompat.Identity{TenantID: "t1", UserID: "u1", Role: "user"}},
	}
	return NewRouter(Deps{Registry: domain.NewDefaultServiceRegistry(), OAuth: routes})
}

func doAuthorize(h http.Handler, query string, withCookie bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query, nil)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: "orca_session", Value: "s"})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthorize_NoCookieRedirectsToLoginWithRelativeReturnTo(t *testing.T) {
	c := &fakeConsent{}
	h := newOAuthRouter(&fakeOAuthAuth{validate: okInfo}, c)
	rec := doAuthorize(h, validAuthorizeQuery, false)
	if rec.Code != http.StatusFound {
		t.Fatalf("code = %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?return_to=") {
		t.Fatalf("location = %q", loc)
	}
	u, _ := url.Parse(loc)
	rt := u.Query().Get("return_to")
	if !strings.HasPrefix(rt, "/oauth/authorize?") || strings.Contains(rt, "://orca") || strings.HasPrefix(rt, "//") {
		t.Fatalf("return_to must be a relative authorize URL, got %q", rt)
	}
	if rt != "/oauth/authorize?"+validAuthorizeQuery {
		t.Fatalf("return_to lost the original request: %q", rt)
	}
	if c.calls != 0 {
		t.Fatal("no consent request may be created without a session")
	}
	for k, v := range map[string]string{"Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY"} {
		if rec.Header().Get(k) != v {
			t.Errorf("header %s = %q", k, rec.Header().Get(k))
		}
	}
}

func TestAuthorize_BearerIsNotASession(t *testing.T) {
	h := newOAuthRouter(&fakeOAuthAuth{validate: okInfo}, &fakeConsent{})
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+validAuthorizeQuery, nil)
	req.Header.Set("Authorization", "Bearer something")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.HasPrefix(rec.Header().Get("Location"), "/login?return_to=") {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
}

func TestAuthorize_WithSessionRedirectsToConsentAndForwardsIdentity(t *testing.T) {
	c := &fakeConsent{resp: &mcpv1.CreateConsentRequestResponse{RequestId: "req-1"}}
	h := newOAuthRouter(&fakeOAuthAuth{validate: okInfo}, c)
	rec := doAuthorize(h, validAuthorizeQuery, true)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/oauth/consent?request_id=req-1" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	if got := c.lastMD.Get("x-orca-user-id"); len(got) == 0 || got[0] != "u1" {
		t.Fatalf("identity metadata = %v", c.lastMD)
	}
	if c.lastReq.GetState() != "xyz" || c.lastReq.GetClientId() != "c1" {
		t.Fatalf("request = %+v", c.lastReq)
	}
}

func TestAuthorize_AutoApproveRedirectsToClient(t *testing.T) {
	c := &fakeConsent{resp: &mcpv1.CreateConsentRequestResponse{RedirectUrl: "https://app.example/cb?code=C&state=xyz&iss=https%3A%2F%2Forca.example.com"}}
	rec := doAuthorize(newOAuthRouter(&fakeOAuthAuth{validate: okInfo}, c), validAuthorizeQuery, true)
	if rec.Header().Get("Location") != c.resp.RedirectUrl {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
}

// The open-redirect guard: a bad client or redirect_uri is a static 400 page,
// never a redirect (not even to the login page, and not to the supplied URI).
func TestAuthorize_InvalidClientOrRedirectIsStaticPageNeverRedirect(t *testing.T) {
	for _, code := range []string{"OAUTH_INVALID_CLIENT", "OAUTH_INVALID_REDIRECT_URI", "OAUTH_CLIENT_NOT_FOUND"} {
		a := &fakeOAuthAuth{validate: func(*authv1.OAuthValidateAuthorizeRequestRequest) (*authv1.OAuthAuthorizeRequestInfo, error) {
			return nil, status.Error(codes.InvalidArgument, code+": nope")
		}}
		c := &fakeConsent{}
		for _, cookie := range []bool{false, true} {
			rec := doAuthorize(newOAuthRouter(a, c), "client_id=c1&redirect_uri=https%3A%2F%2Fevil.example%2Fcb&state=s", cookie)
			if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
				t.Fatalf("%s cookie=%v: %d location=%q", code, cookie, rec.Code, rec.Header().Get("Location"))
			}
			if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") || strings.Contains(rec.Body.String(), "evil.example") {
				t.Fatalf("static page must not echo attacker input: %q", rec.Body.String())
			}
		}
		if c.calls != 0 {
			t.Fatal("must not reach mcp-service")
		}
	}
}

func TestAuthorize_OtherOAuthErrorsRedirectWithStateAndIss(t *testing.T) {
	cases := map[string]string{
		"OAUTH_INVALID_SCOPE": "invalid_scope", "OAUTH_INVALID_TARGET": "invalid_target", "OAUTH_INVALID_REQUEST": "invalid_request",
		"OAUTH_UNSUPPORTED_RESPONSE_TYPE": "unsupported_response_type",
	}
	for code, want := range cases {
		a := &fakeOAuthAuth{validate: func(*authv1.OAuthValidateAuthorizeRequestRequest) (*authv1.OAuthAuthorizeRequestInfo, error) {
			return nil, status.Error(codes.InvalidArgument, code+": bad parameter")
		}}
		// No session: the error must still be reported to the (vetted) client, not hidden behind a login.
		rec := doAuthorize(newOAuthRouter(a, &fakeConsent{}), validAuthorizeQuery, false)
		u, err := url.Parse(rec.Header().Get("Location"))
		if rec.Code != http.StatusFound || err != nil || u.Host != "app.example" || u.Path != "/cb" {
			t.Fatalf("%s: %d %q", code, rec.Code, rec.Header().Get("Location"))
		}
		q := u.Query()
		if q.Get("error") != want || q.Get("state") != "xyz" || q.Get("iss") != "https://orca.example.com" {
			t.Fatalf("%s: query = %v", code, q)
		}
	}
}

func TestAuthorize_ClientNotAllowedBecomesAccessDeniedRedirect(t *testing.T) {
	c := &fakeConsent{err: status.Error(codes.PermissionDenied, "MCP_CLIENT_NOT_ALLOWED: client is blocked")}
	rec := doAuthorize(newOAuthRouter(&fakeOAuthAuth{validate: okInfo}, c), validAuthorizeQuery, true)
	u, _ := url.Parse(rec.Header().Get("Location"))
	if u.Host != "app.example" || u.Query().Get("error") != "access_denied" || u.Query().Get("state") != "xyz" || u.Query().Get("iss") == "" {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
}

func TestAuthorize_InfrastructureFailureIsStaticPage(t *testing.T) {
	c := &fakeConsent{err: status.Error(codes.Unavailable, "dial tcp 10.0.0.5:9090: refused")}
	rec := doAuthorize(newOAuthRouter(&fakeOAuthAuth{validate: okInfo}, c), validAuthorizeQuery, true)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Location") != "" || strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("%d %q %q", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
}

func TestToken_SuccessHeadersAndFormOnly(t *testing.T) {
	var got *authv1.OAuthExchangeTokenRequest
	a := &fakeOAuthAuth{exchange: func(in *authv1.OAuthExchangeTokenRequest) (*authv1.OAuthTokenResponse, error) {
		got = in
		return &authv1.OAuthTokenResponse{AccessToken: "AT", ExpiresIn: 600, RefreshToken: "RT", Scope: "orca:read"}, nil
	}}
	h := newOAuthRouter(a, &fakeConsent{})
	body := url.Values{"grant_type": {"authorization_code"}, "code": {"C"}, "redirect_uri": {"https://app.example/cb"}, "code_verifier": {"V"}, "client_id": {"c1"}, "resource": {"R"}}
	// A code smuggled in the query string must be ignored: query strings end up in logs.
	req := httptest.NewRequest(http.MethodPost, "/oauth/token?code=LEAK&refresh_token=LEAK", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	if got.GetCode() != "C" || got.GetRefreshToken() != "" || got.GetCodeVerifier() != "V" || got.GetResource() != "R" {
		t.Fatalf("exchange request = %+v", got)
	}
	for _, want := range []string{`"access_token":"AT"`, `"token_type":"Bearer"`, `"expires_in":600`, `"refresh_token":"RT"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing %s in %s", want, rec.Body.String())
		}
	}
}

func TestToken_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		name   string
	}{
		{status.Error(codes.InvalidArgument, "OAUTH_INVALID_GRANT: authorization code has expired"), 400, "invalid_grant"},
		{status.Error(codes.InvalidArgument, "OAUTH_INVALID_REQUEST: code is required"), 400, "invalid_request"},
		{status.Error(codes.InvalidArgument, "OAUTH_INVALID_TARGET: bad resource"), 400, "invalid_target"},
		{status.Error(codes.InvalidArgument, "OAUTH_INVALID_SCOPE: too wide"), 400, "invalid_scope"},
		{status.Error(codes.InvalidArgument, "OAUTH_UNSUPPORTED_GRANT_TYPE: nope"), 400, "unsupported_grant_type"},
		{status.Error(codes.Unauthenticated, "OAUTH_INVALID_CLIENT: unknown client"), 401, "invalid_client"},
		{status.Error(codes.Internal, "OAUTH_INTERNAL: pq: relation \"auth.x\" does not exist"), 500, "server_error"},
		{status.Error(codes.Unknown, "something with secret-host:5432"), 500, "server_error"},
		{status.Error(codes.Unavailable, "dial tcp secret-host:9090"), 503, "temporarily_unavailable"},
	}
	for _, c := range cases {
		a := &fakeOAuthAuth{exchange: func(*authv1.OAuthExchangeTokenRequest) (*authv1.OAuthTokenResponse, error) { return nil, c.err }}
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader("grant_type=authorization_code&client_id=c1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		newOAuthRouter(a, &fakeConsent{}).ServeHTTP(rec, req)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), `"error":"`+c.name+`"`) {
			t.Errorf("%v: %d %s", c.err, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" || strings.Contains(rec.Body.String(), "secret-host") || strings.Contains(rec.Body.String(), "auth.x") {
			t.Errorf("headers/leak: %v %s", rec.Header(), rec.Body.String())
		}
		if c.status == 401 && rec.Header().Get("WWW-Authenticate") == "" {
			t.Error("401 needs WWW-Authenticate")
		}
	}
}

func TestToken_RateLimited(t *testing.T) {
	a := &fakeOAuthAuth{exchange: func(*authv1.OAuthExchangeTokenRequest) (*authv1.OAuthTokenResponse, error) {
		return &authv1.OAuthTokenResponse{AccessToken: "AT"}, nil
	}}
	routes := &OAuthRoutes{Auth: a, Consent: &fakeConsent{}, TokenLimiter: usecase.NewRateLimiter(0.001, 2)}
	h := NewRouter(Deps{Registry: domain.NewDefaultServiceRegistry(), OAuth: routes})
	codesSeen := []int{}
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader("grant_type=refresh_token&client_id=c1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codesSeen = append(codesSeen, rec.Code)
	}
	if codesSeen[0] != 200 || codesSeen[3] != 429 {
		t.Fatalf("codes = %v", codesSeen)
	}
}

func TestRegister_CreatedAndErrors(t *testing.T) {
	a := &fakeOAuthAuth{register: func(in *authv1.OAuthRegisterClientRequest) (*authv1.OAuthRegisterClientResponse, error) {
		return &authv1.OAuthRegisterClientResponse{ClientId: "new", ClientIdIssuedAt: 1700000000, ClientName: in.GetClientName(),
			RedirectUris: in.GetRedirectUris(), TokenEndpointAuthMethod: "none", GrantTypes: []string{"authorization_code"}, ResponseTypes: []string{"code"}}, nil
	}}
	post := func(a *fakeOAuthAuth, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newOAuthRouter(a, &fakeConsent{}).ServeHTTP(rec, req)
		return rec
	}
	rec := post(a, `{"client_name":"X","redirect_uris":["https://x.example/cb"],"scope":"ignored","software_id":"abc"}`)
	if rec.Code != 201 || !strings.Contains(rec.Body.String(), `"client_id":"new"`) || strings.Contains(rec.Body.String(), "client_secret") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	bad := map[string]struct {
		err    error
		status int
		name   string
	}{
		"redirect": {status.Error(codes.InvalidArgument, "OAUTH_INVALID_REDIRECT_URI: wildcard"), 400, "invalid_redirect_uri"},
		"metadata": {status.Error(codes.InvalidArgument, "OAUTH_INVALID_CLIENT_METADATA: bad name"), 400, "invalid_client_metadata"},
		"disabled": {status.Error(codes.PermissionDenied, "OAUTH_DCR_DISABLED: off"), 403, "access_denied"},
		"limit":    {status.Error(codes.ResourceExhausted, "OAUTH_DCR_LIMIT_REACHED: full"), 429, "temporarily_unavailable"},
	}
	for name, c := range bad {
		a := &fakeOAuthAuth{register: func(*authv1.OAuthRegisterClientRequest) (*authv1.OAuthRegisterClientResponse, error) {
			return nil, c.err
		}}
		rec := post(a, `{"redirect_uris":["x"]}`)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), `"error":"`+c.name+`"`) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	if rec := post(a, `not json`); rec.Code != 400 {
		t.Errorf("garbage body: %d", rec.Code)
	}
	if rec := post(a, `{"client_name":"`+strings.Repeat("a", 9000)+`"}`); rec.Code != 400 {
		t.Errorf("oversized body: %d", rec.Code)
	}
}

func TestRegister_RateLimitedPerIP(t *testing.T) {
	a := &fakeOAuthAuth{register: func(*authv1.OAuthRegisterClientRequest) (*authv1.OAuthRegisterClientResponse, error) {
		return &authv1.OAuthRegisterClientResponse{ClientId: "n"}, nil
	}}
	routes := &OAuthRoutes{Auth: a, Consent: &fakeConsent{}, DCREnabled: true, RegisterLimiter: usecase.NewRateLimiter(0.0001, 3)}
	h := NewRouter(Deps{Registry: domain.NewDefaultServiceRegistry(), OAuth: routes})
	last := 0
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"redirect_uris":["https://x/cb"]}`))
		req.RemoteAddr = "203.0.113.9:" + string(rune('0'+i)) + "000" // different ephemeral ports, same host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("last = %d", last)
	}
}

func TestRegister_AbsentWhenDCRDisabled(t *testing.T) {
	routes := &OAuthRoutes{Auth: &fakeOAuthAuth{}, Consent: &fakeConsent{}, DCREnabled: false}
	rec := httptest.NewRecorder()
	NewRouter(Deps{Registry: domain.NewDefaultServiceRegistry(), OAuth: routes}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{}`)))
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestRevoke_AlwaysOKExceptInfrastructureFailure(t *testing.T) {
	do := func(err error) int {
		a := &fakeOAuthAuth{revoke: func(*authv1.OAuthRevokeTokenRequest) error { return err }}
		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader("token=whatever&client_id=c1"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		newOAuthRouter(a, &fakeConsent{}).ServeHTTP(rec, req)
		return rec.Code
	}
	if c := do(nil); c != 200 {
		t.Errorf("ok: %d", c)
	}
	if c := do(status.Error(codes.InvalidArgument, "OAUTH_INVALID_REQUEST: weird")); c != 200 {
		t.Errorf("unknown token must still be 200: %d", c)
	}
	if c := do(status.Error(codes.Unavailable, "down")); c != 503 {
		t.Errorf("unavailable: %d", c)
	}
}
