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
	// DefaultTenantID is the tenant a direct-websocket agent's dev_servers
	// row is registered under (see ResolveDirectWebSocketDevServer's doc
	// comment for why this can't come from request context the normal way).
	// ORCA_AGENT_DEFAULT_TENANT_ID, falling back to the well-known bootstrap
	// tenant sentinel every fresh deployment's admin account is seeded
	// under (00000000-0000-0000-0000-000000000001) — correct for today's
	// single-tenant-per-deployment reality; a true multi-tenant agent-token
	// flow would need this configurable per token, not per deployment.
	DefaultTenantID string
}

// defaultBootstrapTenantID mirrors the sentinel bootstrap tenant every
// fresh deployment's admin account is seeded under (live-verified against
// b15.openledger.vn's auth.users row).
const defaultBootstrapTenantID = "00000000-0000-0000-0000-000000000001"

// LoadConfigFromEnv reads ORCA_AGENT_API_SECRET and combines it with
// port/orcaVersion supplied by the caller — main.go already computes both
// (svcconfig.Config.HTTPPort and devserveragent's LoadConfigFromEnv().OrcaVersion)
// so this avoids re-reading ORCA_VERSION redundantly.
func LoadConfigFromEnv(port int, orcaVersion string) Config {
	tenantID := os.Getenv("ORCA_AGENT_DEFAULT_TENANT_ID")
	if tenantID == "" {
		tenantID = defaultBootstrapTenantID
	}
	return Config{
		Port:            port,
		APISecret:       os.Getenv("ORCA_AGENT_API_SECRET"),
		OrcaVersion:     orcaVersion,
		DefaultTenantID: tenantID,
	}
}
