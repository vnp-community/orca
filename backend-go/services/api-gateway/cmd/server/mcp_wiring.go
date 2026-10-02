package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/stablyai/orca-go/common/mcpscope"
	"github.com/stablyai/orca-go/common/oauthmetadata"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/originpolicy"
	svcconfig "github.com/stablyai/orca-go/services/api-gateway/internal/config"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// defaultMCPScopes feeds the RFC 9728 metadata until the scope catalog is
// served from mcp-service (CONTRACT McpScopeId).
var defaultMCPScopes = mcpscope.All()

// buildMCPHandler builds the handler with the fail-closed default verifier
// (every token rejected); production wiring uses buildMCPHandlerWithVerifier.
func buildMCPHandler(cfg svcconfig.MCPConfig, logger *slog.Logger, limiter *usecase.RateLimiter) (*mcpserver.Handler, error) {
	return buildMCPHandlerWithVerifier(cfg, logger, limiter, nil)
}

// buildMCPHandlerWithVerifier returns nil when MCP_ENABLED=false so /mcp 404s.
// A nil verifier means deny-all.
// opts adjust Deps before the handler is built (tool catalog/executor wiring).
func buildMCPHandlerWithVerifier(cfg svcconfig.MCPConfig, logger *slog.Logger, limiter *usecase.RateLimiter, verifier mcpserver.TokenVerifier, opts ...func(*mcpserver.Deps)) (*mcpserver.Handler, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	asMetadata, err := authServerMetadataHandler(cfg)
	if err != nil {
		return nil, err
	}
	origins, err := originpolicy.Parse(cfg.AllowedOrigins)
	if err != nil {
		return nil, fmt.Errorf("MCP_ALLOWED_ORIGINS: %w", err)
	}
	if !origins.Enforced() {
		logger.Warn("MCP_ALLOWED_ORIGINS and WS_ALLOWED_ORIGINS are empty: /mcp rejects every request that carries an Origin header")
	}
	keys, err := loadMCPCursorKeys(cfg)
	if err != nil {
		return nil, err
	}
	deps := mcpserver.Deps{
		Logger: logger,
		Config: mcpserver.Config{
			ResourceURL: cfg.ResourceURL(), IssuerURL: cfg.IssuerURL, AllowedOrigins: origins,
			MaxBodyBytes: cfg.MaxRequestBytes, SessionIdleTTL: cfg.SessionIdleTTL,
			MaxStreamsPerUser: cfg.MaxSSEStreamsPerUser, MaxStreamsPerTenant: cfg.MaxSSEStreamsPerTenant,
			ScopesSupported: defaultMCPScopes, ServerVersion: version,
		},
		CursorKeys:         keys,
		RateLimiter:        limiter,
		Verifier:           verifier,
		AuthServerMetadata: asMetadata,
	}
	for _, opt := range opts {
		opt(&deps)
	}
	return mcpserver.NewHandler(deps), nil
}

// authServerMetadataHandler serves RFC 8414 metadata built from configuration
// only (issuer is never derived from the request Host).
func authServerMetadataHandler(cfg svcconfig.MCPConfig) (http.Handler, error) {
	doc, err := oauthmetadata.BuildAuthorizationServer(cfg.IssuerURL, cfg.DCREnabled, defaultMCPScopes)
	if err != nil {
		return nil, fmt.Errorf("MCP_ISSUER_URL: %w", err)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(body)
	}), nil
}

// loadMCPCursorKeys returns [current, previous?]. A key file ("current" then
// optional "previous", one per line; rendered by Vault Agent like other
// secrets) wins over the inline env vars. No key at all is allowed (the
// handler then uses an ephemeral key and warns); an unreadable file is not.
func loadMCPCursorKeys(cfg svcconfig.MCPConfig) ([][]byte, error) {
	var keys [][]byte
	if cfg.CursorKeyFile != "" {
		raw, err := os.ReadFile(cfg.CursorKeyFile)
		if err != nil {
			return nil, fmt.Errorf("MCP_CURSOR_KEY_FILE: %w", err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				keys = append(keys, []byte(line))
			}
		}
		if len(keys) == 0 {
			return nil, fmt.Errorf("MCP_CURSOR_KEY_FILE %q holds no key", cfg.CursorKeyFile)
		}
		return keys, nil
	}
	for _, k := range []string{cfg.CursorKey, cfg.CursorKeyPrevious} {
		if k != "" {
			keys = append(keys, []byte(k))
		}
	}
	return keys, nil
}
