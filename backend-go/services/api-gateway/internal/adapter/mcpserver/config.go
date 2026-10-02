package mcpserver

import (
	"net/url"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
)

// Config is the slice of gateway configuration the adapter needs.
type Config struct {
	// ResourceURL is the canonical MCP resource (<MCP_PUBLIC_BASE_URL>/mcp).
	ResourceURL string
	// IssuerURL is the OAuth authorization server advertised to clients.
	IssuerURL string
	// AllowedOrigins gates browser-originated requests. A nil/non-enforcing
	// policy rejects EVERY request that carries an Origin (new endpoint, no
	// legacy behavior to preserve); requests without Origin are unaffected.
	AllowedOrigins *originpolicy.Policy
	// MaxBodyBytes bounds a request body (default 1 MiB).
	MaxBodyBytes int64
	// SessionIdleTTL closes idle sessions held in this replica.
	SessionIdleTTL time.Duration
	// PageSize is the page size of list methods (default 50, CONTRACT C7).
	PageSize int
	// MaxStreamsPerUser/PerTenant cap concurrent GET streams per replica.
	MaxStreamsPerUser   int
	MaxStreamsPerTenant int
	// ToolsListChanged enables forwarding tenant "tools_list_changed" signals as
	// notifications/tools/list_changed (set together with the capability, BE-MCP-SOL-007).
	ToolsListChanged bool
	// ScopesSupported feeds the RFC 9728 metadata document.
	ScopesSupported []string
	// ServerVersion is reported as serverInfo.version.
	ServerVersion string
}

func (c Config) withDefaults() Config {
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = 1 << 20
	}
	if c.SessionIdleTTL <= 0 {
		c.SessionIdleTTL = 30 * time.Minute
	}
	if c.PageSize <= 0 {
		c.PageSize = 50
	}
	if c.ServerVersion == "" {
		c.ServerVersion = "dev"
	}
	return c
}

// metadataURL is where RFC 9728 clients find the protected-resource document.
func (c Config) metadataURL() string {
	u, err := url.Parse(c.ResourceURL)
	if err != nil || u.Host == "" {
		return "/.well-known/oauth-protected-resource"
	}
	return u.Scheme + "://" + u.Host + "/.well-known/oauth-protected-resource"
}
