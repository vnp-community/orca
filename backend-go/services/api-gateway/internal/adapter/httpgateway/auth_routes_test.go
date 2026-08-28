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

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeSSOCookieValidator is a minimal CookieSessionValidator stand-in for
// this file's routes — /auth/sso/:provider never consults it (the route is
// a static 501 stub), but mountAuthRoutes requires one to construct.
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
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter())

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
	mountAuthRoutes(r, client, fakeSSOCookieValidator{}, generousLoginLimiter())

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

func TestAuthSSORoute_Returns501(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeAdminAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter())

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
	if body.Error.Code != "NOT_IMPLEMENTED" {
		t.Errorf("error.code = %q, want %q", body.Error.Code, "NOT_IMPLEMENTED")
	}
}

func TestAuthSSORoute_AnyProviderReturns501(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeAdminAuthServiceClient{}, fakeSSOCookieValidator{}, generousLoginLimiter())

	for _, provider := range []string{"google", "github", "okta"} {
		req := httptest.NewRequest(http.MethodGet, "/auth/sso/"+provider, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Errorf("provider %q: status = %d, want %d", provider, w.Code, http.StatusNotImplemented)
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
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter())

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
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter())

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
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter())

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
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, generousLoginLimiter())

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
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, usecase.NewRateLimiter(10.0/60.0, 10))

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
	mountAuthRoutes(r, fake, fakeSSOCookieValidator{}, usecase.NewRateLimiter(10.0/60.0, 10))

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
