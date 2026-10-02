package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/httpgateway"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

func TestAuthorizationServerMetadataServedFromConfig(t *testing.T) {
	cfg := mcpTestConfig(true)
	cfg.DCREnabled = true
	h, err := buildMCPHandler(cfg, quietLogger, usecase.NewRateLimiter(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	r := httpgateway.NewRouter(httpgateway.Deps{Logger: quietLogger, Registry: domain.NewDefaultServiceRegistry(), MCP: h})
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	req.Host = "evil.example" // the issuer must come from config, never from Host
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"issuer": "https://orca.example.com", "authorization_endpoint": "https://orca.example.com/oauth/authorize",
		"token_endpoint": "https://orca.example.com/oauth/token", "revocation_endpoint": "https://orca.example.com/oauth/revoke",
		"registration_endpoint": "https://orca.example.com/oauth/register",
	} {
		if doc[k] != want {
			t.Errorf("%s = %v, want %s", k, doc[k], want)
		}
	}
	if _, has := doc["jwks_uri"]; has {
		t.Error("no jwks_uri")
	}

	cfg.DCREnabled = false
	h, _ = buildMCPHandler(cfg, quietLogger, usecase.NewRateLimiter(100, 100))
	rec = httptest.NewRecorder()
	httpgateway.NewRouter(httpgateway.Deps{Logger: quietLogger, Registry: domain.NewDefaultServiceRegistry(), MCP: h}).ServeHTTP(rec, req)
	var off map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &off); err != nil {
		t.Fatal(err)
	}
	if _, has := off["registration_endpoint"]; has {
		t.Fatal("registration_endpoint must be absent when DCR is disabled")
	}
}
