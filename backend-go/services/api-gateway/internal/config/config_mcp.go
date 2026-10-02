package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	commonconfig "github.com/stablyai/orca-go/common/config"
)

// MCPConfig is the api-gateway slice of the MCP feature (CR-MCP-002/003,
// BE-MCP-SOL-002 section A). Kept in its own file so the MCP knobs do not
// interleave with the unrelated Config fields.
type MCPConfig struct {
	// Enabled is the process-level master switch (MCP_ENABLED, default false):
	// off means /mcp is not mounted, mcp-service is not dialed and mcp.*
	// channels answer MCP_DISABLED (CONTRACT C8).
	Enabled bool
	// PublicBaseURL builds resourceUrl (<base>/mcp); defaults to PUBLIC_BASE_URL.
	PublicBaseURL string
	// IssuerURL is the OAuth authorization server advertised in RFC 9728
	// metadata; defaults to PublicBaseURL (auth-service is the AS, T3).
	IssuerURL string
	// AllowedOrigins is the CSV Origin allow-list for /mcp; defaults to
	// WS_ALLOWED_ORIGINS (D3).
	AllowedOrigins string
	// MaxRequestBytes bounds a /mcp request body.
	MaxRequestBytes int64
	// SessionIdleTTL closes idle MCP sessions.
	SessionIdleTTL time.Duration
	// ReadHeaderTimeout is applied to the public http.Server only (never a
	// Write/ReadTimeout: those would cut SSE and WebSocket streams).
	ReadHeaderTimeout time.Duration
	// MaxSSEStreamsPerUser / PerTenant cap concurrent GET streams per replica.
	MaxSSEStreamsPerUser   int
	MaxSSEStreamsPerTenant int
	// CursorKey / CursorKeyPrevious sign pagination cursors (previous is only
	// accepted for verification so key rotation does not break live cursors).
	// CursorKeyFile, when set, wins and holds "current[\nprevious]".
	CursorKey         string
	CursorKeyPrevious string
	CursorKeyFile     string
	// ServiceAddr is mcp-service's gRPC address (MCP_SERVICE_ADDR); empty means
	// the gateway degrades gracefully instead of dialing.
	ServiceAddr string
	// DCREnabled mirrors auth-service's OAUTH_DCR_ENABLED (default true): it
	// only decides whether the metadata advertises, and the gateway mounts,
	// POST /oauth/register. The tenant-level dcrEnabled is mcp-service's.
	DCREnabled bool
	// SSEBufferMaxBytes bounds the JetStream resume buffer (MCP_SSE_BUFFER_MAX_BYTES, default 256 MiB).
	SSEBufferMaxBytes int64
	// PATMaxPerUser caps active personal access tokens per user (MCP_PAT_MAX_PER_USER, default 20).
	PATMaxPerUser int
}

const (
	defaultMCPMaxRequestBytes = 1 << 20
	defaultMCPSessionIdleTTL  = 30 * time.Minute
	defaultMCPReadHeader      = 10 * time.Second
)

func loadMCP(publicBaseURL, wsAllowedOrigins string) (MCPConfig, error) {
	enabled, err := boolEnv("MCP_ENABLED", false)
	if err != nil {
		return MCPConfig{}, err
	}
	maxBytes, err := intEnv("MCP_MAX_REQUEST_BYTES", defaultMCPMaxRequestBytes)
	if err != nil {
		return MCPConfig{}, err
	}
	idleTTL, err := durationEnv("MCP_SESSION_IDLE_TTL", defaultMCPSessionIdleTTL)
	if err != nil {
		return MCPConfig{}, err
	}
	readHeader, err := durationEnv("MCP_READ_HEADER_TIMEOUT", defaultMCPReadHeader)
	if err != nil {
		return MCPConfig{}, err
	}
	perUser, err := intEnv("MCP_MAX_SSE_STREAMS_PER_USER", 5)
	if err != nil {
		return MCPConfig{}, err
	}
	perTenant, err := intEnv("MCP_MAX_SSE_STREAMS_PER_TENANT", 200)
	if err != nil {
		return MCPConfig{}, err
	}
	dcr, err := boolEnv("OAUTH_DCR_ENABLED", true)
	if err != nil {
		return MCPConfig{}, err
	}
	patMax, err := intEnv("MCP_PAT_MAX_PER_USER", 20)
	if err != nil {
		return MCPConfig{}, err
	}
	bufMax, err := intEnv("MCP_SSE_BUFFER_MAX_BYTES", 256<<20)
	if err != nil {
		return MCPConfig{}, err
	}
	base := strings.TrimRight(commonconfig.StringEnv("MCP_PUBLIC_BASE_URL", publicBaseURL), "/")
	cfg := MCPConfig{
		Enabled:                enabled,
		PublicBaseURL:          base,
		IssuerURL:              strings.TrimRight(commonconfig.StringEnv("MCP_ISSUER_URL", base), "/"),
		AllowedOrigins:         commonconfig.StringEnv("MCP_ALLOWED_ORIGINS", wsAllowedOrigins),
		MaxRequestBytes:        int64(maxBytes),
		SessionIdleTTL:         idleTTL,
		ReadHeaderTimeout:      readHeader,
		MaxSSEStreamsPerUser:   perUser,
		MaxSSEStreamsPerTenant: perTenant,
		CursorKey:              commonconfig.StringEnv("MCP_CURSOR_KEY", ""),
		CursorKeyPrevious:      commonconfig.StringEnv("MCP_CURSOR_KEY_PREVIOUS", ""),
		CursorKeyFile:          commonconfig.StringEnv("MCP_CURSOR_KEY_FILE", ""),
		ServiceAddr:            commonconfig.StringEnv("MCP_SERVICE_ADDR", ""),
		DCREnabled:             dcr,
		PATMaxPerUser:          patMax,
		SSEBufferMaxBytes:      int64(bufMax),
	}
	return cfg, cfg.Validate()
}

// ResourceURL is the canonical MCP resource identifier (<base>/mcp).
func (c MCPConfig) ResourceURL() string {
	if c.PublicBaseURL == "" {
		return ""
	}
	return c.PublicBaseURL + "/mcp"
}

// Validate rejects configurations that cannot work. Only enforced when MCP is
// enabled so a disabled deployment never fails to start over MCP settings;
// malformed values are still rejected when set.
func (c MCPConfig) Validate() error {
	if c.MaxRequestBytes <= 0 {
		return fmt.Errorf("config: MCP_MAX_REQUEST_BYTES must be positive, got %d", c.MaxRequestBytes)
	}
	if c.SessionIdleTTL <= 0 || c.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("config: MCP_SESSION_IDLE_TTL and MCP_READ_HEADER_TIMEOUT must be positive")
	}
	for _, o := range strings.Split(c.AllowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Path != "" && u.Path != "/") {
			return fmt.Errorf("config: MCP_ALLOWED_ORIGINS entry %q must be scheme://host[:port]", o)
		}
	}
	// Slice, not map: a fixed order keeps the reported variable deterministic.
	for _, nv := range [][2]string{{"MCP_PUBLIC_BASE_URL", c.PublicBaseURL}, {"MCP_ISSUER_URL", c.IssuerURL}} {
		name, v := nv[0], nv[1]
		if v == "" {
			continue
		}
		if u, err := url.Parse(v); err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("config: %s %q is not an absolute URL", name, v)
		}
	}
	if !c.Enabled {
		return nil
	}
	if c.PublicBaseURL == "" {
		return fmt.Errorf("config: MCP_ENABLED requires MCP_PUBLIC_BASE_URL (or PUBLIC_BASE_URL)")
	}
	// An empty ServiceAddr is deliberately NOT an error: the gateway degrades
	// (mcp.server.info -> MCP_UNAVAILABLE) like every other optional downstream.
	return nil
}

func boolEnv(key string, def bool) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("config: invalid bool for %s=%q: %w", key, v, err)
	}
	return b, nil
}

func durationEnv(key string, def time.Duration) (time.Duration, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid duration for %s=%q: %w", key, v, err)
	}
	return d, nil
}
