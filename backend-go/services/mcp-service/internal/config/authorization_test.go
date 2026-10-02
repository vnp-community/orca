package config

import "testing"

func TestLoadAuthorization(t *testing.T) {
	t.Setenv("MCP_PUBLIC_BASE_URL", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	if c, err := LoadAuthorization(); err != nil || c.Enabled {
		t.Fatalf("unset must disable cleanly: %+v %v", c, err)
	}

	t.Setenv("PUBLIC_BASE_URL", "https://orca.example.com")
	t.Setenv("OAUTH_INTERNAL_CALLER_TOKEN", "tok")
	c, err := LoadAuthorization()
	if err != nil || !c.Enabled || c.Issuer != "https://orca.example.com" || c.ConsentTTL.Minutes() != 10 {
		t.Fatalf("fallback to PUBLIC_BASE_URL failed: %+v %v", c, err)
	}

	cases := map[string]map[string]string{
		"http issuer":       {"MCP_PUBLIC_BASE_URL": "http://orca.example.com"},
		"issuer with path":  {"MCP_PUBLIC_BASE_URL": "https://orca.example.com/x"},
		"no internal token": {"OAUTH_INTERNAL_CALLER_TOKEN": ""},
		"ttl too long":      {"MCP_CONSENT_TTL": "5h"},
		"ttl too short":     {"MCP_CONSENT_TTL": "5s"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PUBLIC_BASE_URL", "https://orca.example.com")
			t.Setenv("OAUTH_INTERNAL_CALLER_TOKEN", "tok")
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := LoadAuthorization(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
