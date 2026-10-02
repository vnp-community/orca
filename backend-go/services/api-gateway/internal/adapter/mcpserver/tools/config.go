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
	if c.ToolTimeout <= 0 {
		c.ToolTimeout = d.ToolTimeout
	}
	return c
}
