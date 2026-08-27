package httpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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
