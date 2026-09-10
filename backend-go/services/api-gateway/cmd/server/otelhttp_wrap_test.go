package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TestOtelhttpWrap_PassesThroughToInnerHandler proves otelhttp.NewHandler
// (TASK-BE-FFT-004's publicServer.Handler wrapper) is purely observational:
// requests still reach the wrapped handler and status/body are unchanged.
func TestOtelhttpWrap_PassesThroughToInnerHandler(t *testing.T) {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("pong"))
	})

	wrapped := otelhttp.NewHandler(inner, "api-gateway")

	directRec := httptest.NewRecorder()
	inner.ServeHTTP(directRec, httptest.NewRequest(http.MethodGet, "/api/ping", nil))

	wrappedRec := httptest.NewRecorder()
	wrapped.ServeHTTP(wrappedRec, httptest.NewRequest(http.MethodGet, "/api/ping", nil))

	if wrappedRec.Code != directRec.Code {
		t.Errorf("status: direct=%d wrapped=%d, want equal", directRec.Code, wrappedRec.Code)
	}
	if wrappedRec.Body.String() != directRec.Body.String() {
		t.Errorf("body: direct=%q wrapped=%q, want equal", directRec.Body.String(), wrappedRec.Body.String())
	}
	if wrappedRec.Header().Get("X-Test") != directRec.Header().Get("X-Test") {
		t.Errorf("X-Test header: direct=%q wrapped=%q, want equal",
			directRec.Header().Get("X-Test"), wrappedRec.Header().Get("X-Test"))
	}
}

// TestOtelhttpWrap_404UnaffectedByWrap is a regression guard: a route the
// inner handler doesn't know about still 404s the same way through the
// wrapper — otelhttp never swallows or rewrites unmatched routes.
func TestOtelhttpWrap_404UnaffectedByWrap(t *testing.T) {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := otelhttp.NewHandler(inner, "api-gateway")

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404 for unmatched route through wrapper, got %d", rec.Code)
	}
}
