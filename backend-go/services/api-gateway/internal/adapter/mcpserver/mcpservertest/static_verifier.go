// Package mcpservertest holds test doubles for mcpserver ports. It must never
// be imported by production wiring (cmd/server): StaticVerifier trusts a
// hard-coded token table.
package mcpservertest

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// StaticVerifier accepts exactly the tokens in Tokens.
type StaticVerifier struct {
	Tokens map[string]mcpserver.Principal
}

func (v StaticVerifier) Verify(_ context.Context, r *http.Request) (mcpserver.Principal, error) {
	tok := mcpserver.BearerToken(r)
	if tok == "" {
		return mcpserver.Principal{}, mcpserver.ErrNoCredentials
	}
	p, ok := v.Tokens[tok]
	if !ok {
		return mcpserver.Principal{}, mcpserver.ErrInvalidToken
	}
	if p.ExpiresAt.IsZero() {
		p.ExpiresAt = time.Now().Add(time.Hour)
	}
	return p, nil
}

// FakeCatalog is a ToolCatalog test double: Names for everyone, or per-tenant
// names when ByTenant has an entry for the caller's tenant.
type FakeCatalog struct {
	Names    []string
	ByTenant map[string][]string
}

func (c FakeCatalog) ListTools(_ context.Context, p mcpserver.Principal) ([]*mcp.Tool, error) {
	names := c.Names
	if n, ok := c.ByTenant[p.TenantID]; ok {
		names = n
	}
	out := make([]*mcp.Tool, 0, len(names))
	for _, n := range names {
		out = append(out, &mcp.Tool{Name: n, Description: n, InputSchema: map[string]any{"type": "object"}})
	}
	return out, nil
}
