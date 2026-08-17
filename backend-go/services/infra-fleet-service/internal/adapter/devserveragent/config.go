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
// domain.DevServer (this scaffold's proto-sized subset — see its own doc
// comment) carries only Host, not a port or per-device token: unlike
// direct-websocket's per-connection AgentTokenManager-issued token,
// relay-websocket's ORCA_AGENT_TOKEN is documented
// (specs/agent/api/connection-modes.md §"relay-websocket token (contrast)")
// as "a static, operator-set, long-lived shared secret ... never expires or
// rotates" — i.e. deployment-wide, not per-dev-server. Modeling it as
// service-level Config rather than adding columns/proto fields for a
// single shared secret matches that reality; a future per-dev-server
// override would need domain.DevServer extended plus a migration, tracked
// as a follow-up (see this package's README "Known gaps"), not invented
// here.
type Config struct {
	// Port is the agent's AGENT_PORT (agent-connection-relay.ts default 6799).
	Port int
	// Token is sent as `Authorization: Bearer <token>` per
	// agent-connection-relay.ts's authenticate(). Empty means relay-websocket
	// dev servers cannot be reached — Exec/Health report a clear error
	// rather than silently dialing unauthenticated.
	Token string
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

// LoadConfigFromEnv reads AGENT_PORT / ORCA_AGENT_TOKEN / ORCA_VERSION on
// top of DefaultConfig — matching agent-connection-relay.ts's own env var
// names on the agent side, so operators set one consistent name across
// both processes.
func LoadConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("AGENT_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.Port = port
		}
	}
	cfg.Token = os.Getenv("ORCA_AGENT_TOKEN")
	cfg.OrcaVersion = os.Getenv("ORCA_VERSION")
	if cfg.OrcaVersion == "" {
		cfg.OrcaVersion = "backend-go-dev"
	}
	return cfg
}
