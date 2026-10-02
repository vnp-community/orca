package config

import "testing"

func TestLoad_DefaultsTenantEnabledTrue(t *testing.T) {
	t.Setenv("MCP_TENANT_DEFAULT_ENABLED", "")
	t.Setenv("MCP_DEFAULT_MAX_TOKEN_DAYS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.TenantDefaultEnabled || cfg.DefaultMaxTokenDays != 90 {
		t.Fatalf("got enabled=%v days=%d, want true/90 (D6)", cfg.TenantDefaultEnabled, cfg.DefaultMaxTokenDays)
	}
}

func TestLoad_TenantDefaultEnabledFalse(t *testing.T) {
	t.Setenv("MCP_TENANT_DEFAULT_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TenantDefaultEnabled {
		t.Fatal("expected false")
	}
}

func TestLoad_RejectsInvalid(t *testing.T) {
	for k, v := range map[string]string{
		"MCP_TENANT_DEFAULT_ENABLED": "maybe",
		"MCP_DEFAULT_MAX_TOKEN_DAYS": "91",
	} {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, v)
			if _, err := Load(); err == nil {
				t.Fatalf("expected error for %s=%s", k, v)
			}
		})
	}
}
