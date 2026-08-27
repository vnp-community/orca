package httpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

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
	mountAuthRoutes(r, client, fakeSSOCookieValidator{})

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
	mountAuthRoutes(r, client, fakeSSOCookieValidator{})

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

func TestAuthSSORoute_Returns501(t *testing.T) {
	r := chi.NewRouter()
	mountAuthRoutes(r, &fakeAdminAuthServiceClient{}, fakeSSOCookieValidator{})

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
	mountAuthRoutes(r, &fakeAdminAuthServiceClient{}, fakeSSOCookieValidator{})

	for _, provider := range []string{"google", "github", "okta"} {
		req := httptest.NewRequest(http.MethodGet, "/auth/sso/"+provider, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotImplemented {
			t.Errorf("provider %q: status = %d, want %d", provider, w.Code, http.StatusNotImplemented)
		}
	}
}
