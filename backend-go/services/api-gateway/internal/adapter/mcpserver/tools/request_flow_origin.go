package tools

import (
	"context"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// unknownMCPClientName keeps source_site non-empty for display; it is not part of any idempotency key.
const unknownMCPClientName = "unknown-mcp-client"

func isRequestFlowNamespace(ns string) bool {
	switch ns {
	case "request", "solution", "approval", "backlog":
		return true
	}
	return false
}

// requestClientName is the verified MCP client name, or a placeholder.
func (e *Executor) requestClientName(ctx context.Context) string {
	if e.guards.ClientName != nil {
		if n := e.guards.ClientName(ctx); n != "" {
			return n
		}
	}
	return unknownMCPClientName
}

// withRequestOrigin marks Request-flow calls as MCP-originated so the gateway
// sets source_provider=mcp itself; the tool input has no such field, so a client
// cannot claim another source. Other namespaces keep their context unchanged.
func (e *Executor) withRequestOrigin(ctx context.Context, p mcpserver.Principal, spec *ToolSpec) context.Context {
	if !isRequestFlowNamespace(spec.Namespace) {
		return ctx
	}
	origin := wscompat.ToolOrigin{ClientName: e.requestClientName(ctx), UserID: p.UserID}
	if e.guards.SessionID != nil {
		origin.MCPSessionID = e.guards.SessionID(ctx)
	}
	return wscompat.WithToolOrigin(ctx, origin)
}
