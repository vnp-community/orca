package config

import "testing"

func TestLoadExternal_Defaults(t *testing.T) {
	c, err := LoadExternal()
	if err != nil {
		t.Fatal(err)
	}
	if c.StdioEnabled || c.AgentConfigEnabled || len(c.AllowedPorts) != 1 || c.AllowedPorts[0] != 443 || c.MaxAgentDepth != 2 {
		t.Fatalf("unsafe defaults: %+v", c)
	}
}

func TestLoadExternal_Rejects(t *testing.T) {
	t.Setenv("MCP_EXTERNAL_STDIO_ALLOW_UNSANDBOXED", "true")
	if _, err := LoadExternal(); err == nil {
		t.Fatal("unsandboxed without stdio enabled must fail")
	}
	t.Setenv("MCP_EXTERNAL_STDIO_ALLOW_UNSANDBOXED", "")
	t.Setenv("MCP_EXTERNAL_ALLOWED_PORTS", "443,abc")
	if _, err := LoadExternal(); err == nil {
		t.Fatal("bad port must fail")
	}
}
