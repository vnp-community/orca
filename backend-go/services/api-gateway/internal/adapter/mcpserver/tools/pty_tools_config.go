package tools

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// WorktreeTargets resolves where a worktree's terminal runs (infra-fleet
// connection, local/SSH/dev server alike) and its checkout path, so MCP
// terminals take the same route as UI terminals. wscompat.WorktreeTargetResolver
// implements it.
type WorktreeTargets interface {
	ResolveWorktreeTarget(ctx context.Context, id wscompat.Identity, worktreeID string) (connectionID, cwd string, err error)
}

// PtyToolsConfig bounds the long-running terminal/agent tools (BE-MCP-SOL-009 §D).
type PtyToolsConfig struct {
	RingBytes              int           // per-PTY output ring (MCP_TERMINAL_RING_BYTES, 1 MiB)
	ReadMaxBytes           int           // default bytes per terminal_read (MCP_TERMINAL_READ_MAX_BYTES, 16 KiB)
	ReadHardMaxBytes       int           // absolute cap per read (64 KiB)
	MaxTerminalsPerSession int           // MCP_TERMINAL_MAX_PER_SESSION (4)
	MaxAgentsPerSession    int           // MCP_AGENT_MAX_PER_SESSION (2)
	MaxTerminalsPerUser    int           // MCP_TERMINAL_MAX_PER_USER (8)
	MaxTerminalsPerTenant  int           // MCP_TERMINAL_MAX_PER_TENANT (32)
	IdleTimeout            time.Duration // MCP_TERMINAL_IDLE_TIMEOUT (30m)
	JanitorEvery           time.Duration // idle sweep period (30s)
	ReadRate               float64       // MCP_TERMINAL_READ_RATE reads/s per session (5)
	ReadBurst              int           // burst (10)
	CloseTimeout           time.Duration // budget to stop everything of a closed session (10s)
	AgentGrace             time.Duration // polite agent.stop wait before agent.kill (3s)
	QuotaCacheTTL          time.Duration // durable terminal.list count cache (5s)
	Targets                WorktreeTargets
	// OnIdleStopped is told when the janitor stopped a PTY for inactivity.
	OnIdleStopped func(tenantID, userID, mcpSessionID, ptyID string)
}

// DefaultPtyToolsConfig is the shipped default.
func DefaultPtyToolsConfig() PtyToolsConfig {
	return PtyToolsConfig{
		RingBytes: 1 << 20, ReadMaxBytes: 16 << 10, ReadHardMaxBytes: 64 << 10,
		MaxTerminalsPerSession: 4, MaxAgentsPerSession: 2, MaxTerminalsPerUser: 8, MaxTerminalsPerTenant: 32,
		IdleTimeout: 30 * time.Minute, JanitorEvery: 30 * time.Second, ReadRate: 5, ReadBurst: 10,
		CloseTimeout: 10 * time.Second, AgentGrace: 3 * time.Second, QuotaCacheTTL: 5 * time.Second,
	}
}

func (c PtyToolsConfig) withDefaults() PtyToolsConfig {
	d := DefaultPtyToolsConfig()
	pickI := func(v *int, def int) {
		if *v <= 0 {
			*v = def
		}
	}
	pickD := func(v *time.Duration, def time.Duration) {
		if *v <= 0 {
			*v = def
		}
	}
	pickI(&c.RingBytes, d.RingBytes)
	pickI(&c.ReadMaxBytes, d.ReadMaxBytes)
	pickI(&c.ReadHardMaxBytes, d.ReadHardMaxBytes)
	pickI(&c.MaxTerminalsPerSession, d.MaxTerminalsPerSession)
	pickI(&c.MaxAgentsPerSession, d.MaxAgentsPerSession)
	pickI(&c.MaxTerminalsPerUser, d.MaxTerminalsPerUser)
	pickI(&c.MaxTerminalsPerTenant, d.MaxTerminalsPerTenant)
	pickI(&c.ReadBurst, d.ReadBurst)
	pickD(&c.IdleTimeout, d.IdleTimeout)
	pickD(&c.JanitorEvery, d.JanitorEvery)
	pickD(&c.CloseTimeout, d.CloseTimeout)
	pickD(&c.AgentGrace, d.AgentGrace)
	pickD(&c.QuotaCacheTTL, d.QuotaCacheTTL)
	if c.ReadRate <= 0 {
		c.ReadRate = d.ReadRate
	}
	if c.ReadMaxBytes > c.ReadHardMaxBytes {
		c.ReadMaxBytes = c.ReadHardMaxBytes
	}
	return c
}

// PtyToolsConfigFromEnv reads the MCP_TERMINAL_* / MCP_AGENT_* variables over
// the defaults; getenv is os.Getenv in production.
func PtyToolsConfigFromEnv(getenv func(string) string) (PtyToolsConfig, error) {
	c := DefaultPtyToolsConfig()
	for _, e := range []struct {
		key string
		dst *int
	}{
		{"MCP_TERMINAL_RING_BYTES", &c.RingBytes}, {"MCP_TERMINAL_READ_MAX_BYTES", &c.ReadMaxBytes},
		{"MCP_TERMINAL_MAX_PER_SESSION", &c.MaxTerminalsPerSession}, {"MCP_AGENT_MAX_PER_SESSION", &c.MaxAgentsPerSession},
		{"MCP_TERMINAL_MAX_PER_USER", &c.MaxTerminalsPerUser}, {"MCP_TERMINAL_MAX_PER_TENANT", &c.MaxTerminalsPerTenant},
	} {
		if v := getenv(e.key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return c, fmt.Errorf("%s: %q is not a positive integer", e.key, v)
			}
			*e.dst = n
		}
	}
	if v := getenv("MCP_TERMINAL_IDLE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return c, fmt.Errorf("MCP_TERMINAL_IDLE_TIMEOUT: %q is not a positive duration", v)
		}
		c.IdleTimeout = d
	}
	if v := getenv("MCP_TERMINAL_READ_RATE"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 {
			return c, fmt.Errorf("MCP_TERMINAL_READ_RATE: %q is not a positive number", v)
		}
		c.ReadRate = f
	}
	return c.withDefaults(), nil
}

// ToolError is a tool-level failure whose code and message are safe to show the
// client verbatim (unlike channel errors, which only keep a code).
type ToolError struct{ Code, Msg string }

func (e *ToolError) Error() string { return e.Code + ": " + e.Msg }
