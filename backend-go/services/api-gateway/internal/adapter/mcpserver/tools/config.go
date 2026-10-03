package tools

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config selects which packs are catalogued and bounds execution.
type Config struct {
	// Packs enabled (1..4). Default {1}: write/exec/admin packs stay hidden
	// until governance (BE-012/013) is live, per BE-008.
	Packs map[int]bool
	// ToolTimeout bounds one tools/call (default 55s: below Dispatch's 60s so
	// the error is ours and mentions the cause).
	ToolTimeout time.Duration
	// SCMRatePerMin caps calls per tenant and provider group (github, gitlab,
	// linear, hostedReview) so agents cannot exhaust the user's API quota.
	// 0 = default 30, negative = disabled.
	SCMRatePerMin int
	// PIIMask: "off", "directory" (tools flagged PII; default) or "all".
	PIIMask string
	// SensitivePathExtra widens the secret-path deny list of the files tools.
	SensitivePathExtra []string
}

const (
	PIIMaskOff       = "off"
	PIIMaskDirectory = "directory"
	PIIMaskAll       = "all"

	defaultSCMRatePerMin = 30
)

// ApplyEnv reads MCP_SCM_RATE_PER_MIN, MCP_PII_MASK and MCP_SENSITIVE_PATH_EXTRA.
func (c Config) ApplyEnv(getenv func(string) string) (Config, error) {
	if v := strings.TrimSpace(getenv("MCP_SCM_RATE_PER_MIN")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return c, fmt.Errorf("MCP_SCM_RATE_PER_MIN: %q is not a non-negative integer", v)
		}
		if n == 0 {
			n = -1 // explicit 0 disables the limiter
		}
		c.SCMRatePerMin = n
	}
	if v := strings.ToLower(strings.TrimSpace(getenv("MCP_PII_MASK"))); v != "" {
		switch v {
		case PIIMaskOff, PIIMaskDirectory, PIIMaskAll:
			c.PIIMask = v
		default:
			return c, fmt.Errorf("MCP_PII_MASK: %q must be off, directory or all", v)
		}
	}
	for _, g := range strings.Split(getenv("MCP_SENSITIVE_PATH_EXTRA"), ",") {
		if g = strings.TrimSpace(g); g != "" {
			c.SensitivePathExtra = append(c.SensitivePathExtra, g)
		}
	}
	return c, nil
}

// DefaultConfig is the shipped default (pack 1 only).
func DefaultConfig() Config {
	return Config{Packs: map[int]bool{1: true}, ToolTimeout: 55 * time.Second}
}

// ParsePacks parses MCP_TOOL_PACKS_ENABLED ("1,2"). Empty = default.
func ParsePacks(s string) (map[int]bool, error) {
	out := map[int]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 4 {
			return nil, fmt.Errorf("MCP_TOOL_PACKS_ENABLED: %q is not a pack in 1..4", p)
		}
		out[n] = true
	}
	if len(out) == 0 {
		return DefaultConfig().Packs, nil
	}
	return out, nil
}

func (c Config) withDefaults() Config {
	d := DefaultConfig()
	if c.Packs == nil {
		c.Packs = d.Packs
	}
	if c.SCMRatePerMin == 0 {
		c.SCMRatePerMin = defaultSCMRatePerMin
	}
	if c.PIIMask == "" {
		c.PIIMask = PIIMaskDirectory
	}
	if c.ToolTimeout <= 0 {
		c.ToolTimeout = d.ToolTimeout
	}
	return c
}
