package config

import (
	"testing"
	"time"
)

func TestLoadOAuth(t *testing.T) {
	t.Setenv("OAUTH_RESOURCE_URL", "")
	if c, err := LoadOAuth(); err != nil || c.Enabled {
		t.Fatalf("unset must disable cleanly: %+v %v", c, err)
	}

	t.Setenv("OAUTH_RESOURCE_URL", "https://orca.example.com/mcp")
	t.Setenv("OAUTH_INTERNAL_CALLER_TOKEN", "tok")
	c, err := LoadOAuth()
	if err != nil || !c.Enabled || c.AccessTokenTTL != 10*time.Minute || !c.DCREnabled || c.DCRMaxClients != 1000 {
		t.Fatalf("defaults wrong: %+v %v", c, err)
	}

	bad := map[string]map[string]string{
		"access ttl over 15m": {"OAUTH_ACCESS_TOKEN_TTL": "16m"},
		"http resource":       {"OAUTH_RESOURCE_URL": "http://orca.example.com/mcp"},
		"no caller token":     {"OAUTH_INTERNAL_CALLER_TOKEN": ""},
		"fragment":            {"OAUTH_RESOURCE_URL": "https://orca.example.com/mcp#x"},
	}
	for name, env := range bad {
		t.Run(name, func(t *testing.T) {
			t.Setenv("OAUTH_RESOURCE_URL", "https://orca.example.com/mcp")
			t.Setenv("OAUTH_INTERNAL_CALLER_TOKEN", "tok")
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := LoadOAuth(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	t.Setenv("OAUTH_RESOURCE_URL", "http://localhost:8080/mcp")
	t.Setenv("OAUTH_INTERNAL_CALLER_TOKEN", "tok")
	if _, err := LoadOAuth(); err != nil {
		t.Fatalf("loopback http must be allowed: %v", err)
	}
}
