package httpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
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
