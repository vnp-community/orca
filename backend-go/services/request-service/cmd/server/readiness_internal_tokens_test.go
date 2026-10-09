package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/common/health"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
)

func readyzOf(cfg config.Config) (int, map[string]string) {
	h := health.New()
	registerInternalTokenReadiness(h, cfg)
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestReadyzReflectsInternalTokens(t *testing.T) {
	cases := []struct {
		name         string
		gateway, svc string
		wantCode     int
	}{
		{"both set", "g", "s", http.StatusOK},
		{"gateway token missing", "", "s", http.StatusServiceUnavailable},
		{"service token missing", "g", "", http.StatusServiceUnavailable},
		{"both missing", "", "", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, body := readyzOf(config.Config{GatewayInternalToken: tc.gateway, ServiceInternalToken: tc.svc})
			if code != tc.wantCode {
				t.Fatalf("status = %d, want %d (%v)", code, tc.wantCode, body)
			}
			if tc.wantCode != http.StatusOK && body["internal_tokens"] == "ok" {
				t.Fatalf("the failing check must be named, got %v", body)
			}
		})
	}
}
