package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// BE-MCP-SOL-004 channels: mcp.session.list, mcp.session.close and
// mcp.admin.session.list. Each takes at most one object in args[0] (C10) and
// returns CONTRACT-shaped camelCase JSON; ids are row ids, the Mcp-Session-Id
// secret never reaches the UI.

// McpSessionCloser fans a session close out to every gateway replica (cancels
// in-flight tools, drops the resume buffer). Optional.
type McpSessionCloser interface {
	CloseSession(rowID, reason string)
}

// McpSessionView mirrors CONTRACT section 1.
type McpSessionView struct {
	ID              string `json:"id"`
	ClientName      string `json:"clientName"`
	GrantID         string `json:"grantId,omitempty"`
	TokenID         string `json:"tokenId,omitempty"`
	CreatedAt       string `json:"createdAt"`
	LastSeenAt      string `json:"lastSeenAt"`
	ProtocolVersion string `json:"protocolVersion"`
	ActiveStreams   int    `json:"activeStreams"`
	ToolCalls       int64  `json:"toolCalls"`
	UserID          string `json:"userId,omitempty"` // admin channel only
	UserName        string `json:"userName,omitempty"`
}

func registerMcpSessionChannels(r *Registry, d McpChannelDeps) {
	r.Register("mcp.session.list", mcpHandler(d, false, mcpSessionList(d, false)))
	r.Register("mcp.session.close", mcpHandler(d, false, mcpSessionClose(d)))
	r.Register("mcp.admin.session.list", mcpHandler(d, true, mcpSessionList(d, true)))
}

func toMcpSessionView(s *mcpv1.McpSession, admin bool) McpSessionView {
	v := McpSessionView{
		ID: s.GetId(), ClientName: s.GetClientName(), GrantID: s.GetGrantId(), TokenID: s.GetTokenId(),
		CreatedAt: tsString(s.GetCreatedAt()), LastSeenAt: tsString(s.GetLastSeenAt()), ProtocolVersion: s.GetProtocolVersion(),
		ActiveStreams: int(s.GetActiveStreams()), ToolCalls: s.GetToolCalls(),
	}
	if admin {
		v.UserID = s.GetUserId()
	}
	return v
}

func mcpSessionList(d McpChannelDeps, admin bool) ChannelHandler {
	return func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		var resp *mcpv1.ListSessionsResponse
		if admin {
			resp, err = c.ListSessionsAdmin(ctx, &mcpv1.ListSessionsRequest{})
		} else {
			resp, err = c.ListSessions(ctx, &mcpv1.ListSessionsRequest{})
		}
		if err != nil {
			return nil, err
		}
		out := make([]McpSessionView, 0, len(resp.GetSessions()))
		for _, s := range resp.GetSessions() {
			out = append(out, toMcpSessionView(s, admin))
		}
		return out, nil
	}
}

func mcpSessionClose(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			SessionID string `json:"sessionId"`
		}](args)
		if err != nil {
			return nil, err
		}
		in.SessionID = strings.TrimSpace(in.SessionID)
		if in.SessionID == "" {
			return nil, errors.New("MCP_INVALID_ARGUMENT: sessionId is required")
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		reason := "user"
		if id.Role == "admin" {
			reason = "admin"
		}
		// mcp-service answers MCP_NOT_FOUND for sessions that are not the
		// caller's (admins: not in their tenant) and succeeds idempotently for
		// sessions that are already closed.
		if _, err := c.CloseSession(ctx, &mcpv1.CloseSessionRequest{SessionId: in.SessionID, Reason: reason}); err != nil {
			return nil, err
		}
		if d.SessionCloser != nil {
			d.SessionCloser.CloseSession(in.SessionID, reason)
		}
		return map[string]bool{"ok": true}, nil
	}
}
