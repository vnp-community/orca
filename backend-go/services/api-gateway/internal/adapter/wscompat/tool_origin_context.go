package wscompat

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// ToolOrigin names the MCP session on whose behalf a terminal/agent is created
// (CONTRACT mcp-ui-api §5). It rides ctx, set by the MCP tool executor from the
// verified session, so a WS client can never supply or forge it through args.
type ToolOrigin struct {
	ClientName   string
	MCPSessionID string
	UserID       string
}

type toolOriginCtxKey struct{}

// WithToolOrigin marks ctx as an MCP-originated call.
func WithToolOrigin(ctx context.Context, o ToolOrigin) context.Context {
	return context.WithValue(ctx, toolOriginCtxKey{}, o)
}

func toolOriginFromContext(ctx context.Context) (ToolOrigin, bool) {
	o, ok := ctx.Value(toolOriginCtxKey{}).(ToolOrigin)
	return o, ok
}

// sessionOriginView is the additive `origin` member of terminal/agent results.
type sessionOriginView struct {
	Type         string `json:"type"`
	ClientName   string `json:"clientName"`
	McpSessionID string `json:"mcpSessionId"`
	UserID       string `json:"userId"`
}

// originProtoFromContext is nil for UI-originated calls.
func originProtoFromContext(ctx context.Context) *infrafleetv1.SessionOrigin {
	o, ok := toolOriginFromContext(ctx)
	if !ok {
		return nil
	}
	return &infrafleetv1.SessionOrigin{Type: "mcp", ClientName: o.ClientName, McpSessionId: o.MCPSessionID, UserId: o.UserID}
}

func originViewFromProto(o *infrafleetv1.SessionOrigin) *sessionOriginView {
	if o == nil || (o.GetType() == "" && o.GetMcpSessionId() == "" && o.GetClientName() == "" && o.GetUserId() == "") {
		return nil
	}
	return &sessionOriginView{Type: o.GetType(), ClientName: o.GetClientName(), McpSessionID: o.GetMcpSessionId(), UserID: o.GetUserId()}
}
