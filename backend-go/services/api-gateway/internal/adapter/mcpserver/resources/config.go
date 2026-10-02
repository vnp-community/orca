// Package resources serves MCP resources (BE-MCP-SOL-010): orca:// URIs mapped
// to read-only wscompat channels, authorized by the same PolicyGate as a
// risk=read tool call. It is a protocol adapter only; no policy lives here.
package resources

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config bounds resource reads and subscriptions.
type Config struct {
	// MaxBytes caps one resource's text (default 256 KiB): MCP_RESOURCE_MAX_BYTES.
	MaxBytes int
	// MaxSubsPerSession caps subscriptions per MCP session (default 50): MCP_RESOURCE_MAX_SUBS.
	MaxSubsPerSession int
	// FileEnabled exposes orca://worktree/{id}/file/{+path}. OFF by default
	// until git-gateway rejects symlinks that escape the worktree on every
	// path (localfs resolve() is lexical only): MCP_RESOURCE_FILE_ENABLED.
	FileEnabled bool
	// SensitivePathExtra widens the secret-path deny list (never narrows): MCP_SENSITIVE_PATH_EXTRA.
	SensitivePathExtra []string
	// Debounce coalesces bursts of events into one notification (default 250ms).
	Debounce time.Duration
	// CallTimeout bounds each downstream channel call (default 20s).
	CallTimeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.MaxBytes <= 0 {
		c.MaxBytes = 256 << 10
	}
	if c.MaxSubsPerSession <= 0 {
		c.MaxSubsPerSession = 50
	}
	if c.Debounce <= 0 {
		c.Debounce = 250 * time.Millisecond
	}
	if c.CallTimeout <= 0 {
		c.CallTimeout = 20 * time.Second
	}
	return c
}

// ConfigFromEnv reads the MCP_RESOURCE_* variables; invalid values fall back
// to defaults except the file flag, which only a literal "true" enables.
func ConfigFromEnv() Config {
	var c Config
	if n, err := strconv.Atoi(os.Getenv("MCP_RESOURCE_MAX_BYTES")); err == nil && n > 0 {
		c.MaxBytes = n
	}
	if n, err := strconv.Atoi(os.Getenv("MCP_RESOURCE_MAX_SUBS")); err == nil && n > 0 {
		c.MaxSubsPerSession = n
	}
	c.FileEnabled = strings.EqualFold(strings.TrimSpace(os.Getenv("MCP_RESOURCE_FILE_ENABLED")), "true")
	for _, g := range strings.Split(os.Getenv("MCP_SENSITIVE_PATH_EXTRA"), ",") {
		if g = strings.TrimSpace(g); g != "" {
			c.SensitivePathExtra = append(c.SensitivePathExtra, g)
		}
	}
	return c
}
