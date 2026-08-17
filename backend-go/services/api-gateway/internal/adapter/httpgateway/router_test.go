package httpgateway

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// fakeJWT builds a structurally-valid (unsigned) JWT carrying the given
// claims — enough to pass AuthValidator's unverified-parsing placeholder,
// same as usecase's own test helper (kept package-local to avoid an
// internal test-only export).
func fakeJWT(claimsJSON string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(claimsJSON))
	return header + "." + payload + ".fakesig"
}

func testRouter() http.Handler {
	return NewRouter(Deps{
		Logger:        slog.Default(),
		Registry:      domain.NewDefaultServiceRegistry(),
		AuthValidator: usecase.NewAuthValidator(),
		RateLimiter:   usecase.NewRateLimiter(1000, 1000), // effectively unlimited for routing tests
		UsageClient:   nil,                                // not exercised by these tests
	})
}

func authedRequest(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("Authorization", "Bearer "+fakeJWT(`{"tenant_id":"tenant-1","sub":"user-1"}`))
	return r
}

func TestRouter_StubbedServiceReturns501WithExplanatoryBody(t *testing.T) {
	router := testRouter()

	cases := []struct {
		name        string
		path        string
		wantService string
	}{
		{"task-service catch-all", "/v1/tasks/123", "task-service"},
		{"task-service bare prefix", "/v1/tasks", "task-service"},
		{"project-service", "/v1/projects/abc/tasks", "project-service"},
		{"notification-service REST (WS is the only real path)", "/v1/notifications/preferences", "notification-service"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, authedRequest(http.MethodGet, tc.path))

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
			}

			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body is not the expected JSON shape: %v; body=%s", err, rec.Body.String())
			}
			if body.Error.Code != "NOT_IMPLEMENTED" {
				t.Fatalf("error.code = %q, want NOT_IMPLEMENTED", body.Error.Code)
			}
			if !strings.Contains(body.Error.Message, tc.wantService) {
				t.Fatalf("error.message = %q, want it to mention %q", body.Error.Message, tc.wantService)
			}
			if !strings.Contains(body.Error.Message, "once its gRPC contract stabilizes") {
				t.Fatalf("error.message = %q, want it to explain the stub", body.Error.Message)
			}
		})
	}
}

func TestRouter_UnauthenticatedRequestReturns401(t *testing.T) {
	router := testRouter()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tasks/123", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
