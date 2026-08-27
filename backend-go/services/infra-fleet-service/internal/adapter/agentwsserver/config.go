package agentwsserver

import "os"

// Config configures both halves of this package: server.go's inbound WS
// handshake handler and token_endpoint.go's POST/GET /api/agent-token HTTP
// endpoint.
type Config struct {
	// Port is infra-fleet-service's own HTTP port — reused from
	// svcconfig.Config.HTTPPort by the composition root (main.go), not read
	// from its own env var here. Used only as a best-effort fallback
	// host:port for the agentCommand hint in POST /api/agent-token's
	// response, when the incoming request carries no usable Host header.
	Port int
	// APISecret is ORCA_AGENT_API_SECRET. Empty is a valid, intentional
	// "endpoint disabled" state — see TokenIssuer.isAuthorized's doc
	// comment for why this must fail secure and never fall back to any
	// other check (BUG-AWS-004 in the TS reference was exactly that
	// bypass — an insecure X-Orca-Admin header fallback — and must not be
	// replicated here).
	APISecret string
	// OrcaVersion is echoed back in a successful agent.handshake response's
	// orcaVersion field. Reused from the same ORCA_VERSION env var
	// devserveragent.Config already reads — threaded in via
	// LoadConfigFromEnv's parameter rather than a second os.Getenv call for
	// the same value.
	OrcaVersion string
	// MinAgentVersion gates a direct-websocket agent's handshake — an agent
	// reporting an older AgentVersion is rejected with 1008 (see
	// isBelowMinimumVersion, rejectVersion in version.go). Empty disables
	// the check entirely (fail open — matches an agent build too old to
	// send agentVersion at all, see server.go's firstNonEmpty fallback).
	MinAgentVersion string
	// MaxConcurrentSessions caps live direct-websocket sessions accepted by
	// this process — see capacity.go (TASK-AWS-02-03). <= 0 disables the
	// check.
	MaxConcurrentSessions int
}

// LoadConfigFromEnv reads ORCA_AGENT_API_SECRET and combines it with
// port/orcaVersion supplied by the caller — main.go already computes both
// (svcconfig.Config.HTTPPort and devserveragent's LoadConfigFromEnv().OrcaVersion)
// so this avoids re-reading ORCA_VERSION redundantly.
func LoadConfigFromEnv(port int, orcaVersion string) Config {
	return Config{
		Port:                  port,
		APISecret:             os.Getenv("ORCA_AGENT_API_SECRET"),
		OrcaVersion:           orcaVersion,
		MinAgentVersion:       os.Getenv("ORCA_AGENT_MIN_VERSION"), // e.g. "1.0.0"; empty = no check
		MaxConcurrentSessions: 500,                                 // circuit-breaker default, see capacity.go's doc comment — not a tuned production limit
	}
}
