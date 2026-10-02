package mcpserver

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// protectedResourceMetadata serves the RFC 9728 document built purely from
// configuration (no token logic): which AS issues tokens for this resource.
func protectedResourceMetadata(cfg Config) http.Handler {
	md := &oauthex.ProtectedResourceMetadata{
		Resource:               cfg.ResourceURL,
		ScopesSupported:        cfg.ScopesSupported,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Orca MCP",
	}
	if cfg.IssuerURL != "" {
		md.AuthorizationServers = []string{cfg.IssuerURL}
	}
	return auth.ProtectedResourceMetadataHandler(md)
}
