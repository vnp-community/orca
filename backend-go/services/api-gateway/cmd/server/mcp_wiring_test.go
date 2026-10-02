package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/httpgateway"
	svcconfig "github.com/stablyai/orca-go/services/api-gateway/internal/config"
	"github.com/stablyai/orca-go/services/api-gateway/internal/domain"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func mcpTestConfig(enabled bool) svcconfig.MCPConfig {
	return svcconfig.MCPConfig{
		Enabled: enabled, PublicBaseURL: "https://orca.example.com", IssuerURL: "https://orca.example.com",
		MaxRequestBytes: 1 << 20, SessionIdleTTL: time.Minute, ReadHeaderTimeout: time.Second,
	}
}

func routerFor(t *testing.T, enabled bool) http.Handler {
	t.Helper()
	h, err := buildMCPHandler(mcpTestConfig(enabled), quietLogger, usecase.NewRateLimiter(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	return httpgateway.NewRouter(httpgateway.Deps{Logger: quietLogger, Registry: domain.NewDefaultServiceRegistry(), MCP: h})
}

func TestMCPDisabledRoutesAre404(t *testing.T) {
	r := routerFor(t, false)
	for _, p := range []string{"/mcp", "/mcp/", "/.well-known/oauth-protected-resource"} {
		for _, m := range []string{http.MethodGet, http.MethodPost} {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(m, p, nil))
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s with MCP disabled: %d, want 404", m, p, rec.Code)
			}
		}
	}
}

func TestMCPEnabledChallengesWithoutToken(t *testing.T) {
	r := routerFor(t, true)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Header().Get("WWW-Authenticate"), "resource_metadata=") {
		t.Errorf("got %d %q", rec.Code, rec.Header().Get("WWW-Authenticate"))
	}
	// Metadata is public.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"resource":"https://orca.example.com/mcp"`) {
		t.Errorf("metadata: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLoadMCPCursorKeys(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "keys")
	if err := os.WriteFile(f, []byte("current\n\nprevious\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := loadMCPCursorKeys(svcconfig.MCPConfig{CursorKeyFile: f, CursorKey: "ignored"})
	if err != nil || len(keys) != 2 || string(keys[0]) != "current" || string(keys[1]) != "previous" {
		t.Fatalf("file keys: %q %v", keys, err)
	}
	if keys, err = loadMCPCursorKeys(svcconfig.MCPConfig{CursorKey: "a", CursorKeyPrevious: "b"}); err != nil || len(keys) != 2 {
		t.Fatalf("env keys: %q %v", keys, err)
	}
	if keys, err = loadMCPCursorKeys(svcconfig.MCPConfig{}); err != nil || len(keys) != 0 {
		t.Fatalf("no keys must be allowed: %q %v", keys, err)
	}
	if _, err = loadMCPCursorKeys(svcconfig.MCPConfig{CursorKeyFile: filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("unreadable key file must fail startup")
	}
}
