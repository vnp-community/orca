package devserveragent

import (
	"os"
	"strconv"
	"time"
)

// Config configures the relay-websocket transport. Fields mirror the TS
// side's constants directly (see doc comments) rather than inventing new
// tuning knobs.
//
// Per-DevServer bearer tokens are NOT modeled here — see TASK-AWS-01-03 and
// SOL-AWS-01: a single deployment-wide shared-secret token field here used
// to mean every relay-websocket DevServer authenticated with the same
// value, so revoking or rotating one DevServer's token was impossible
// without a full redeploy. Client.AgentTokenSource (client.go) now
// resolves each dial's token individually via usecase.AgentTokenRepository
// + usecase.CredentialBrokerClient.
type Config struct {
	// Port is the agent's AGENT_PORT (agent-connection-relay.ts default 6799).
	Port int
	// OrcaVersion is sent in the agent.handshake params — cosmetic
	// (agent-session.ts doesn't gate on it), but included for parity with
	// dev-server-relay-bridge.ts's getPlatform().app.getVersion().
	OrcaVersion string

	// DialTimeout bounds the initial TCP+WS-upgrade connect — matches
	// dev-server-relay-bridge.ts's connectRelayWebSocket 10s connectionTimeout.
	DialTimeout time.Duration
	// HandshakeTimeout matches AGENT_TIMEOUT_MS (20s) — how long Orca waits
	// for the agent's agent.handshake response.
	HandshakeTimeout time.Duration
	// RequestTimeout matches SshChannelMultiplexer's REQUEST_TIMEOUT_MS
	// (30s) — the default per-call timeout once past the handshake.
	RequestTimeout time.Duration
	// KeepAliveInterval matches KEEPALIVE_SEND_MS (5s).
	KeepAliveInterval time.Duration
	// IdleTimeout matches TIMEOUT_MS (20s) — no frame received within this
	// window is treated as a dead connection, triggering reconnect.
	IdleTimeout time.Duration

	// ReconnectBaseDelay/ReconnectMaxDelay match calcBackoffDelay's
	// documented "2s * 2^attempt, capped 60s, +0-1s jitter"
	// (specs/agent/api/connection-modes.md §7).
	ReconnectBaseDelay time.Duration
	ReconnectMaxDelay  time.Duration
}

// DefaultConfig returns Config populated with the TS-side constants;
// callers override Port/Token/OrcaVersion.
func DefaultConfig() Config {
	return Config{
		Port:               6799,
		DialTimeout:        10 * time.Second,
		HandshakeTimeout:   20 * time.Second,
		RequestTimeout:     30 * time.Second,
		KeepAliveInterval:  5 * time.Second,
		IdleTimeout:        20 * time.Second,
		ReconnectBaseDelay: 2 * time.Second,
		ReconnectMaxDelay:  60 * time.Second,
	}
}

// LoadConfigFromEnv reads AGENT_PORT / ORCA_VERSION on top of DefaultConfig
// — matching agent-connection-relay.ts's own env var names on the agent
// side, so operators set one consistent name across both processes. The
// per-DevServer bearer token is resolved separately per dial — see the
// package doc comment above and AgentTokenSource (client.go).
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("AGENT_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.Port = port
		}
	}
	cfg.OrcaVersion = os.Getenv("ORCA_VERSION")
	if cfg.OrcaVersion == "" {
		cfg.OrcaVersion = "backend-go-dev"
	}
	return cfg
}
