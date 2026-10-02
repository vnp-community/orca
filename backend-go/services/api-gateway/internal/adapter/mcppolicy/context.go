package mcppolicy

import (
	"context"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// The verified token claims (mcp_depth, mcp_root) and the MCP session id are
// not part of mcpserver.Principal, so the transport layer passes them to the
// gate through the request context. Absent values mean depth 0 / no session,
// which is the safe direction only because mcp-service also applies its own
// limits; the claims are never read from request parameters.

type ctxKey int

const (
	depthKey ctxKey = iota + 1
	sessionKey
	clientNameKey
)

type depthValue struct {
	depth int
	root  string
}

// WithDepth records the recursion depth and root session id taken from the
// VERIFIED token's mcp_depth / mcp_root claims.
func WithDepth(ctx context.Context, depth int, root string) context.Context {
	return context.WithValue(ctx, depthKey, depthValue{depth: depth, root: root})
}

func DepthFromContext(ctx context.Context) (int, string) {
	v, _ := ctx.Value(depthKey).(depthValue)
	if v.depth < 0 {
		return 0, v.root
	}
	return v.depth, v.root
}

// WithSessionID records the MCP session id of the current request.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionKey, id)
}

func SessionIDFromContext(ctx context.Context) string {
	s, _ := ctx.Value(sessionKey).(string)
	return s
}

// WithClientName records the display name the client registered with
// (sanitized again by mcp-service before it is shown or stored).
func WithClientName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, clientNameKey, name)
}

func ClientNameFromContext(ctx context.Context) string {
	s, _ := ctx.Value(clientNameKey).(string)
	return s
}

// RequestContext is the mcpserver.Deps.RequestContext hook: it records the
// verified session id, client name and token depth/root for the gate. Depth and
// root come from the verified token claims carried by the Principal, never
// from request data; the session id is the non-secret row id.
func RequestContext(ctx context.Context, _ mcpserver.Principal, info mcpserver.RequestInfo) context.Context {
	ctx = WithDepth(ctx, info.Depth, info.Root)
	if info.SessionID != "" {
		ctx = WithSessionID(ctx, info.SessionID)
	}
	return WithClientName(ctx, info.ClientName)
}
