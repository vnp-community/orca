package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
	"github.com/stablyai/orca-go/common/mcpscope"
)

// ExternalConfig configures the external MCP server registry and agent config
// issuing (BE-MCP-SOL-014). Env: MCP_EXTERNAL_HTTP_ALLOWLIST,
// MCP_EXTERNAL_ALLOWED_PORTS, MCP_EXTERNAL_STDIO_ENABLED,
// MCP_EXTERNAL_STDIO_ALLOW_UNSANDBOXED, MCP_STDIO_FOUR_EYES,
// MCP_EXTERNAL_HEALTH_INTERVAL, MCP_AGENT_CONFIG_ENABLED, MCP_MAX_AGENT_DEPTH,
// MCP_AGENT_TOKEN_TTL, MCP_AGENT_TOKEN_SCOPES, CREDENTIAL_BROKER_ADDR,
// TENANT_SERVICE_ADDR (the Orca MCP URL derives from MCP_PUBLIC_BASE_URL).
type ExternalConfig struct {
	HTTPAllowlist         []string // host:port pairs allowed over plain http (dev only)
	AllowedPorts          []int
	StdioEnabled          bool
	StdioAllowUnsandboxed bool
	StdioFourEyes         bool
	HealthInterval        time.Duration // 0 disables the health worker
	AgentConfigEnabled    bool
	MaxAgentDepth         int
	AgentTokenTTL         time.Duration
	AgentTokenScopes      []string
	BrokerAddr            string // empty: secrets are unavailable (MCP_UNAVAILABLE)
	TenantServiceAddr     string // empty: ResolveAgentMcpConfig cannot read profiles
	OrcaMcpURL            string
}

func csv(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func LoadExternal() (ExternalConfig, error) {
	var err error
	c := ExternalConfig{
		HTTPAllowlist:     csv(commonconfig.StringEnv("MCP_EXTERNAL_HTTP_ALLOWLIST", "")),
		BrokerAddr:        commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", ""),
		TenantServiceAddr: commonconfig.StringEnv("TENANT_SERVICE_ADDR", ""),
	}
	for _, p := range csv(commonconfig.StringEnv("MCP_EXTERNAL_ALLOWED_PORTS", "443")) {
		n, perr := strconv.Atoi(p)
		if perr != nil || n < 1 || n > 65535 {
			return c, fmt.Errorf("config: invalid port %q in MCP_EXTERNAL_ALLOWED_PORTS", p)
		}
		c.AllowedPorts = append(c.AllowedPorts, n)
	}
	for _, f := range []struct {
		env string
		def bool
		dst *bool
	}{
		{"MCP_EXTERNAL_STDIO_ENABLED", false, &c.StdioEnabled},
		{"MCP_EXTERNAL_STDIO_ALLOW_UNSANDBOXED", false, &c.StdioAllowUnsandboxed},
		{"MCP_STDIO_FOUR_EYES", false, &c.StdioFourEyes},
		{"MCP_AGENT_CONFIG_ENABLED", false, &c.AgentConfigEnabled},
	} {
		if *f.dst, err = boolEnv(f.env, f.def); err != nil {
			return c, err
		}
	}
	if c.HealthInterval, err = durationEnv("MCP_EXTERNAL_HEALTH_INTERVAL", 5*time.Minute); err != nil {
		return c, err
	}
	if c.MaxAgentDepth, err = intEnv("MCP_MAX_AGENT_DEPTH", 2); err != nil {
		return c, err
	}
	if c.AgentTokenTTL, err = durationEnv("MCP_AGENT_TOKEN_TTL", 24*time.Hour); err != nil {
		return c, err
	}
	scopes, serr := mcpscope.Parse(strings.ReplaceAll(commonconfig.StringEnv("MCP_AGENT_TOKEN_SCOPES", mcpscope.Read), ",", " "))
	if serr != nil {
		return c, fmt.Errorf("config: MCP_AGENT_TOKEN_SCOPES: %w", serr)
	}
	c.AgentTokenScopes = scopes
	if base := commonconfig.StringEnv("MCP_PUBLIC_BASE_URL", commonconfig.StringEnv("PUBLIC_BASE_URL", "")); base != "" {
		c.OrcaMcpURL = strings.TrimRight(base, "/") + "/mcp"
	}
	return c, c.Validate()
}

func (c ExternalConfig) Validate() error {
	switch {
	case c.MaxAgentDepth < 0:
		return errors.New("config: MCP_MAX_AGENT_DEPTH must be >= 0")
	case c.AgentTokenTTL <= 0 || c.AgentTokenTTL > 90*24*time.Hour:
		return errors.New("config: MCP_AGENT_TOKEN_TTL must be between 1ns and 90 days")
	case c.HealthInterval < 0:
		return errors.New("config: MCP_EXTERNAL_HEALTH_INTERVAL must not be negative")
	case c.StdioAllowUnsandboxed && !c.StdioEnabled:
		return errors.New("config: MCP_EXTERNAL_STDIO_ALLOW_UNSANDBOXED requires MCP_EXTERNAL_STDIO_ENABLED")
	}
	return nil
}
