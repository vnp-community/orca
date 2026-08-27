package httpgateway

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// TestNewAgentProxyHandler_ForwardsPathMethodAndBody exercises the proxy in
// isolation (no router involved) against a fake infra-fleet-service HTTP
// server, confirming it's a genuine byte-for-byte pass-through: same path,
// method, and body land on the backend unchanged.
func TestNewAgentProxyHandler_ForwardsPathMethodAndBody(t *testing.T) {
	var gotPath, gotMethod, gotBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"agt-test"}`))
	}))
	defer backend.Close()

	proxy := NewAgentProxyHandler(strings.TrimPrefix(backend.URL, "http://"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agent-token", strings.NewReader(`{"devServerId":"dev-01"}`))
	proxy.ServeHTTP(rec, req)

	if gotMethod != http.MethodPost {
		t.Fatalf("backend saw method %q, want POST", gotMethod)
	}
	if gotPath != "/api/agent-token" {
		t.Fatalf("backend saw path %q, want /api/agent-token", gotPath)
	}
	if gotBody != `{"devServerId":"dev-01"}` {
		t.Fatalf("backend saw body %q", gotBody)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if rec.Body.String() != `{"token":"agt-test"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestRouter_AgentProxyRoutes confirms /agent and /api/agent-token are
// mounted unauthenticated (no Authorization header needed, unlike every
// /v1/* route — see NewRouter's doc comment) when AgentProxyHandler is set,
// and simply absent (404, not 401) when it's nil — matching every other
// optional-downstream degrade-not-panic convention in this router.
func TestRouter_AgentProxyRoutes(t *testing.T) {
	auth := newTestAuth(t)

	t.Run("mounted and unauthenticated", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()

		router := NewRouter(Deps{
			Logger:            slog.Default(),
			Registry:          domain.NewDefaultServiceRegistry(),
			AuthValidator:     usecase.NewAuthValidator(auth.jwks),
			RateLimiter:       usecase.NewRateLimiter(1000, 1000),
			AgentProxyHandler: NewAgentProxyHandler(strings.TrimPrefix(backend.URL, "http://")),
		})

		for _, tc := range []struct {
			method, path string
		}{
			{http.MethodGet, "/agent"},
			{http.MethodGet, "/api/agent-token"},
			{http.MethodPost, "/api/agent-token"},
		} {
			rec := httptest.NewRecorder()
			// No Authorization header — the Dev Server Agent has no user
			// session/JWT to present, unlike every /v1/* route.
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusNotFound {
				t.Fatalf("%s %s: status = %d, want neither 401 nor 404", tc.method, tc.path, rec.Code)
			}
		}
	})

	t.Run("nil handler leaves routes unmounted", func(t *testing.T) {
		router := NewRouter(Deps{
			Logger:        slog.Default(),
			Registry:      domain.NewDefaultServiceRegistry(),
			AuthValidator: usecase.NewAuthValidator(auth.jwks),
			RateLimiter:   usecase.NewRateLimiter(1000, 1000),
			// AgentProxyHandler intentionally nil.
		})

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/agent", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (route should not be mounted)", rec.Code, http.StatusNotFound)
		}
	})
}
