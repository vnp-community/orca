package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeSSOCookieValidator is a minimal CookieSessionValidator stand-in for
// this file's routes — none of them consult it (login/SSO endpoints are the
// unauthenticated entry points themselves), but mountAuthRoutes requires
// one to construct.
type fakeSSOCookieValidator struct {
	err error
}

func (f fakeSSOCookieValidator) ValidateCookie(_ context.Context, _ *http.Request) (wscompat.Identity, error) {
	if f.err != nil {
		return wscompat.Identity{}, f.err
	}
	return wscompat.Identity{}, nil
}

// fakeCLITokenAuthServiceClient overrides Login/IssueServiceToken on top of
// fakeAdminAuthServiceClient's Unimplemented stubs — TestAuthCLITokenRoute's
// cases need configurable responses/errors for exactly those two RPCs, and
// track whether IssueServiceToken was called at all (invalid credentials
// must short-circuit before ever reaching it).
type fakeCLITokenAuthServiceClient struct {
	fakeAdminAuthServiceClient

	loginResp *authv1.LoginResponse
	loginErr  error

	issueServiceTokenResp   *authv1.IssueServiceTokenResponse
	issueServiceTokenErr    error
	issueServiceTokenCalled bool
}

func (f *fakeCLITokenAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
	return f.loginResp, f.loginErr
}

func (f *fakeCLITokenAuthServiceClient) IssueServiceToken(ctx context.Context, in *authv1.IssueServiceTokenRequest, opts ...grpc.CallOption) (*authv1.IssueServiceTokenResponse, error) {
	f.issueServiceTokenCalled = true
	return f.issueServiceTokenResp, f.issueServiceTokenErr
}

// TestAuthCLITokenRoute_ValidCredentials_ReturnsJWTWithoutCookie proves
// POST /auth/cli-token returns {jwt, expires_at, user} and — unlike
// /auth/local — sets NO Set-Cookie header, since a CLI/CI caller can't use
// one.
func TestAuthCLITokenRoute_ValidCredentials_ReturnsJWTWithoutCookie(t *testing.T) {
	client := &fakeCLITokenAuthServiceClient{
		loginResp: &authv1.LoginResponse{
			User: &authv1.User{Id: "user-1", Email: "dev@example.com", Name: "Dev"},
		},
		issueServiceTokenResp: &authv1.IssueServiceTokenResponse{Jwt: "signed.jwt.token"},
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	bodyJSON := `{"email":"dev@example.com","password":"correct-horse"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/cli-token", strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if cookies := w.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("expected no Set-Cookie header, got %+v", cookies)
	}

	var body struct {
		JWT  string `json:"jwt"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v; body=%s", err, w.Body.String())
	}
	if body.JWT != "signed.jwt.token" {
		t.Fatalf("jwt = %q, want %q", body.JWT, "signed.jwt.token")
	}
	if body.User.ID != "user-1" {
		t.Fatalf("user.id = %q, want %q", body.User.ID, "user-1")
	}
	if !client.issueServiceTokenCalled {
		t.Fatal("expected IssueServiceToken to be called")
	}
}

// TestAuthCLITokenRoute_InvalidCredentials_Returns401WithoutIssuingToken
// proves a failed Login short-circuits before IssueServiceToken is ever
// called.
func TestAuthCLITokenRoute_InvalidCredentials_Returns401WithoutIssuingToken(t *testing.T) {
	client := &fakeCLITokenAuthServiceClient{
		loginErr: context.DeadlineExceeded, // any non-nil error stands in for auth-service's invalid-credentials error
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	bodyJSON := `{"email":"dev@example.com","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/cli-token", strings.NewReader(bodyJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	if client.issueServiceTokenCalled {
		t.Fatal("IssueServiceToken must not be called when Login fails")
	}
}

// generousLoginLimiter returns a login rate limiter effectively unlimited
// for tests that aren't exercising the rate-limiting behavior itself —
// mirrors production's NewRateLimiter shape but with a burst large enough
// that a handful of test requests never trip it.
func generousLoginLimiter() *usecase.RateLimiter {
	return usecase.NewRateLimiter(1000, 1000)
}

// fakeSSOAuthServiceClient implements authv1.AuthServiceClient with
// canned StartSsoLogin/CompleteSsoLogin responses — every other method is
// an unused pass-through, present only because Go requires every interface
// method to compile.
type fakeSSOAuthServiceClient struct {
	authv1.AuthServiceClient // embed: panics on any unimplemented method, intentional — see fakeTenantServiceClient's identical convention

	startResp    *authv1.StartSsoLoginResponse
	startErr     error
	lastStartReq *authv1.StartSsoLoginRequest

	completeResp *authv1.CompleteSsoLoginResponse
	completeErr  error

	refreshResp    *authv1.RefreshSessionResponse
	refreshErr     error
	lastRefreshReq *authv1.RefreshSessionRequest

	// validateResp/validateErr back POST /auth/refresh's follow-up
	// ValidateSession call (RefreshSessionResponse carries no User, unlike
	// Login/CompleteSsoLogin — see auth_routes.go's /auth/refresh handler).
	validateResp *authv1.ValidateSessionResponse
	validateErr  error
}

func (f *fakeSSOAuthServiceClient) Login(ctx context.Context, in *authv1.LoginRequest, opts ...grpc.CallOption) (*authv1.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) Logout(ctx context.Context, in *authv1.LogoutRequest, opts ...grpc.CallOption) (*authv1.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ValidateSession(ctx context.Context, in *authv1.ValidateSessionRequest, opts ...grpc.CallOption) (*authv1.ValidateSessionResponse, error) {
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	if f.validateResp != nil {
		return f.validateResp, nil
	}
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) IssueServiceToken(ctx context.Context, in *authv1.IssueServiceTokenRequest, opts ...grpc.CallOption) (*authv1.IssueServiceTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) GetJWKS(ctx context.Context, in *authv1.GetJWKSRequest, opts ...grpc.CallOption) (*authv1.GetJWKSResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, opts ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, opts ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ListTenantMemberDirectory(ctx context.Context, in *authv1.ListTenantMemberDirectoryRequest, opts ...grpc.CallOption) (*authv1.ListTenantMemberDirectoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) UpdateUserRole(ctx context.Context, in *authv1.UpdateUserRoleRequest, opts ...grpc.CallOption) (*authv1.UpdateUserRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) RevokeSession(ctx context.Context, in *authv1.RevokeSessionRequest, opts ...grpc.CallOption) (*authv1.RevokeSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) QueryAuditLog(ctx context.Context, in *authv1.QueryAuditLogRequest, opts ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, opts ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ReactivateUser(ctx context.Context, in *authv1.ReactivateUserRequest, opts ...grpc.CallOption) (*authv1.ReactivateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ListSessionsForUser(ctx context.Context, in *authv1.ListSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ListSessionsForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ForceRevokeAllSessionsForUser(ctx context.Context, in *authv1.ForceRevokeAllSessionsForUserRequest, opts ...grpc.CallOption) (*authv1.ForceRevokeAllSessionsForUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ForceRevokeSession(ctx context.Context, in *authv1.ForceRevokeSessionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) CreateAccessPolicy(ctx context.Context, in *authv1.CreateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) GetAccessPolicy(ctx context.Context, in *authv1.GetAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) ListAccessPolicies(ctx context.Context, in *authv1.ListAccessPoliciesRequest, opts ...grpc.CallOption) (*authv1.ListAccessPoliciesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) UpdateAccessPolicy(ctx context.Context, in *authv1.UpdateAccessPolicyRequest, opts ...grpc.CallOption) (*authv1.AccessPolicy, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) DeleteAccessPolicy(ctx context.Context, in *authv1.DeleteAccessPolicyRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeSSOAuthServiceClient) GetAdminStats(ctx context.Context, in *authv1.GetAdminStatsRequest, opts ...grpc.CallOption) (*authv1.GetAdminStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeSSOAuthServiceClient) StartSsoLogin(ctx context.Context, in *authv1.StartSsoLoginRequest, opts ...grpc.CallOption) (*authv1.StartSsoLoginResponse, error) {
	f.lastStartReq = in
	if f.startErr != nil {
		return nil, f.startErr
	}
	return f.startResp, nil
}

func (f *fakeSSOAuthServiceClient) CompleteSsoLogin(ctx context.Context, in *authv1.CompleteSsoLoginRequest, opts ...grpc.CallOption) (*authv1.CompleteSsoLoginResponse, error) {
	if f.completeErr != nil {
		return nil, f.completeErr
	}
	return f.completeResp, nil
}

func (f *fakeSSOAuthServiceClient) RefreshSession(ctx context.Context, in *authv1.RefreshSessionRequest, opts ...grpc.CallOption) (*authv1.RefreshSessionResponse, error) {
	f.lastRefreshReq = in
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	return f.refreshResp, nil
}

// AppendAuditEntry/UpdateSsoGroupMapping/ListSsoGroupMapping: unused
// pass-throughs, present only so this fake keeps satisfying
// authv1.AuthServiceClient as the interface grows (TASK-BE-017,
// CR-RBAC-003) — none of this file's tests exercise them.
func (f *fakeSSOAuthServiceClient) AppendAuditEntry(ctx context.Context, in *authv1.AppendAuditEntryRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeSSOAuthServiceClient) IsServiceTokenRevoked(ctx context.Context, in *authv1.IsServiceTokenRevokedRequest, opts ...grpc.CallOption) (*authv1.IsServiceTokenRevokedResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeSSOAuthServiceClient) ListCliTokens(ctx context.Context, in *authv1.ListCliTokensRequest, opts ...grpc.CallOption) (*authv1.ListCliTokensResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeSSOAuthServiceClient) RevokeCliToken(ctx context.Context, in *authv1.RevokeCliTokenRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeSSOAuthServiceClient) UpdateSsoGroupMapping(ctx context.Context, in *authv1.UpdateSsoGroupMappingRequest, opts ...grpc.CallOption) (*authv1.UpdateSsoGroupMappingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

func (f *fakeSSOAuthServiceClient) ListSsoGroupMapping(ctx context.Context, in *authv1.ListSsoGroupMappingRequest, opts ...grpc.CallOption) (*authv1.ListSsoGroupMappingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

var _ authv1.AuthServiceClient = (*fakeSSOAuthServiceClient)(nil)

func TestAuthSSORoute_UnconfiguredReturns501(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeSSOAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/google", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNotImplemented, w.Body.String())
	}
	var body errorBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, w.Body.String())
	}
	if body.Error.Code != "AUTH_SSO_NOT_CONFIGURED" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "AUTH_SSO_NOT_CONFIGURED")
	}
}

func TestAuthSSORoute_RedirectsToAuthorizationURL(t *testing.T) {
	client := &fakeSSOAuthServiceClient{
		startResp: &authv1.StartSsoLoginResponse{AuthorizationUrl: "https://github.com/login/oauth/authorize?state=abc", State: "abc"},
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{PublicBaseURL: "https://app.example.com", GithubClientID: "gh-id"})

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/github", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if got := w.Header().Get("Location"); got != client.startResp.AuthorizationUrl {
		t.Errorf("Location = %q, want %q", got, client.startResp.AuthorizationUrl)
	}
	if client.lastStartReq.GetRedirectUri() != "https://app.example.com/auth/callback" {
		t.Errorf("redirect_uri = %q, want the configured PublicBaseURL + /auth/callback, never derived from the request", client.lastStartReq.GetRedirectUri())
	}
	if client.lastStartReq.GetProvider() != "github" {
		t.Errorf("provider = %q, want %q", client.lastStartReq.GetProvider(), "github")
	}
}

func TestAuthSSORoute_KeycloakURLTranslatesToOidcProviderKey(t *testing.T) {
	client := &fakeSSOAuthServiceClient{startResp: &authv1.StartSsoLoginResponse{AuthorizationUrl: "https://idp.example.com/auth"}}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{PublicBaseURL: "https://app.example.com", OidcClientID: "oidc-id"})

	req := httptest.NewRequest(http.MethodGet, "/auth/sso/keycloak", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if client.lastStartReq.GetProvider() != "oidc" {
		t.Errorf("provider sent to auth-service = %q, want %q (the frontend-facing url segment is \"keycloak\", the wire value is \"oidc\")", client.lastStartReq.GetProvider(), "oidc")
	}
}

func TestAuthSSOCallback_SetsSessionCookieAndRedirects(t *testing.T) {
	client := &fakeSSOAuthServiceClient{
		completeResp: &authv1.CompleteSsoLoginResponse{
			SessionToken: "raw-session-token",
			User:         &authv1.User{Id: "u1", Email: "alice@example.com", Provider: "github"},
		},
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{PublicBaseURL: "https://app.example.com", GithubClientID: "gh-id"})

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=xyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if got := w.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want %q", got, "/")
	}
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value == "raw-session-token" {
			found = true
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
				t.Errorf("orca_session cookie flags = %+v, want HttpOnly+Secure+SameSiteStrict, matching POST /auth/local's setSessionCookie", c)
			}
		}
	}
	if !found {
		t.Error("expected an orca_session cookie to be set to the session token CompleteSsoLogin returned")
	}
}

func TestAuthSSOCallback_FailureRedirectsWithErrorMarker(t *testing.T) {
	client := &fakeSSOAuthServiceClient{completeErr: status.Error(codes.PermissionDenied, "sso state token is invalid, expired, or tampered with")}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{PublicBaseURL: "https://app.example.com"})

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if got := w.Header().Get("Location"); got != "/?ssoError=1" {
		t.Errorf("Location = %q, want %q", got, "/?ssoError=1")
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			t.Error("expected no orca_session cookie to be set on a failed callback")
		}
	}
}

func TestClientIP_PrefersXForwardedForFirstHop(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/local", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	req.RemoteAddr = "5.6.7.8:1234"

	if got := clientIP(req); got != "1.2.3.4" {
		t.Fatalf("clientIP() = %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/local", nil)
	req.RemoteAddr = "5.6.7.8:1234"

	if got := clientIP(req); got != "5.6.7.8" {
		t.Fatalf("clientIP() = %q, want %q", got, "5.6.7.8")
	}
}

func TestClientIP_FallsBackToRemoteAddrWithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/local", nil)
	req.RemoteAddr = "not-a-host-port"

	if got := clientIP(req); got != "not-a-host-port" {
		t.Fatalf("clientIP() = %q, want %q", got, "not-a-host-port")
	}
}

func TestAuthRoutes_LoginSetsSessionCookie(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{
		loginResp: &authv1.LoginResponse{
			SessionToken: "sess-token",
			User:         &authv1.User{Id: "u1", Email: "a@example.com", Name: "A", Role: authv1.Role_ROLE_USER},
		},
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	body, _ := json.Marshal(loginRequestBody{Email: "a@example.com", Password: "pw"})
	req := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if fake.lastLoginReq.GetIp() != "9.9.9.9" {
		t.Fatalf("Login called with Ip = %q, want %q", fake.lastLoginReq.GetIp(), "9.9.9.9")
	}
	found := false
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName && c.Value == "sess-token" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected session cookie %q to be set with value %q; got cookies=%v", sessionCookieName, "sess-token", w.Result().Cookies())
	}
}

func TestAuthRoutes_Login_DeactivatedAccountMapsTo403(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{
		loginErr: status.Error(codes.PermissionDenied, "account is deactivated"),
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	body, _ := json.Marshal(loginRequestBody{Email: "a@example.com", Password: "pw"})
	req := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
	var eb errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &eb); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, w.Body.String())
	}
	if eb.Error.Code != "account_inactive" {
		t.Fatalf("error.code = %q, want %q", eb.Error.Code, "account_inactive")
	}
}

func TestAuthRoutes_Login_WrongPasswordMapsTo401(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{
		loginErr: status.Error(codes.Unauthenticated, "invalid credentials"),
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	body, _ := json.Marshal(loginRequestBody{Email: "a@example.com", Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
	var eb errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &eb); err != nil {
		t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, w.Body.String())
	}
	if eb.Error.Code != "invalid_credentials" {
		t.Fatalf("error.code = %q, want %q", eb.Error.Code, "invalid_credentials")
	}
}

func TestAuthRoutes_Login_UnknownEmailAlsoMapsTo401(t *testing.T) {
	// NotFound (or any other non-PermissionDenied code) must collapse to the
	// same generic 401 as wrong-password — must not leak "user not found"
	// vs "wrong password" distinctions to the client.
	fake := &fakeAdminAuthServiceClient{
		loginErr: status.Error(codes.NotFound, "no such user"),
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	body, _ := json.Marshal(loginRequestBody{Email: "nobody@example.com", Password: "pw"})
	req := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

func TestAuthRoutes_Login_RateLimitedAfter10AttemptsPerIP(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{
		loginErr: status.Error(codes.Unauthenticated, "invalid credentials"),
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, usecase.NewRateLimiter(10.0/60.0, 10), SsoRouteConfig{})

	body, _ := json.Marshal(loginRequestBody{Email: "a@example.com", Password: "wrong"})

	var lastCode int
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
		req.Header.Set("X-Forwarded-For", "3.3.3.3")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		lastCode = w.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("11th request status = %d, want %d", lastCode, http.StatusTooManyRequests)
	}
	if fake.loginCallCount > 10 {
		t.Fatalf("Login called %d times, want <= 10 (rate limiter should have blocked the 11th)", fake.loginCallCount)
	}
}

func TestAuthRoutes_Login_RateLimitTracksIPsIndependently(t *testing.T) {
	fake := &fakeAdminAuthServiceClient{
		loginErr: status.Error(codes.Unauthenticated, "invalid credentials"),
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, usecase.NewRateLimiter(10.0/60.0, 10), SsoRouteConfig{})

	body, _ := json.Marshal(loginRequestBody{Email: "a@example.com", Password: "wrong"})

	// Exhaust IP A's budget.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
		req.Header.Set("X-Forwarded-For", "1.1.1.1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
	reqA := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
	reqA.Header.Set("X-Forwarded-For", "1.1.1.1")
	wA := httptest.NewRecorder()
	r.ServeHTTP(wA, reqA)
	if wA.Code != http.StatusTooManyRequests {
		t.Fatalf("IP A's 11th request status = %d, want %d", wA.Code, http.StatusTooManyRequests)
	}

	// IP B should still have its own untouched budget.
	reqB := httptest.NewRequest(http.MethodPost, "/auth/local", bytes.NewReader(body))
	reqB.Header.Set("X-Forwarded-For", "2.2.2.2")
	wB := httptest.NewRecorder()
	r.ServeHTTP(wB, reqB)
	if wB.Code == http.StatusTooManyRequests {
		t.Fatalf("IP B's first request status = %d, want its own independent budget (not 429)", wB.Code)
	}
}

func TestAuthConfig_OnlyListsProvidersWithClientIDSet(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeSSOAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{
		AuthMode: "both", GithubClientID: "gh-id",
		// GoogleClientID / OidcClientID left empty — must not appear.
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body struct {
		Providers    []string `json:"providers"`
		LocalEnabled bool     `json:"localEnabled"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Providers) != 1 || body.Providers[0] != "github" {
		t.Errorf("providers = %v, want [\"github\"]", body.Providers)
	}
	if !body.LocalEnabled {
		t.Error("localEnabled = false, want true under AuthMode=both")
	}
}

func TestAuthConfig_AuthModeLocal_HidesAllProviders(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeSSOAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{
		AuthMode: "local", GithubClientID: "gh-id", GoogleClientID: "g-id", OidcClientID: "o-id",
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body struct {
		Providers    []string `json:"providers"`
		LocalEnabled bool     `json:"localEnabled"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Providers) != 0 {
		t.Errorf("providers = %v, want none under AuthMode=local even with client ids configured", body.Providers)
	}
	if !body.LocalEnabled {
		t.Error("localEnabled = false, want true under AuthMode=local")
	}
}

func TestAuthConfig_AuthModeSso_DisablesLocal(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeSSOAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{AuthMode: "sso", GithubClientID: "gh-id"})

	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body struct {
		Providers    []string `json:"providers"`
		LocalEnabled bool     `json:"localEnabled"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.LocalEnabled {
		t.Error("localEnabled = true, want false under AuthMode=sso")
	}
	if len(body.Providers) != 1 || body.Providers[0] != "github" {
		t.Errorf("providers = %v, want [\"github\"]", body.Providers)
	}
}

// TASK-BE-012 — POST /auth/refresh

func TestAuthRefresh_NoCookiePresent_Returns401(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeSSOAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (no orca_refresh cookie sent)", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthRefresh_Success_RotatesBothCookiesAndReturnsUser(t *testing.T) {
	client := &fakeSSOAuthServiceClient{
		refreshResp: &authv1.RefreshSessionResponse{
			SessionToken: "new-session-token",
			RefreshToken: "new-refresh-token",
		},
		validateResp: &authv1.ValidateSessionResponse{
			Valid: true,
			User:  &authv1.User{Id: "u1", Email: "alice@example.com", Provider: "github"},
		},
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "old-refresh-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if client.lastRefreshReq.GetRefreshToken() != "old-refresh-token" {
		t.Errorf("RefreshSession called with refresh_token = %q, want the cookie's value", client.lastRefreshReq.GetRefreshToken())
	}

	var gotSession, gotRefresh *http.Cookie
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case sessionCookieName:
			gotSession = c
		case refreshCookieName:
			gotRefresh = c
		}
	}
	if gotSession == nil || gotSession.Value != "new-session-token" {
		t.Errorf("orca_session cookie = %+v, want value %q", gotSession, "new-session-token")
	}
	if gotRefresh == nil || gotRefresh.Value != "new-refresh-token" {
		t.Errorf("orca_refresh cookie = %+v, want value %q (old refresh token must be single-use)", gotRefresh, "new-refresh-token")
	}
	if gotRefresh != nil && gotRefresh.Path != "/auth" {
		t.Errorf("orca_refresh cookie Path = %q, want %q (must not be sent on ordinary app requests)", gotRefresh.Path, "/auth")
	}

	var body authUserResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "u1" || body.Email != "alice@example.com" {
		t.Errorf("body = %+v, want the refreshed session's user", body)
	}
}

func TestAuthRefresh_InvalidToken_ClearsBothCookiesAndReturns401(t *testing.T) {
	client := &fakeSSOAuthServiceClient{
		refreshErr: status.Error(codes.PermissionDenied, "refresh token expired"),
	}
	r := chi.NewRouter()
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter(), SsoRouteConfig{})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "expired-or-reused-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	var sawSessionClear, sawRefreshClear bool
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName && c.MaxAge < 0 {
			sawSessionClear = true
		}
		if c.Name == refreshCookieName && c.MaxAge < 0 {
			sawRefreshClear = true
		}
	}
	if !sawSessionClear || !sawRefreshClear {
		t.Errorf("expected both cookies cleared on a failed refresh, sawSessionClear=%v sawRefreshClear=%v", sawSessionClear, sawRefreshClear)
	}
}
