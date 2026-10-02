package config

import (
	"strings"
	"testing"
	"time"
)

func clearMCPEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"MCP_ENABLED", "MCP_PUBLIC_BASE_URL", "MCP_ISSUER_URL", "MCP_ALLOWED_ORIGINS", "MCP_MAX_REQUEST_BYTES",
		"MCP_SESSION_IDLE_TTL", "MCP_READ_HEADER_TIMEOUT", "MCP_SERVICE_ADDR", "MCP_CURSOR_KEY",
		"MCP_CURSOR_KEY_PREVIOUS", "MCP_CURSOR_KEY_FILE", "WS_ALLOWED_ORIGINS", "PUBLIC_BASE_URL",
		"MCP_MAX_SSE_STREAMS_PER_USER", "MCP_MAX_SSE_STREAMS_PER_TENANT",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadMCP_Defaults(t *testing.T) {
	clearMCPEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := cfg.MCP
	if m.Enabled {
		t.Error("MCP must be disabled by default")
	}
	if m.MaxRequestBytes != 1<<20 || m.SessionIdleTTL != 30*time.Minute || m.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("unexpected defaults: %+v", m)
	}
	if m.MaxSSEStreamsPerUser != 5 || m.MaxSSEStreamsPerTenant != 200 {
		t.Errorf("unexpected stream caps: %+v", m)
	}
}

func TestLoadMCP_InheritsPublicBaseAndOrigins(t *testing.T) {
	clearMCPEnv(t)
	t.Setenv("PUBLIC_BASE_URL", "https://orca.example.com/")
	t.Setenv("WS_ALLOWED_ORIGINS", "https://orca.example.com,https://*.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.MCP.ResourceURL(); got != "https://orca.example.com/mcp" {
		t.Errorf("ResourceURL = %q", got)
	}
	if cfg.MCP.IssuerURL != "https://orca.example.com" {
		t.Errorf("IssuerURL = %q", cfg.MCP.IssuerURL)
	}
	if cfg.MCP.AllowedOrigins != cfg.WSAllowedOrigins {
		t.Errorf("MCP_ALLOWED_ORIGINS must default to WS_ALLOWED_ORIGINS, got %q", cfg.MCP.AllowedOrigins)
	}
}

func TestLoadMCP_ExplicitOverrides(t *testing.T) {
	clearMCPEnv(t)
	t.Setenv("PUBLIC_BASE_URL", "https://orca.example.com")
	t.Setenv("WS_ALLOWED_ORIGINS", "https://ws.example.com")
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_BASE_URL", "https://mcp.example.com")
	t.Setenv("MCP_ISSUER_URL", "https://auth.example.com")
	t.Setenv("MCP_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("MCP_MAX_REQUEST_BYTES", "2048")
	t.Setenv("MCP_SESSION_IDLE_TTL", "5m")
	t.Setenv("MCP_SERVICE_ADDR", "mcp-service:9090")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := cfg.MCP
	if !m.Enabled || m.ResourceURL() != "https://mcp.example.com/mcp" || m.IssuerURL != "https://auth.example.com" ||
		m.AllowedOrigins != "https://app.example.com" || m.MaxRequestBytes != 2048 ||
		m.SessionIdleTTL != 5*time.Minute || m.ServiceAddr != "mcp-service:9090" {
		t.Errorf("unexpected: %+v", m)
	}
}

func TestLoadMCP_Invalid(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"bad bool", map[string]string{"MCP_ENABLED": "maybe"}, "MCP_ENABLED"},
		{"bad duration", map[string]string{"MCP_SESSION_IDLE_TTL": "soon"}, "MCP_SESSION_IDLE_TTL"},
		{"bad bytes", map[string]string{"MCP_MAX_REQUEST_BYTES": "0"}, "MCP_MAX_REQUEST_BYTES"},
		{"origin with path", map[string]string{"MCP_ALLOWED_ORIGINS": "https://a.example.com/x"}, "MCP_ALLOWED_ORIGINS"},
		{"enabled without base url", map[string]string{"MCP_ENABLED": "true"}, "MCP_PUBLIC_BASE_URL"},
		{"relative base url", map[string]string{"MCP_PUBLIC_BASE_URL": "orca.example.com"}, "MCP_PUBLIC_BASE_URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearMCPEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error mentioning %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLoadMCP_EnabledWithoutServiceAddrDegrades(t *testing.T) {
	clearMCPEnv(t)
	t.Setenv("MCP_ENABLED", "true")
	t.Setenv("MCP_PUBLIC_BASE_URL", "https://orca.example.com")
	if _, err := Load(); err != nil {
		t.Fatalf("empty MCP_SERVICE_ADDR must degrade, not fail: %v", err)
	}
}
